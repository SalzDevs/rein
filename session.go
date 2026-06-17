package rein

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/SalzDevs/rein/internal/procgroup"
	reinSignal "github.com/SalzDevs/rein/internal/signal"
)

// Line is a single line of output from a [Session].
type Line struct {
	// Stream is "stdout" or "stderr". With PTY enabled, both are
	// merged on a single stream and Stream is always "stdout".
	Stream string

	// Text is the line content, without the trailing newline.
	Text string
}

// maxLineSize is the maximum size of a single line in bytes.
// The default 64KB cap of bufio.Scanner is too small for some
// real-world outputs (e.g. a minified JSON blob, an SVG path).
const maxLineSize = 16 * 1024 * 1024 // 16 MB

// Session is a handle to a long-running command started by [Start]
// or [StartExec].
//
// A Session streams output line-by-line via [Session.Lines],
// exposes the process lifecycle via [Session.Done] and
// [Session.Wait], and can be cleanly stopped via [Session.Stop].
//
// If the session was started with a PTY (see [WithPTY]), input
// can be written to the child's stdin via [Session.Write] and
// the terminal window can be resized via [Session.Resize].
type Session struct {
	cmd             *exec.Cmd
	lines           chan Line
	done            chan struct{}
	gracefulTimeout time.Duration
	idleTimeout     time.Duration

	// ptyMaster is the master end of the PTY when the session was
	// started with WithPTY. Nil otherwise. Used by Write and Resize.
	ptyMaster *os.File

	// activityCh receives non-blocking pings on every line emitted.
	// Set by watchIdle if an idle timeout is configured. Nil otherwise.
	activityCh chan struct{}

	stopOnce sync.Once
	mu       sync.Mutex
	stopped  bool
	result   *Result
	err      error
}

// Start launches a long-running command and returns a [Session].
//
// The command runs in its own process group, so signals propagate
// to all descendants. Output is streamed line-by-line via
// [Session.Lines]. The process is killed on context cancellation,
// timeout, idle timeout, or [Session.Stop].
//
// The shutdown sequence on stop is:
//  1. SIGTERM is sent to the process group.
//  2. rein waits up to [Options.GracefulTimeout] for the process
//     to exit on its own.
//  3. If the process is still running, SIGKILL is sent to the
//     process group.
//
// For non-PTY sessions, the child's stdin is /dev/null. For
// PTY sessions (see [WithPTY]), the child's stdin is the PTY
// slave and input can be written via [Session.Write].
func Start(ctx context.Context, command string, opts ...Option) (*Session, error) {
	if command == "" {
		return nil, errors.New("rein: command is empty")
	}

	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	cmd := exec.Command("sh", "-c", command)
	if options.Dir != "" {
		cmd.Dir = options.Dir
	}
	if options.Env != nil {
		cmd.Env = options.Env
	}
	procgroup.Apply(cmd)

	var stdoutReader, stderrReader io.Reader
	var ptyMaster *os.File
	if options.PTY {
		// startWithPTY calls cmd.Start() internally (required by
		// the creack/pty API) and returns the master as a file.
		master, err := startWithPTY(cmd)
		if err != nil {
			return nil, err
		}
		ptyMaster = master
		// On a real TTY, stderr is merged with stdout.
		stdoutReader = master
		stderrReader = nil
	} else {
		sr, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("rein: failed to get stdout pipe: %w", err)
		}
		er, err := cmd.StderrPipe()
		if err != nil {
			return nil, fmt.Errorf("rein: failed to get stderr pipe: %w", err)
		}
		// For non-PTY sessions, the child should NOT read from
		// the agent's stdin pipe. We point it at /dev/null so
		// programs that read stdin (cat, head, etc.) get EOF
		// immediately rather than reading the agent's control
		// messages by accident.
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			return nil, fmt.Errorf("rein: failed to open %s: %w", os.DevNull, err)
		}
		cmd.Stdin = devNull
		if err := cmd.Start(); err != nil {
			devNull.Close()
			return nil, fmt.Errorf("rein: failed to start: %w", err)
		}
		// Child has its own copy of the devNull FD; parent can
		// close its copy safely.
		devNull.Close()
		stdoutReader = sr
		stderrReader = er
	}

	lineBuffer := options.LineBuffer
	if lineBuffer <= 0 {
		lineBuffer = 4096
	}
	s := &Session{
		cmd:             cmd,
		lines:           make(chan Line, lineBuffer),
		done:            make(chan struct{}),
		gracefulTimeout: options.GracefulTimeout,
		idleTimeout:     options.IdleTimeout,
		ptyMaster:       ptyMaster,
	}
	// Initialize the activity channel up-front if an idle timeout
	// is configured, so the line reader goroutines (started
	// below) can read it without a race.
	if options.IdleTimeout > 0 {
		s.activityCh = make(chan struct{}, 1)
	}

	// Watcher goroutines.
	go s.watchContext(ctx)
	if options.IdleTimeout > 0 {
		go s.watchIdle()
	}
	go s.waitAndClose(stdoutReader, stderrReader)

	return s, nil
}

// readLines reads from r line-by-line and sends each line to the
// session's lines channel. It also pings the activity channel if
// one is configured.
func (s *Session) readLines(stream string, r io.Reader) {
	if r == nil {
		return
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)
	for scanner.Scan() {
		line := Line{Stream: stream, Text: scanner.Text()}
		select {
		case s.lines <- line:
		case <-s.done:
			return
		}
		// Bump activity (non-blocking) so the idle watcher knows
		// the process is still emitting.
		if s.activityCh != nil {
			select {
			case s.activityCh <- struct{}{}:
			default:
			}
		}
	}
}

// watchContext triggers graceful shutdown when ctx is cancelled.
func (s *Session) watchContext(ctx context.Context) {
	<-ctx.Done()
	s.triggerShutdown()
}

// triggerShutdown sends SIGTERM, waits for the graceful timeout,
// then escalates to SIGKILL. Safe to call multiple times.
func (s *Session) triggerShutdown() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		s.mu.Unlock()

		if s.cmd == nil || s.cmd.Process == nil {
			return
		}

		_ = reinSignal.ProcessGroup(s.cmd, syscall.SIGTERM)

		select {
		case <-s.done:
			return
		case <-time.After(s.gracefulTimeout):
			_ = reinSignal.ProcessGroup(s.cmd, syscall.SIGKILL)
		}
	})
}

// watchIdle kills the process if no output is produced within the
// idle timeout. Communicates with readLines via a buffered
// activity channel (size 1) with non-blocking sends. The
// activity channel is initialized in Start() if needed.
func (s *Session) watchIdle() {
	if s.activityCh == nil {
		return
	}

	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()

	for {
		select {
		case <-s.activityCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.idleTimeout)
		case <-timer.C:
			s.triggerShutdown()
			return
		case <-s.done:
			return
		}
	}
}

// waitAndClose reads from the pipes, then waits for the process
// to exit, builds the Result, and closes the lines and done
// channels.
func (s *Session) waitAndClose(stdout, stderr io.Reader) {
	// Launch line readers in parallel. They exit when their pipe
	// returns EOF (the process exited) or the done channel is
	// closed (Stop() was called and the process was killed).
	go s.readLines("stdout", stdout)
	if stderr != nil {
		go s.readLines("stderr", stderr)
	}

	err := s.cmd.Wait()

	s.mu.Lock()
	if err == nil {
		s.result = &Result{ExitCode: 0}
		s.err = nil
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			s.result = &Result{ExitCode: exitErr.ExitCode(), Err: err}
		} else {
			s.result = &Result{ExitCode: -1, Err: err}
		}
		s.err = err
	}
	s.mu.Unlock()

	close(s.done)
	close(s.lines)
}

// Lines returns a channel that emits one [Line] per line of
// process output. The channel is closed when the process exits.
//
// The channel is buffered (default 4096 lines; see
// [WithLineBuffer]). If the consumer is slow, the producer will
// block once the buffer fills, which back-pressures the
// subprocess and (with [WithIdleTimeout] configured) triggers the
// idle kill.
func (s *Session) Lines() <-chan Line {
	return s.lines
}

// Wait blocks until the process exits and returns the final
// [Result]. It is safe to call Wait multiple times; subsequent
// calls return the same Result.
func (s *Session) Wait() (*Result, error) {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.err
}

// Done returns a channel that is closed when the process exits.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Stop sends SIGTERM, waits for the graceful timeout, then SIGKILL.
// It blocks until the process has exited. Safe to call multiple
// times; subsequent calls are no-ops that return nil.
func (s *Session) Stop() error {
	s.triggerShutdown()
	<-s.done
	return nil
}

// PID returns the OS process ID, or -1 if the process has not been
// started yet.
func (s *Session) PID() int {
	if s.cmd == nil || s.cmd.Process == nil {
		return -1
	}
	return s.cmd.Process.Pid
}

// Write sends p to the child's stdin. Only works for sessions
// started with [WithPTY]; for non-PTY sessions it returns an
// error.
//
// Write is safe to call from any goroutine.
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	master := s.ptyMaster
	s.mu.Unlock()
	if master == nil {
		return 0, errors.New("rein: session is not running with a PTY; use WithPTY")
	}
	return master.Write(p)
}

// Resize resizes the PTY window to rows x cols. Only works for
// sessions started with [WithPTY]; for non-PTY sessions it
// returns an error.
//
// Resize is safe to call from any goroutine.
func (s *Session) Resize(rows, cols int) error {
	s.mu.Lock()
	master := s.ptyMaster
	s.mu.Unlock()
	if master == nil {
		return errors.New("rein: session is not running with a PTY; use WithPTY")
	}
	return resizePTY(master, rows, cols)
}

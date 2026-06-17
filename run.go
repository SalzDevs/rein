package rein

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/SalzDevs/rein/internal/procgroup"
	reinSignal "github.com/SalzDevs/rein/internal/signal"
)

// Run executes a shell command and waits for it to complete or until
// the timeout fires.
//
// The command is run via `sh -c <command>`, so shell features
// (pipes, redirects, globs, env var expansion) work as expected.
// The process tree is isolated into its own process group so that
// signals reach every descendant, not just the leader.
//
// Shutdown sequence on timeout or context cancellation:
//  1. SIGTERM is sent to the process group.
//  2. rein waits up to [Options.GracefulTimeout] for the process
//     to exit on its own.
//  3. If the process is still running, SIGKILL is sent to the
//     process group.
//
// Run always returns a non-nil *Result, even on failure. The
// returned error is non-nil if and only if result.Err is non-nil.
func Run(ctx context.Context, command string, opts ...Option) (*Result, error) {
	if command == "" {
		return nil, errors.New("rein: command is empty")
	}

	start := time.Now()
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	runCtx, cancel := context.WithCancel(ctx)
	if options.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	}
	defer cancel()

	cmd := exec.Command("sh", "-c", command)
	if options.Dir != "" {
		cmd.Dir = options.Dir
	}
	if options.Env != nil {
		cmd.Env = options.Env
	}
	procgroup.Apply(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process in the current goroutine so that cmd.Process
	// is set before any other code reads it. Only Wait runs in a
	// goroutine, racing the context cancellation.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("rein: failed to start command: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var runErr error
	select {
	case runErr = <-done:
		// Process completed on its own (success or non-zero exit).
	case <-runCtx.Done():
		// Timeout or context cancel. Stop the process tree
		// gracefully: SIGTERM, wait, then SIGKILL.
		_ = reinSignal.ProcessGroup(cmd, syscall.SIGTERM)
		select {
		case runErr = <-done:
			// Process exited gracefully.
		case <-time.After(options.GracefulTimeout):
			_ = reinSignal.ProcessGroup(cmd, syscall.SIGKILL)
			runErr = <-done
		}
	}

	duration := time.Since(start)
	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if runErr == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}

	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		result.Err = fmt.Errorf("command timed out after %s", options.Timeout)
	case errors.Is(runCtx.Err(), context.Canceled):
		result.Err = fmt.Errorf("command cancelled: %w", context.Canceled)
	case result.ExitCode != 0:
		result.Err = fmt.Errorf("command exited with code %d", result.ExitCode)
	default:
		result.Err = runErr
	}

	return result, result.Err
}

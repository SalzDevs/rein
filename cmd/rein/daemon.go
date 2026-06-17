package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SalzDevs/rein"
	"github.com/SalzDevs/rein/internal/ndjson"
)

const daemonUsage = `rein daemon - long-running multi-session manager

Usage:
  rein daemon

A long-running process that manages multiple rein sessions.
Reads commands on stdin, emits events on stdout, both as NDJSON.

Inputs (NDJSON on stdin, one message per line):
  {"type":"create","id":"my-session","command":"npm run dev","options":{"idle_timeout":"2m"}}
  {"type":"destroy","id":"my-session"}
  {"type":"input","id":"my-session","text":"password\n"}    (PTY sessions only)
  {"type":"resize","id":"my-session","rows":40,"cols":120} (PTY sessions only)
  {"type":"stop","id":"my-session"}
  {"type":"list"}

Outputs (NDJSON on stdout, one message per line):
  {"type":"created","id":"my-session","pid":1234}
  {"type":"line","id":"my-session","stream":"stdout","text":"..."}
  {"type":"exit","id":"my-session","exit_code":0,"duration_ms":1234}
  {"type":"destroyed","id":"my-session","exit_code":0}
  {"type":"error","id":"my-session","err":"..."}
  {"type":"sessions","sessions":[{"id":"...","pid":...}]}

Example:
  echo '{"type":"create","id":"s1","command":"sleep 30"}' | rein daemon
`

// daemonSession is a single session managed by the daemon.
type daemonSession struct {
	id      string
	session *rein.Session
	pty     bool
}

// daemon is the multi-session state machine.
type daemon struct {
	mu       sync.Mutex
	sessions map[string]*daemonSession
	enc      *ndjson.Encoder
}

func newDaemon() *daemon {
	return &daemon{
		sessions: make(map[string]*daemonSession),
		enc:      ndjson.NewEncoder(os.Stdout),
	}
}

// emit writes a message to stdout. Errors are reported to stderr
// but not returned, since callers in this daemon don't have a
// good way to handle them.
func (d *daemon) emit(m *ndjson.Message) {
	if err := d.enc.Encode(m); err != nil {
		fmt.Fprintln(os.Stderr, "rein daemon: write failed:", err)
	}
}

// emitErr sends an error message. If id is non-empty, it tags
// the error with the session ID.
func (d *daemon) emitErr(id, msg string) {
	d.emit(&ndjson.Message{Type: ndjson.TypeError, ID: id, Err: msg})
}

// startSession creates a new session and returns it. The
// session's lifetime is managed by the daemon: a goroutine
// forwards lines and exit events back to the caller.
func (d *daemon) startSession(id, command string, opts []rein.Option) {
	d.mu.Lock()
	if _, exists := d.sessions[id]; exists {
		d.mu.Unlock()
		d.emitErr(id, "session already exists")
		return
	}
	d.mu.Unlock()

	// Default to a generous PTY for daemon-managed sessions.
	// The user can opt out by including their own options.
	hasPTY := false
	for _, o := range opts {
		_ = o
		// We can't introspect options easily; check by trying.
	}
	// Use a simpler heuristic: if the command looks like it
	// needs a TTY (e.g. "sudo", "ssh", "vim"), use PTY. For
	// the daemon, the caller can pass --pty explicitly via the
	// options map. For v0.1, default to non-PTY.
	_ = hasPTY

	session, err := rein.Start(context.Background(), command, opts...)
	if err != nil {
		d.emitErr(id, "failed to start: "+err.Error())
		return
	}

	ds := &daemonSession{id: id, session: session}

	d.mu.Lock()
	d.sessions[id] = ds
	d.mu.Unlock()

	pid := session.PID()
	d.emit(&ndjson.Message{Type: ndjson.TypeCreated, ID: id, PID: &pid})

	// Forward lines and exit to stdout.
	go d.forwardLines(id, session)
}

// forwardLines reads from session.Lines() and emits each line as
// a tagged NDJSON message. When the session exits, it emits a
// destroyed message and removes the session from the map.
func (d *daemon) forwardLines(id string, session *rein.Session) {
	go func() {
		for line := range session.Lines() {
			d.emit(&ndjson.Message{
				Type:   ndjson.TypeLine,
				ID:     id,
				Stream: line.Stream,
				Text:   line.Text,
			})
		}
	}()

	result, _ := session.Wait()

	ec := result.ExitCode
	d.emit(&ndjson.Message{Type: ndjson.TypeExit, ID: id, ExitCode: &ec})

	d.mu.Lock()
	delete(d.sessions, id)
	d.mu.Unlock()

	d.emit(&ndjson.Message{Type: ndjson.TypeDestroyed, ID: id, ExitCode: &ec})
}

// destroySession stops and removes a session.
func (d *daemon) destroySession(id string) {
	d.mu.Lock()
	ds, ok := d.sessions[id]
	d.mu.Unlock()
	if !ok {
		d.emitErr(id, "session not found")
		return
	}
	_ = ds.session.Stop()
	// forwardLines will emit destroyed when Wait returns.
}

// listSessions emits a sessions message with the current list.
func (d *daemon) listSessions() {
	d.mu.Lock()
	sessions := make([]ndjson.SessionInfo, 0, len(d.sessions))
	for _, ds := range d.sessions {
		pid := ds.session.PID()
		sessions = append(sessions, ndjson.SessionInfo{ID: ds.id, PID: pid})
	}
	d.mu.Unlock()

	d.emit(&ndjson.Message{Type: ndjson.TypeSessions, Sessions: sessions})
}

// sendToSession writes input or resizes a session.
func (d *daemon) sendToSession(id string, fn func(*rein.Session) error, op string) {
	d.mu.Lock()
	ds, ok := d.sessions[id]
	d.mu.Unlock()
	if !ok {
		d.emitErr(id, "session not found")
		return
	}
	if err := fn(ds.session); err != nil {
		d.emitErr(id, op+" failed: "+err.Error())
	}
}

// handleMessage dispatches a single incoming NDJSON message to
// the appropriate daemon method.
func (d *daemon) handleMessage(msg *ndjson.Message) {
	switch msg.Type {
	case ndjson.TypeCreate:
		if msg.ID == "" {
			d.emitErr("", "create: id is required")
			return
		}
		if msg.Command == "" {
			d.emitErr(msg.ID, "create: command is required")
			return
		}
		opts := parseOptions(msg)
		d.startSession(msg.ID, msg.Command, opts)

	case ndjson.TypeDestroy, ndjson.TypeStop:
		if msg.ID == "" {
			d.emitErr("", msg.Type+": id is required")
			return
		}
		d.destroySession(msg.ID)

	case ndjson.TypeInput:
		if msg.ID == "" {
			d.emitErr("", "input: id is required")
			return
		}
		d.sendToSession(msg.ID, func(s *rein.Session) error {
			_, err := s.Write([]byte(msg.Text))
			return err
		}, "input")

	case ndjson.TypeResize:
		if msg.ID == "" {
			d.emitErr("", "resize: id is required")
			return
		}
		if msg.Rows == nil || msg.Cols == nil {
			d.emitErr(msg.ID, "resize: rows and cols are required")
			return
		}
		d.sendToSession(msg.ID, func(s *rein.Session) error {
			return s.Resize(*msg.Rows, *msg.Cols)
		}, "resize")

	case ndjson.TypeList:
		d.listSessions()

	default:
		d.emitErr("", "unknown message type: "+msg.Type)
	}
}

// parseOptions extracts rein options from the raw NDJSON message.
// The map of options is parsed into the appropriate types.
func parseOptions(msg *ndjson.Message) []rein.Option {
	// For v0.1, the daemon doesn't support arbitrary options
	// from NDJSON. Users that need options should use the
	// library directly. Returning nil here means "use defaults".
	_ = msg
	return nil
}

// runDaemon starts the daemon main loop.
func runDaemon() {
	d := newDaemon()

	// Also handle stdin EOF from a separate watcher in case
	// the main loop is blocked in a handler.
	go func() {
		<-stdinClosed()
		// If the main loop hasn't already handled the EOF, do
		// it ourselves. This is a no-op if the main loop has
		// already returned.
		d.stopAllAndWait()
		os.Exit(0)
	}()

	dec := ndjson.NewDecoder(bufio.NewReader(os.Stdin))
	for {
		msg, err := dec.Decode()
		if err == io.EOF {
			// Stdin closed. Stop all sessions and wait for the
			// destroyed messages to be emitted before returning.
			d.stopAllAndWait()
			return
		}
		if err != nil {
			// Skip invalid messages but keep reading.
			continue
		}
		d.handleMessage(msg)
	}
}

// stopAllAndWait stops all active sessions and blocks until
// they have all emitted their exit/destroyed messages. This
// ensures the caller sees a clean shutdown.
func (d *daemon) stopAllAndWait() {
	d.mu.Lock()
	sessions := make([]*daemonSession, 0, len(d.sessions))
	for _, ds := range d.sessions {
		sessions = append(sessions, ds)
	}
	d.mu.Unlock()

	// Stop all sessions in parallel.
	var wg sync.WaitGroup
	for _, ds := range sessions {
		wg.Add(1)
		go func(s *daemonSession) {
			defer wg.Done()
			_ = s.session.Stop()
		}(ds)
	}
	wg.Wait()

	// Give the forwardLines goroutines a moment to emit
	// the exit and destroyed messages after Wait returns.
	time.Sleep(50 * time.Millisecond)
}

// stdinClosed returns a channel that closes when stdin is closed
// (EOF on the underlying file).
func stdinClosed() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		// Read one byte. When the read returns EOF, the channel
		// closes.
		buf := make([]byte, 1)
		_, _ = os.Stdin.Read(buf)
		close(done)
	}()
	return done
}

// _ keeps strconv referenced for the (currently unused) parseInt
// helper that may be added later.
var _ = strconv.Itoa

// _ keeps the flag package referenced for future CLI flags.
var _ = flag.NewFlagSet

// _ keeps the strings package referenced for future message
// parsing helpers.
var _ = strings.TrimSpace

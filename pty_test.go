package rein

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// startPTYOrSkip calls Start with WithPTY and either returns the
// session, fails the test, or skips it (when the environment does
// not permit PTY allocation, e.g. inside the pi agent's sandbox
// where fork+exec is restricted for PTY specifically).
func startPTYOrSkip(t *testing.T, command string, opts ...Option) *Session {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PTY not yet implemented on Windows")
	}
	allOpts := append([]Option{WithPTY()}, opts...)
	session, err := Start(context.Background(), command, allOpts...)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("PTY not available in this environment: %v", err)
		}
		t.Fatalf("Start with PTY failed: %v", err)
	}
	return session
}

func TestStart_WithPTY_SeesTTY(t *testing.T) {
	// `tty -s` exits 0 if stdin is a TTY, non-zero otherwise.
	session := startPTYOrSkip(t,
		`if tty -s 2>/dev/null; then echo is-tty; else echo not-tty; fi`,
		WithTimeout(5*time.Second),
	)
	defer session.Stop()

	lines := readLines(t, session, 1, 3*time.Second)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(lines), lines)
	}
	if lines[0].Text != "is-tty" {
		t.Errorf("expected 'is-tty' (PTY should make tty -s succeed), got: %q", lines[0].Text)
	}
}

func TestStart_WithoutPTY_NoTTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tty command not available on Windows")
	}

	session, err := Start(context.Background(),
		`tty 2>/dev/null || echo not-a-tty`,
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer session.Stop()

	lines := readLines(t, session, 1, 3*time.Second)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0].Text, "not") {
		t.Errorf("expected 'not a tty', got: %q", lines[0].Text)
	}
}

func TestStart_PTY_StreamsOutput(t *testing.T) {
	// With PTY, output is still streamed line-by-line via Lines().
	session := startPTYOrSkip(t,
		`echo pty-line-1; echo pty-line-2`,
		WithTimeout(5*time.Second),
	)
	defer session.Stop()

	lines := readLines(t, session, 2, 3*time.Second)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Text != "pty-line-1" || lines[1].Text != "pty-line-2" {
		t.Errorf("unexpected lines: %+v", lines)
	}
}

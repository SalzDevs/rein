package rein

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRun_Success(t *testing.T) {
	result, err := Run(context.Background(), `echo hello`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got: %d", result.ExitCode)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("expected stdout 'hello', got: %q", result.Stdout)
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	result, err := Run(context.Background(), `exit 1`)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got: %d", result.ExitCode)
	}
}

func TestRun_Stderr(t *testing.T) {
	result, err := Run(context.Background(), `echo error >&2`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if strings.TrimSpace(result.Stderr) != "error" {
		t.Fatalf("expected stderr 'error', got: %q", result.Stderr)
	}
}

func TestRun_Timeout(t *testing.T) {
	result, err := Run(context.Background(), `sleep 10`, WithTimeout(100*time.Millisecond))
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit code after timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected 'timed out' in error, got: %v", err)
	}
}

func TestRun_TimeoutShorterThanCommand(t *testing.T) {
	start := time.Now()
	_, err := Run(context.Background(), `sleep 5`, WithTimeout(50*time.Millisecond))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("command should have been killed quickly, took %s", elapsed)
	}
}

func TestRun_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := Run(ctx, `sleep 10`)
	if err == nil {
		t.Fatal("expected error for cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in error chain, got: %v", err)
	}
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit code after cancel")
	}
}

func TestRun_Env(t *testing.T) {
	result, err := Run(context.Background(), `echo "$REIN_TEST_VAR"`,
		WithEnv([]string{"REIN_TEST_VAR=hello"}),
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("expected stdout 'hello', got: %q", result.Stdout)
	}
}

func TestRun_Dir(t *testing.T) {
	result, err := Run(context.Background(), `pwd`, WithDir("/tmp"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// macOS symlinks /tmp -> /private/tmp, so accept either.
	got := strings.TrimSpace(result.Stdout)
	if got != "/tmp" && got != "/private/tmp" {
		t.Fatalf("expected pwd to be /tmp, got: %q", got)
	}
}

func TestRun_EmptyCommand(t *testing.T) {
	_, err := Run(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestRun_Duration(t *testing.T) {
	result, err := Run(context.Background(), `sleep 0.1`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Duration < 100*time.Millisecond {
		t.Fatalf("expected duration >= 100ms, got: %s", result.Duration)
	}
}

// TestRun_KillsProcessGroup verifies that when rein kills the parent
// process on timeout, every descendant in the same process group is
// also killed. This is the whole point of the procgroup + signal
// plumbing — without it, `npm install && watch` would leak the
// watcher forever.
func TestRun_KillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group isolation not implemented on Windows")
	}

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")

	// The script spawns a long-running child and then waits. When
	// rein times out the parent, the child must also die because it
	// is in the same process group.
	script := fmt.Sprintf(`
		sleep 60 &
		CHILD_PID=$!
		echo $CHILD_PID > %s
		wait $CHILD_PID
	`, pidFile)

	_, err := Run(context.Background(), script,
		WithTimeout(200*time.Millisecond),
		WithGracefulTimeout(100*time.Millisecond),
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Give the OS a moment to deliver the signal and reap the child.
	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("could not read child PID file: %v", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("invalid child PID %q: %v", pidStr, err)
	}

	// kill(pid, 0) on a dead process returns ESRCH. On a live
	// process it returns nil. We use this to check liveness without
	// actually sending a signal.
	if err := processIsAlive(pid); err == nil {
		t.Errorf("child process %d is still alive after parent timeout — process group isolation is not working", pid)
	} else if !errors.Is(err, syscall.ESRCH) {
		// Some other error. On Linux this could be EPERM if a
		// different process now owns the PID (PID reuse) or
		// something stranger. Surface it as a warning, not a fail,
		// unless the process is clearly alive.
		t.Logf("kill(%d, 0) returned: %v", pid, err)
	}
}

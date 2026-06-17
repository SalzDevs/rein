package rein

import (
	"context"
	"strings"
	"testing"
	"time"
)

// readLines drains up to n lines from a session's Lines channel
// with a timeout. Returns the lines read so far.
func readLines(t *testing.T, s *Session, n int, timeout time.Duration) []Line {
	t.Helper()
	var got []Line
	deadline := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case line, ok := <-s.Lines():
			if !ok {
				return got
			}
			got = append(got, line)
		case <-deadline:
			return got
		}
	}
	return got
}

func TestStart_StreamsStdout(t *testing.T) {
	session, err := Start(context.Background(), `echo hello; echo world`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer session.Stop()

	lines := readLines(t, session, 2, 2*time.Second)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Stream != "stdout" || lines[0].Text != "hello" {
		t.Errorf("expected stdout 'hello', got %+v", lines[0])
	}
	if lines[1].Stream != "stdout" || lines[1].Text != "world" {
		t.Errorf("expected stdout 'world', got %+v", lines[1])
	}

	result, err := session.Wait()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got: %d", result.ExitCode)
	}
}

func TestStart_StreamsStderrSeparately(t *testing.T) {
	session, err := Start(context.Background(),
		`echo out; echo err >&2`,
	)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer session.Stop()

	lines := readLines(t, session, 2, 2*time.Second)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}

	var sawStdout, sawStderr bool
	for _, l := range lines {
		if l.Stream == "stdout" && l.Text == "out" {
			sawStdout = true
		}
		if l.Stream == "stderr" && l.Text == "err" {
			sawStderr = true
		}
	}
	if !sawStdout {
		t.Errorf("expected stdout line 'out', got: %+v", lines)
	}
	if !sawStderr {
		t.Errorf("expected stderr line 'err', got: %+v", lines)
	}
}

func TestStart_IdleTimeoutKillsHangingCommand(t *testing.T) {
	// Process produces one line then hangs forever.
	session, err := Start(context.Background(),
		`echo hello; sleep 60`,
		WithIdleTimeout(200*time.Millisecond),
		WithGracefulTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Consume the first line so the line goroutine sends the
	// activity ping and the timer starts ticking from the
	// last-output time.
	lines := readLines(t, session, 1, 1*time.Second)
	if len(lines) != 1 || lines[0].Text != "hello" {
		t.Fatalf("expected first line 'hello', got: %+v", lines)
	}

	// Now the process is hanging. Idle timeout should fire.
	start := time.Now()
	result, err := session.Wait()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error after idle kill")
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit after idle kill")
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected quick idle kill, took %s", elapsed)
	}
}

func TestStart_IdleTimeoutResetsOnActivity(t *testing.T) {
	// Process produces a line every 100ms for 500ms total.
	// Idle timeout is 300ms. Since the process is never silent
	// for 300ms, it should NOT be killed by idle timeout.
	session, err := Start(context.Background(),
		`for i in 1 2 3 4 5; do echo tick; sleep 0.1; done`,
		WithIdleTimeout(300*time.Millisecond),
		WithGracefulTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Drain the lines in a goroutine so they don't block.
	done := make(chan struct{})
	go func() {
		for range session.Lines() {
		}
		close(done)
	}()

	result, err := session.Wait()
	<-done

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got: %d (process was wrongly killed by idle timeout)", result.ExitCode)
	}
}

func TestStart_StopKillsLongRunningCommand(t *testing.T) {
	session, err := Start(context.Background(), `sleep 60`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it run a moment.
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	if err := session.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Stop should return quickly, took %s", elapsed)
	}

	result, err := session.Wait()
	if err == nil {
		t.Error("expected error after Stop")
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit after Stop")
	}
}

func TestStart_StopIsIdempotent(t *testing.T) {
	session, err := Start(context.Background(), `sleep 5`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := session.Stop(); err != nil {
		t.Errorf("first Stop: %v", err)
	}
	// Second Stop should be a no-op that returns nil.
	if err := session.Stop(); err != nil {
		t.Errorf("second Stop: %v", err)
	}
	// Third for good measure.
	if err := session.Stop(); err != nil {
		t.Errorf("third Stop: %v", err)
	}
}

func TestStart_ContextCancelKillsProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	session, err := Start(ctx, `sleep 60`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	start := time.Now()
	result, err := session.Wait()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error after cancel")
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit after cancel")
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected quick cancel, took %s", elapsed)
	}
}

func TestStart_EmptyCommand(t *testing.T) {
	_, err := Start(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

func TestStart_Env(t *testing.T) {
	session, err := Start(context.Background(),
		`echo "$REIN_SESSION_TEST"`,
		WithEnv([]string{"REIN_SESSION_TEST=from-env"}),
	)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer session.Stop()

	lines := readLines(t, session, 1, 2*time.Second)
	if len(lines) != 1 || lines[0].Text != "from-env" {
		t.Errorf("expected 'from-env', got: %+v", lines)
	}
}

func TestStart_PID(t *testing.T) {
	session, err := Start(context.Background(), `sleep 5`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer session.Stop()

	pid := session.PID()
	if pid <= 0 {
		t.Errorf("expected positive PID, got: %d", pid)
	}
}

func TestStart_DoneChannel(t *testing.T) {
	session, err := Start(context.Background(), `echo done`)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case <-session.Done():
		// Good, Done was closed when the process exited
	case <-time.After(2 * time.Second):
		t.Fatal("Done channel was not closed within 2s")
	}
}

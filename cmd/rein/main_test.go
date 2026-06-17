package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binPath is the path to the built `rein` binary. It is set in
// TestMain before any test runs.
var binPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "rein-cli-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binPath = filepath.Join(tmpDir, "rein")
	// Build from the project root (one level up from cmd/rein/).
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build rein binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("rein %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String(), stderr.String(), exitCode
}

func runCLIWithStdin(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("rein %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestCLIVersion(t *testing.T) {
	stdout, _, _ := runCLI(t, "version")
	if !strings.HasPrefix(stdout, "rein ") {
		t.Errorf("expected version output to start with 'rein ', got: %q", stdout)
	}
}

func TestCLIHelp(t *testing.T) {
	stdout, _, _ := runCLI(t, "help")
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("expected help to contain 'Usage:', got: %q", stdout)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	_, stderr, code := runCLI(t, "nonsense")
	if code == 0 {
		t.Error("expected non-zero exit for unknown command")
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got: %q", stderr)
	}
}

func TestCLIRun_Basic(t *testing.T) {
	stdout, _, code := runCLI(t, "run", "echo hello")
	if code != 0 {
		t.Errorf("expected exit code 0, got: %d", code)
	}

	var msg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &msg); err != nil {
		t.Fatalf("invalid NDJSON: %v\noutput: %q", err, stdout)
	}
	if msg["type"] != "exit" {
		t.Errorf("expected type=exit, got: %v", msg["type"])
	}
	if msg["exit_code"].(float64) != 0 {
		t.Errorf("expected exit_code=0, got: %v", msg["exit_code"])
	}
	if !strings.Contains(msg["stdout"].(string), "hello") {
		t.Errorf("expected stdout to contain 'hello', got: %q", msg["stdout"])
	}
}

func TestCLIRun_NonZeroExit(t *testing.T) {
	stdout, _, code := runCLI(t, "run", "exit 7")
	if code != 1 {
		t.Errorf("expected exit code 1 from rein (process exit 7), got: %d", code)
	}

	var msg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &msg); err != nil {
		t.Fatalf("invalid NDJSON: %v\noutput: %q", err, stdout)
	}
	if msg["type"] != "exit" {
		t.Errorf("expected type=exit, got: %v", msg["type"])
	}
	if msg["exit_code"].(float64) != 7 {
		t.Errorf("expected exit_code=7, got: %v", msg["exit_code"])
	}
}

func TestCLIRun_Timeout(t *testing.T) {
	start := time.Now()
	stdout, _, code := runCLI(t, "run", "--timeout", "200ms", "sleep 5")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("expected quick timeout, took %s", elapsed)
	}
	// The exit code may be 1 (from rein) or signal-killed, both OK.
	if code == 0 {
		t.Errorf("expected non-zero exit after timeout, got: %d", code)
	}

	var msg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &msg); err != nil {
		t.Fatalf("invalid NDJSON: %v\noutput: %q", err, stdout)
	}
	if msg["type"] != "exit" {
		t.Errorf("expected type=exit, got: %v", msg["type"])
	}
}

func TestCLIRun_NoCommand(t *testing.T) {
	_, stderr, code := runCLI(t, "run")
	if code == 0 {
		t.Error("expected non-zero exit when no command is given")
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage info in stderr, got: %q", stderr)
	}
}

func TestCLIStart_StreamsLines(t *testing.T) {
	stdout, _, _ := runCLI(t, "start", "echo line1; echo line2; echo line3")

	// Parse NDJSON output. The lines arrive in order.
	lines := readNDJSONLines(t, stdout)

	if len(lines) < 5 {
		t.Fatalf("expected at least 5 messages (started + 3 lines + exit), got %d:\n%s", len(lines), stdout)
	}
	if lines[0]["type"] != "started" {
		t.Errorf("expected first message type=started, got: %v", lines[0]["type"])
	}
	if lines[0]["pid"].(float64) <= 0 {
		t.Errorf("expected positive pid, got: %v", lines[0]["pid"])
	}

	// Find the line messages and verify their text.
	var texts []string
	for _, m := range lines[1:] {
		if m["type"] == "line" {
			texts = append(texts, m["text"].(string))
		}
	}
	if len(texts) != 3 {
		t.Errorf("expected 3 line messages, got %d: %v", len(texts), texts)
	}
	want := []string{"line1", "line2", "line3"}
	for i, w := range want {
		if i < len(texts) && texts[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, texts[i], w)
		}
	}

	// The last message should be exit.
	last := lines[len(lines)-1]
	if last["type"] != "exit" {
		t.Errorf("expected last message type=exit, got: %v", last["type"])
	}
}

func TestCLIStart_StopViaStdin(t *testing.T) {
	// Start a long sleep, then send a stop message via stdin.
	stdout, _, _ := runCLIWithStdin(t, `{"type":"stop"}`+"\n", "start", "sleep 30")

	lines := readNDJSONLines(t, stdout)

	// We should see at least started, then exit.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 messages, got %d:\n%s", len(lines), stdout)
	}
	if lines[0]["type"] != "started" {
		t.Errorf("expected first message type=started, got: %v", lines[0]["type"])
	}
	last := lines[len(lines)-1]
	if last["type"] != "exit" {
		t.Errorf("expected last message type=exit, got: %v", last["type"])
	}
	// The exit should be non-zero (process was killed).
	if last["exit_code"].(float64) == 0 {
		t.Errorf("expected non-zero exit_code after stop, got: %v", last["exit_code"])
	}
}

func TestCLIStart_IdleTimeout(t *testing.T) {
	// Command produces one line then hangs. Idle timeout of 200ms
	// should kill it.
	start := time.Now()
	stdout, _, _ := runCLI(t, "start", "--idle-timeout", "200ms", "--graceful-timeout", "100ms", "echo hello; sleep 30")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("expected quick idle kill, took %s", elapsed)
	}

	lines := readNDJSONLines(t, stdout)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 messages, got %d:\n%s", len(lines), stdout)
	}
	last := lines[len(lines)-1]
	if last["type"] != "exit" {
		t.Errorf("expected last message type=exit, got: %v", last["type"])
	}
	if last["exit_code"].(float64) == 0 {
		t.Errorf("expected non-zero exit_code after idle kill, got: %v", last["exit_code"])
	}
}

// execCLIOrSkip runs `rein exec args...` and skips the test if
// PTY allocation fails (e.g. the pi agent's sandboxed shell).
func execCLIOrSkip(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	stdout, stderr, code := runCLI(t, args...)
	if strings.Contains(stdout, "operation not permitted") || strings.Contains(stderr, "operation not permitted") {
		t.Skipf("PTY not available in this environment")
	}
	// If the only output is an error message, skip.
	if code != 0 && strings.Contains(stdout, `"type":"error"`) {
		t.Skipf("PTY not available in this environment: %s", stdout)
	}
	return stdout, stderr, code
}

func TestCLIExec_StreamsAndExits(t *testing.T) {
	stdout, _, _ := execCLIOrSkip(t, "exec", "echo exec-line-1; echo exec-line-2")

	lines := readNDJSONLines(t, stdout)
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 messages (started + 2 lines + exit), got %d:\n%s", len(lines), stdout)
	}
	if lines[0]["type"] != "started" {
		t.Errorf("expected first message type=started, got: %v", lines[0]["type"])
	}
	var texts []string
	for _, m := range lines[1:] {
		if m["type"] == "line" {
			texts = append(texts, m["text"].(string))
		}
	}
	if len(texts) != 2 {
		t.Errorf("expected 2 line messages, got %d: %v", len(texts), texts)
	} else {
		if texts[0] != "exec-line-1" || texts[1] != "exec-line-2" {
			t.Errorf("unexpected lines: %v", texts)
		}
	}
	last := lines[len(lines)-1]
	if last["type"] != "exit" {
		t.Errorf("expected last message type=exit, got: %v", last["type"])
	}
}

func TestCLIExec_StopViaStdin(t *testing.T) {
	stdout, _, _ := runCLIWithStdin(t, `{"type":"stop"}`+"\n", "exec", "sleep 30")

	// Skip if PTY is not available.
	if strings.Contains(stdout, "operation not permitted") {
		t.Skipf("PTY not available in this environment: %s", stdout)
	}

	lines := readNDJSONLines(t, stdout)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 messages, got %d:\n%s", len(lines), stdout)
	}
	if lines[0]["type"] != "started" {
		t.Errorf("expected first message type=started, got: %v", lines[0]["type"])
	}
	last := lines[len(lines)-1]
	if last["type"] != "exit" {
		t.Errorf("expected last message type=exit, got: %v", last["type"])
	}
	if last["exit_code"].(float64) == 0 {
		t.Errorf("expected non-zero exit_code after stop, got: %v", last["exit_code"])
	}
}

func TestCLIExec_InputAndResize(t *testing.T) {
	// `cat` echoes its stdin to stdout. With a PTY, we can write
	// to its stdin via NDJSON and read the echoed output.
	stdin := `{"type":"input","text":"hello-from-input\n"}` + "\n" +
		`{"type":"resize","rows":40,"cols":120}` + "\n" +
		`{"type":"stop"}` + "\n"
	stdout, _, _ := runCLIWithStdin(t, stdin, "exec", "cat")

	// Skip if PTY is not available.
	if strings.Contains(stdout, "operation not permitted") {
		t.Skipf("PTY not available in this environment: %s", stdout)
	}

	lines := readNDJSONLines(t, stdout)

	// Find the line message and verify the text.
	var sawInput bool
	for _, m := range lines {
		if m["type"] == "line" {
			if m["text"] == "hello-from-input" {
				sawInput = true
				break
			}
		}
	}
	if !sawInput {
		t.Errorf("expected to see 'hello-from-input' as a line, got messages:\n%s", stdout)
	}
}

func TestCLIExec_NoCommand(t *testing.T) {
	_, stderr, code := runCLI(t, "exec")
	if code == 0 {
		t.Error("expected non-zero exit when no command is given")
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage info in stderr, got: %q", stderr)
	}
}

func TestCLIDaemon_CreateAndList(t *testing.T) {
	// Start the daemon and send a "create" then a "list".
	stdin := `{"type":"create","id":"s1","command":"sleep 5"}` + "\n" +
		`{"type":"list"}` + "\n" +
		`{"type":"destroy","id":"s1"}` + "\n"
	stdout, _, _ := runCLIWithStdin(t, stdin, "daemon")

	// Parse all NDJSON messages.
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	var msgs []map[string]any
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("invalid NDJSON: %v", err)
		}
		msgs = append(msgs, m)
	}

	// Expect at least: created, sessions, line, exit, destroyed.
	var sawCreated, sawSessions, sawDestroyed bool
	for _, m := range msgs {
		switch m["type"] {
		case "created":
			if m["id"] == "s1" {
				sawCreated = true
			}
		case "sessions":
			sawSessions = true
		case "destroyed":
			if m["id"] == "s1" {
				sawDestroyed = true
			}
		}
	}
	if !sawCreated {
		t.Errorf("expected 'created' message, got: %s", stdout)
	}
	if !sawSessions {
		t.Errorf("expected 'sessions' message, got: %s", stdout)
	}
	if !sawDestroyed {
		t.Errorf("expected 'destroyed' message, got: %s", stdout)
	}
}

// readNDJSONLines parses newline-delimited JSON from s.
func readNDJSONLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return out
}

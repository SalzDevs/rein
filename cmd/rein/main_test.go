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

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SalzDevs/rein"
	"github.com/SalzDevs/rein/internal/ndjson"
)

const execUsage = `rein exec - run a command with a PTY, drive it interactively

Usage:
  rein exec [options] <command>

Options:
  --timeout <duration>          Maximum wall-clock duration (0 = no timeout)
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL (default 5s)
  --idle-timeout <duration>     Kill the process if no output for this duration (0 = no idle timeout)
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --line-buffer <n>             Size of the output channel buffer (default 4096)
  --initial-size <rows>x<cols>  Initial PTY window size (default 24x80)

A PTY is always allocated. This subcommand is for interactive
commands that prompt the user (sudo, ssh-add, npm init, etc.) and
for any TTY-aware tool that needs isatty() to return true.

Output (NDJSON on stdout, one message per line):
  {"type":"started","pid":1234}
  {"type":"line","stream":"stdout","text":"hello"}

Control (NDJSON on stdin, one message per line):
  {"type":"input","text":"password\n"}   Write bytes to the child's stdin.
  {"type":"resize","rows":40,"cols":120} Resize the PTY window.
  {"type":"stop"}                          Stop the process.

Signals:
  SIGINT/SIGTERM also stop the process.

Example:
  rein exec "sudo apt update"
  echo '{"type":"input","text":"password\n"}' | rein exec "sudo ls /root"
`

func execCmd(args []string) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	var (
		timeout         = fs.Duration("timeout", 0, "Maximum wall-clock duration (0 = no timeout)")
		gracefulTimeout = fs.Duration("graceful-timeout", 5*time.Second, "Time to wait between SIGTERM and SIGKILL")
		idleTimeout     = fs.Duration("idle-timeout", 0, "Kill the process if no output for this duration (0 = no idle timeout)")
		dir             = fs.String("dir", "", "Working directory")
		env             multiFlag
		lineBuffer      = fs.Int("line-buffer", 4096, "Size of the output channel buffer")
		initialSize     = fs.String("initial-size", "24x80", "Initial PTY window size as ROWSxCOLS")
	)
	fs.Var(&env, "env", "Environment variable in KEY=VALUE form (repeatable)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() == 0 {
		fmt.Fprint(os.Stderr, execUsage)
		os.Exit(1)
	}
	command := strings.Join(fs.Args(), " ")

	rows, cols, err := parseSize(*initialSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rein: invalid --initial-size %q: %v\n", *initialSize, err)
		os.Exit(2)
	}

	opts := []rein.Option{
		rein.WithPTY(),
		rein.WithGracefulTimeout(*gracefulTimeout),
		rein.WithLineBuffer(*lineBuffer),
	}
	if *timeout > 0 {
		opts = append(opts, rein.WithTimeout(*timeout))
	}
	if *idleTimeout > 0 {
		opts = append(opts, rein.WithIdleTimeout(*idleTimeout))
	}
	if *dir != "" {
		opts = append(opts, rein.WithDir(*dir))
	}
	if len(env) > 0 {
		opts = append(opts, rein.WithEnv(env))
	}

	session, err := rein.Start(context.Background(), command, opts...)
	if err != nil {
		sendErrorAndExit(err)
	}
	if err := session.Resize(rows, cols); err != nil {
		// Non-fatal: the session already started with a default
		// PTY size. Just report it on stderr.
		fmt.Fprintln(os.Stderr, "rein: resize failed:", err)
	}

	enc := ndjson.NewEncoder(os.Stdout)

	pid := session.PID()
	if err := enc.Encode(&ndjson.Message{Type: ndjson.TypeStarted, PID: &pid}); err != nil {
		fmt.Fprintln(os.Stderr, "rein: write failed:", err)
		os.Exit(1)
	}

	// forward lines to stdout as NDJSON
	go func() {
		for line := range session.Lines() {
			if err := enc.Encode(&ndjson.Message{
				Type:   ndjson.TypeLine,
				Stream: line.Stream,
				Text:   line.Text,
			}); err != nil {
				return
			}
		}
	}()

	// SIGINT/SIGTERM -> stop
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = session.Stop()
	}()

	// NDJSON control messages on stdin
	go func() {
		dec := ndjson.NewDecoder(bufio.NewReader(os.Stdin))
		for {
			msg, err := dec.Decode()
			if err == io.EOF {
				return
			}
			if err != nil {
				continue
			}
			switch msg.Type {
			case ndjson.TypeStop:
				_ = session.Stop()
				return
			case ndjson.TypeInput:
				if msg.Text != "" {
					if _, err := session.Write([]byte(msg.Text)); err != nil {
						fmt.Fprintln(os.Stderr, "rein: write to PTY failed:", err)
					}
				}
			case ndjson.TypeResize:
				if msg.Rows != nil && msg.Cols != nil {
					if err := session.Resize(*msg.Rows, *msg.Cols); err != nil {
						fmt.Fprintln(os.Stderr, "rein: resize failed:", err)
					}
				}
			}
		}
	}()

	result, _ := session.Wait()

	ec := result.ExitCode
	dur := result.Duration.Milliseconds()
	exitMsg := &ndjson.Message{
		Type:       ndjson.TypeExit,
		ExitCode:   &ec,
		DurationMS: &dur,
	}
	if result.Err != nil {
		exitMsg.Err = result.Err.Error()
	}
	if err := enc.Encode(exitMsg); err != nil {
		fmt.Fprintln(os.Stderr, "rein: write failed:", err)
		os.Exit(1)
	}

	if result.ExitCode != 0 {
		os.Exit(1)
	}
}

// parseSize parses a "ROWSxCOLS" string (e.g. "40x120").
func parseSize(s string) (int, int, error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected ROWSxCOLS, got %q", s)
	}
	rows, err := parsePositiveInt(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("rows: %w", err)
	}
	cols, err := parsePositiveInt(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("cols: %w", err)
	}
	return rows, cols, nil
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a positive integer: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return 0, fmt.Errorf("must be > 0: %q", s)
	}
	return n, nil
}

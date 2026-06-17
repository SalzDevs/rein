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

const startUsage = `rein start - start a long-running command, stream lines as NDJSON

Usage:
  rein start [options] <command>

Options:
  --timeout <duration>          Maximum wall-clock duration (0 = no timeout)
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL (default 5s)
  --idle-timeout <duration>     Kill the process if no output for this duration (0 = no idle timeout)
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --pty                         Allocate a pseudo-terminal
  --line-buffer <n>             Size of the output channel buffer (default 4096)

Output (NDJSON on stdout, one message per line):
  {"type":"started","pid":1234}
  {"type":"line","stream":"stdout","text":"hello"}
  {"type":"line","stream":"stderr","text":"warning"}
  {"type":"exit","exit_code":0,"duration_ms":1234}

Control (NDJSON on stdin, one message per line):
  {"type":"stop"}                Ask rein to stop the process.

Signals:
  SIGINT/SIGTERM also stop the process.

Example:
  rein start --idle-timeout 2m "npm run dev"
`

func startCmd(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	var (
		timeout         = fs.Duration("timeout", 0, "Maximum wall-clock duration (0 = no timeout)")
		gracefulTimeout = fs.Duration("graceful-timeout", 5*time.Second, "Time to wait between SIGTERM and SIGKILL")
		idleTimeout     = fs.Duration("idle-timeout", 0, "Kill the process if no output for this duration (0 = no idle timeout)")
		dir             = fs.String("dir", "", "Working directory")
		env             multiFlag
		pty             = fs.Bool("pty", false, "Allocate a pseudo-terminal")
		lineBuffer      = fs.Int("line-buffer", 4096, "Size of the output channel buffer")
	)
	fs.Var(&env, "env", "Environment variable in KEY=VALUE form (repeatable)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() == 0 {
		fmt.Fprint(os.Stderr, startUsage)
		os.Exit(1)
	}
	command := strings.Join(fs.Args(), " ")

	opts := []rein.Option{
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
	if *pty {
		opts = append(opts, rein.WithPTY())
	}

	session, err := rein.Start(context.Background(), command, opts...)
	if err != nil {
		sendErrorAndExit(err)
	}

	enc := ndjson.NewEncoder(os.Stdout)

	// started
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
				// stdout is closed; nothing to do
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
			if msg.Type == ndjson.TypeStop {
				_ = session.Stop()
				return
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

// sendErrorAndExit emits an error NDJSON message and exits with
// a non-zero status.
func sendErrorAndExit(err error) {
	msg := &ndjson.Message{
		Type: ndjson.TypeError,
		Err:  err.Error(),
	}
	enc := ndjson.NewEncoder(os.Stdout)
	_ = enc.Encode(msg)
	os.Exit(1)
}

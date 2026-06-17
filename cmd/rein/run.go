package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SalzDevs/rein"
	"github.com/SalzDevs/rein/internal/ndjson"
)

const runUsage = `rein run - run a command to completion, print the result as NDJSON

Usage:
  rein run [options] <command>

Options:
  --timeout <duration>          Maximum wall-clock duration (0 = no timeout)
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL (default 5s)
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --pty                         Allocate a pseudo-terminal

Output:
  A single NDJSON line on stdout:
    {"type":"exit","exit_code":0,"duration_ms":1234,"stdout":"...","stderr":"..."}

Example:
  rein run --timeout 30s "echo hello"
  rein run "npm install"
`

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var (
		timeout         = fs.Duration("timeout", 0, "Maximum wall-clock duration (0 = no timeout)")
		gracefulTimeout = fs.Duration("graceful-timeout", 5*time.Second, "Time to wait between SIGTERM and SIGKILL")
		dir             = fs.String("dir", "", "Working directory")
		env             multiFlag
		pty             = fs.Bool("pty", false, "Allocate a pseudo-terminal")
	)
	fs.Var(&env, "env", "Environment variable in KEY=VALUE form (repeatable)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() == 0 {
		fmt.Fprint(os.Stderr, runUsage)
		os.Exit(1)
	}
	command := strings.Join(fs.Args(), " ")

	opts := []rein.Option{
		rein.WithGracefulTimeout(*gracefulTimeout),
	}
	if *timeout > 0 {
		opts = append(opts, rein.WithTimeout(*timeout))
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

	result, _ := rein.Run(context.Background(), command, opts...)

	ec := result.ExitCode
	dur := result.Duration.Milliseconds()
	msg := &ndjson.Message{
		Type:       ndjson.TypeExit,
		ExitCode:   &ec,
		DurationMS: &dur,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
	}
	if result.Err != nil {
		msg.Err = result.Err.Error()
	}

	enc := ndjson.NewEncoder(os.Stdout)
	if err := enc.Encode(msg); err != nil {
		fmt.Fprintln(os.Stderr, "rein: failed to write NDJSON:", err)
		os.Exit(1)
	}

	if result.ExitCode != 0 {
		os.Exit(1)
	}
}

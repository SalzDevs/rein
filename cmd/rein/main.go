// Command rein is the CLI for the rein shell-layer library.
//
// rein is a small tool that runs shell commands the way AI agents
// actually need: with timeouts that work, signals that propagate,
// and process trees that get cleaned up.
//
// Subcommands:
//
//	run       Run a command to completion, print the result as NDJSON.
//	start     Start a long-running command, stream lines as NDJSON.
//	version   Print the version.
//	help      Print this help.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// version is set at build time via -ldflags, or falls back to the
// value below for plain `go build`.
var version = "0.0.2"

const usageText = `rein - the shell layer AI agents don't break

Usage:
  rein <command> [arguments]

Commands:
  run       Run a command to completion, print the result as NDJSON
  start     Start a long-running command, stream lines as NDJSON
  exec      Run a command with a PTY, drive it interactively via NDJSON
  daemon    Long-running multi-session manager (NDJSON over stdio)
  version   Print the version
  help      Print this help

Run 'rein <command> -h' for command-specific help.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "start":
		startCmd(os.Args[2:])
	case "exec":
		execCmd(os.Args[2:])
	case "daemon":
		runDaemon()
	case "version", "-v", "--version":
		fmt.Println("rein " + version)
	case "help", "-h", "--help":
		fmt.Print(usageText)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(1)
	}
}

// multiFlag is a flag.Value that accumulates repeated string flags.
type multiFlag []string

func (m *multiFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m, ", ")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// flagOrStdinFlag is a flag.Value that reads from stdin if the value
// is "-". It is used by 'start' so a caller can pipe in commands or
// control messages from another process. Unused for v0.1; reserved
// for the future.
var _ = (*flag.Flag)(nil)

// Package rein provides a small, reliable shell layer for AI agents.
//
// rein runs shell commands with timeouts that work, signals that
// propagate, and process trees that get cleaned up when the context
// is cancelled. It is designed to be the foundation underneath any
// agent framework that needs to execute commands.
package rein

import "time"

// Result is the outcome of a command run by Run or Start.
//
// Result is always returned, even on failure. Callers should check
// Err to determine whether the command succeeded.
type Result struct {
	// Stdout is the standard output captured during execution.
	Stdout string

	// Stderr is the standard error captured during execution.
	Stderr string

	// ExitCode is the process exit code. It is -1 if the process did
	// not exit normally (for example, it was killed by a signal).
	ExitCode int

	// Duration is the wall-clock time from start to exit.
	Duration time.Duration

	// Err is the terminal error, if any. It is nil on success.
	// For non-zero exit codes, Err is set to a non-nil error that
	// describes the failure. For timeouts, it indicates how long the
	// command ran before being killed.
	Err error
}

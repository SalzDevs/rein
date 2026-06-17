package rein

import (
	"io"
	"os/exec"
)

// startWithPTY starts the command attached to a pseudo-terminal
// and returns a reader for the merged stdout/stderr stream.
// The command is started as part of the PTY allocation.
//
// Only implemented on POSIX in v0.0.1. On Windows, this returns
// [ErrPTYNotSupported].
func startWithPTY(cmd *exec.Cmd) (io.Reader, error) {
	return startWithPTYPlatform(cmd)
}

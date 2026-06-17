package rein

import (
	"io"
	"os"
	"os/exec"
)

// startWithPTY starts the command attached to a pseudo-terminal
// and returns the master as an *os.File. The master is also an
// io.Reader (the merged stdout/stderr stream on the TTY).
//
// Only implemented on POSIX in v0.0.1. On Windows, this returns
// [ErrPTYNotSupported].
func startWithPTY(cmd *exec.Cmd) (*os.File, error) {
	return startWithPTYPlatform(cmd)
}

// resizePTY resizes the PTY window. Only implemented on POSIX in
// v0.0.1; on Windows it returns ErrPTYNotSupported.
func resizePTY(master *os.File, rows, cols int) error {
	return resizePTYPlatform(master, rows, cols)
}

// _ keeps io referenced for future use.
var _ = (*io.Reader)(nil)

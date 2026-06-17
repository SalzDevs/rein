//go:build !windows

package rein

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// startWithPTYPlatform allocates a pseudo-terminal, starts the
// command attached to it, and returns the master as an *os.File.
// stderr is merged with stdout because they share the same TTY.
func startWithPTYPlatform(cmd *exec.Cmd) (*os.File, error) {
	master, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("rein: failed to start with pty: %w", err)
	}
	return master, nil
}

// resizePTYPlatform resizes the PTY window to rows x cols by
// sending the TIOCSWINSZ ioctl to the master file descriptor.
func resizePTYPlatform(master *os.File, rows, cols int) error {
	return pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

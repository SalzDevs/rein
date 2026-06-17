//go:build !windows

package rein

import (
	"fmt"
	"os/exec"

	"github.com/creack/pty"
)

// startWithPTYPlatform allocates a pseudo-terminal, starts the
// command attached to it, and returns the master state. stderr is
// merged with stdout because they share the same TTY.
func startWithPTYPlatform(cmd *exec.Cmd) (*ptyState, error) {
	master, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("rein: failed to start with pty: %w", err)
	}
	return &ptyState{
		read:  master,
		write: master,
		resize: func(rows, cols int) error {
			return pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
		},
	}, nil
}

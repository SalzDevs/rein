//go:build !windows

package rein

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/creack/pty"
)

// startWithPTYPlatform allocates a pseudo-terminal, starts the
// command attached to it, and returns the master as an io.Reader.
// stderr is merged with stdout because they share the same TTY.
func startWithPTYPlatform(cmd *exec.Cmd) (io.Reader, error) {
	master, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("rein: failed to start with pty: %w", err)
	}
	return master, nil
}

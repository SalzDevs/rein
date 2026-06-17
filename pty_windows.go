//go:build windows

package rein

import (
	"errors"
	"os"
	"os/exec"
)

// startWithPTYPlatform returns ErrPTYNotSupported on Windows in
// v0.0.1. PTY support on Windows requires ConPTY (introduced in
// Windows 10 1809), which is more involved than the POSIX path.
// See the roadmap for when this lands.
func startWithPTYPlatform(cmd *exec.Cmd) (*os.File, error) {
	return nil, errors.New("rein: PTY is not yet supported on Windows")
}

// resizePTYPlatform returns ErrPTYNotSupported on Windows in
// v0.0.1. Resizing a ConPTY-based PTY requires a separate
// ioctl-equivalent call (ResizePseudoConsole) that is not yet
// wired up.
func resizePTYPlatform(master *os.File, rows, cols int) error {
	return errors.New("rein: PTY is not yet supported on Windows")
}

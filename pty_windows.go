//go:build windows

package rein

import (
	"errors"
	"io"
	"os/exec"
)

// startWithPTYPlatform returns ErrPTYNotSupported on Windows in
// v0.0.1. PTY support on Windows requires ConPTY (introduced in
// Windows 10 1809), which is more involved than the POSIX path.
// See the roadmap for when this lands.
func startWithPTYPlatform(cmd *exec.Cmd) (io.Reader, error) {
	return nil, errors.New("rein: PTY is not yet supported on Windows")
}

// _ keeps cmd referenced for the Windows build.
var _ = (*exec.Cmd)(nil)

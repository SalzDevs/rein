//go:build windows

package rein

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

const (
	procThreadAttributePseudoConsole = 0x00020016
	extStartupInfoPresent            = 0x00080000
)

// startWithPTYPlatform is a stub for Windows. Real ConPTY support
// requires calling CreatePseudoConsole, CreatePipe, and
// CreateProcess with an extended STARTUPINFOEX — but the
// golang.org/x/sys/windows package does not expose the
// necessary CreatePseudoConsole / NewStartupInfoEx /
// UpdateProcThreadAttribute / DeleteStartupInfoEx symbols
// publicly. Implementing ConPTY from scratch would require
// either:
//
//  1. Importing an external ConPTY library
//     (e.g. github.com/UserExistsError/conpty).
//  2. Using a private/internal ConPTY binding.
//
// Both are tracked in the roadmap. For v0.1.0 we return a
// clear error so the caller knows PTY is not yet supported on
// Windows; non-PTY sessions (the default) work fine on Windows.
func startWithPTYPlatform(cmd *exec.Cmd) (*ptyState, error) {
	_ = procThreadAttributePseudoConsole
	_ = extStartupInfoPresent
	return nil, fmt.Errorf("rein: PTY is not yet supported on Windows (use non-PTY sessions, or track https://github.com/SalzDevs/rein/issues)")
}

// resizePTYPlatform is a stub for Windows. See startWithPTYPlatform.
func resizePTYPlatform(master *ptyState, rows, cols int) error {
	return errors.New("rein: PTY resize is not yet supported on Windows")
}

// _ keeps the imports referenced.
var _ = (*os.File)(nil)
var _ = windows.PROCESS_TERMINATE

//go:build windows

package signal

import (
	"os/exec"
	"syscall"
)

// processGroup is a best-effort fallback on Windows. POSIX-style
// process groups do not exist; the signal is sent to the leader
// process only. Grandchildren may leak. See the procgroup package
// for the full Windows story.
func processGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	return cmd.Process.Signal(sig)
}

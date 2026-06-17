//go:build windows

package signal

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// processGroup translates rein's POSIX "send to process group" to
// the Windows equivalent: TerminateProcess on the leader (Windows
// has no POSIX-style process groups, so the leader is the best we
// can do). For full process-tree cleanup, see the Job Object
// configured by internal/procgroup.
func processGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// Translate the POSIX signal into a Windows termination.
	// We use TerminateProcess with a non-zero exit code that
	// encodes the signal number for diagnostic purposes.
	// SIGTERM (15) -> exit 15, SIGKILL (9) -> exit 9.
	var exitCode uint32
	if sig == syscall.SIGTERM {
		exitCode = 15
	} else if sig == syscall.SIGKILL {
		exitCode = 9
	} else {
		exitCode = uint32(sig)
	}
	// We use OpenProcess + TerminateProcess rather than the
	// (*os.Process).Handle() method because the Handle() method
	// is only available on Go 1.20+; using OpenProcess keeps us
	// compatible with the older Go versions that some CI
	// runners are stuck on.
	pid := uint32(cmd.Process.Pid)
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, exitCode)
}

// Package signal sends signals to a child process group.
//
// The rein package uses this to deliver SIGTERM (and later SIGKILL)
// to every process in a child process tree, not just the leader.
package signal

import (
	"os/exec"
	"syscall"
)

// ProcessGroup sends sig to the entire process group of cmd.
//
// On POSIX, this is done by passing a negative PID to syscall.Kill,
// which the kernel interprets as a PGID. The caller must have
// configured the child as a process group leader (see the
// internal/procgroup package).
//
// On Windows, where POSIX-style process groups do not exist, the
// signal is sent only to the leader process. The internal/procgroup
// package documents this limitation.
func ProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return processGroup(cmd, sig)
}

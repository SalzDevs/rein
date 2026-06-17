//go:build !windows

package signal

import (
	"os/exec"
	"syscall"
)

// processGroup sends sig to the entire process group of cmd by
// passing -PID to syscall.Kill. The negative sign tells the kernel
// to interpret the argument as a PGID rather than a PID.
func processGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	return syscall.Kill(-cmd.Process.Pid, sig)
}

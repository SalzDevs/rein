//go:build !windows

package procgroup

import (
	"os/exec"
	"syscall"
)

// apply sets the Setpgid flag so the child becomes the leader of a
// new process group with PGID equal to its PID. Once the child is
// the leader, sending a signal to -PID reaches every process in the
// group, including grandchildren.
func apply(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

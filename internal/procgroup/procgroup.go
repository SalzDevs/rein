// Package procgroup isolates a child process into its own process
// group so that signals can be delivered to the entire process tree,
// not just the leader.
package procgroup

import "os/exec"

// Apply configures cmd to start in its own process group. The
// platform-specific implementation is in procgroup_posix.go or
// procgroup_windows.go.
func Apply(cmd *exec.Cmd) {
	apply(cmd)
}

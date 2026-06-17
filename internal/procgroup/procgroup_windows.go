//go:build windows

package procgroup

import "os/exec"

// apply is a no-op on Windows in v0.0.1.
//
// On Windows, process group isolation requires a Job Object with
// the JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE flag set, which is more
// involved than the POSIX Setpgid flag. The signal package's
// Windows fallback signals only the leader process; leaked
// grandchildren may remain. Use a process supervisor on Windows
// until proper Job Object support is added.
func apply(cmd *exec.Cmd) {
	_ = cmd
}

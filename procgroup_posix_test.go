//go:build !windows

package rein

import "syscall"

// processIsAlive returns nil if the process is alive, an error
// otherwise. On POSIX this is syscall.Kill(pid, 0) which returns
// ESRCH for dead processes.
func processIsAlive(pid int) error {
	return syscall.Kill(pid, 0)
}

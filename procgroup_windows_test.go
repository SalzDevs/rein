//go:build windows

package rein

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// processIsAlive returns nil if the process is alive, an error
// otherwise. On Windows this uses OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION;
// a non-existent process returns ERROR_INVALID_PARAMETER or
// ERROR_ACCESS_DENIED. We treat both as "not alive".
func processIsAlive(pid int) error {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		// ESRCH is the POSIX equivalent.
		return syscall.ESRCH
	}
	windows.CloseHandle(handle)
	return nil
}

// _ keeps errors referenced for future use.
var _ = errors.New

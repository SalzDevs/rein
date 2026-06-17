package rein

import (
	"io"
	"os"
	"os/exec"
)

// ptyState holds the resources for a session running in a PTY.
// It is platform-specific: on POSIX, read and write point to the
// same master file; on Windows, they point to separate pipes
// and resize is implemented via ConPTY.
type ptyState struct {
	read   *os.File
	write  *os.File
	resize func(rows, cols int) error
}

func (p *ptyState) Read(b []byte) (int, error)  { return p.read.Read(b) }
func (p *ptyState) Write(b []byte) (int, error) { return p.write.Write(b) }

// startWithPTY starts the command attached to a pseudo-terminal
// and returns the master state. Only implemented on POSIX in
// the first cut; on Windows, see pty_windows.go.
func startWithPTY(cmd *exec.Cmd) (*ptyState, error) {
	return startWithPTYPlatform(cmd)
}

// resizePTY resizes the PTY window. Only implemented on POSIX in
// the first cut; on Windows, see pty_windows.go.
func resizePTY(master *ptyState, rows, cols int) error {
	if master == nil {
		return errPTYRequired
	}
	return master.resize(rows, cols)
}

// errPTYRequired is returned when a PTY-dependent operation is
// called on a session that was not started with WithPTY.
var errPTYRequired = errPTY("rein: session is not running with a PTY; use WithPTY")

type errPTY string

func (e errPTY) Error() string { return string(e) }

// _ keeps io referenced for future use.
var _ = (*io.Reader)(nil)

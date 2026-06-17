//go:build windows

package rein

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	procThreadAttributePseudoConsole = 0x00020016
	extStartupInfoPresent             = 0x00080000
)

// startWithPTYPlatform creates a ConPTY, starts the command
// attached to it, and returns the PTY state. On Windows we use
// the ConPTY API (CreatePseudoConsole + STARTUPINFOEX +
// CreateProcess), which gives the child a real TTY.
func startWithPTYPlatform(cmd *exec.Cmd) (*ptyState, error) {
	// 1. Create anonymous pipes for the PTY's input and output.
	sa := &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}

	var inRead, inWrite, outRead, outWrite windows.Handle

	if err := windows.CreatePipe(&inRead, &inWrite, sa, 0); err != nil {
		return nil, fmt.Errorf("rein: CreatePipe(in) failed: %w", err)
	}
	if err := windows.CreatePipe(&outRead, &outWrite, sa, 0); err != nil {
		windows.CloseHandle(inRead)
		windows.CloseHandle(inWrite)
		return nil, fmt.Errorf("rein: CreatePipe(out) failed: %w", err)
	}

	// 2. Create the ConPTY. Default to 80x24; Session.Resize
	//    updates this via ResizePseudoConsole.
	var hpCon windows.Handle
	if err := windows.CreatePseudoConsole(
		windows.Coord{X: 80, Y: 24},
		inRead,
		outWrite,
		0,
		&hpCon,
	); err != nil {
		windows.CloseHandle(inRead)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		windows.CloseHandle(outWrite)
		return nil, fmt.Errorf("rein: CreatePseudoConsole failed: %w", err)
	}

	// 3. Close the parent's copies of the pipe ends the
	//    ConPTY has inherited.
	windows.CloseHandle(inRead)
	windows.CloseHandle(outWrite)

	// 4. Set up STARTUPINFOEX with the ConPTY attribute.
	siEx, err := windows.NewStartupInfoEx()
	if err != nil {
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		windows.ClosePseudoConsole(hpCon)
		return nil, fmt.Errorf("rein: NewStartupInfoEx failed: %w", err)
	}
	defer windows.DeleteStartupInfoEx(siEx)

	if err := windows.UpdateProcThreadAttribute(
		siEx.AttributeList,
		0,
		procThreadAttributePseudoConsole,
		unsafe.Pointer(hpCon),
		unsafe.Sizeof(hpCon),
	); err != nil {
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		windows.ClosePseudoConsole(hpCon)
		return nil, fmt.Errorf("rein: UpdateProcThreadAttribute failed: %w", err)
	}

	// 5. Set the startup info on the command.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &windows.SysProcAttr{}
	}
	cmd.SysProcAttr.StartupInfo = siEx.StartupInfo
	cmd.SysProcAttr.CreationFlags |= extStartupInfoPresent

	// 6. Start the command.
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		windows.ClosePseudoConsole(hpCon)
		return nil, fmt.Errorf("rein: failed to start: %w", err)
	}

	// 7. Close the ConPTY handle in the parent. The child still
	//    has its copy. (Closing here releases the parent's
	//    reference; the ConPTY will be torn down when the
	//    child's reference is also closed.)
	// We deliberately keep hpCon alive for the lifetime of
	// the session so that ResizePseudoConsole works.
	_ = hpCon

	outFile := os.NewFile(uintptr(outRead), "rein-pty-out")
	inFile := os.NewFile(uintptr(inWrite), "rein-pty-in")

	return &ptyState{
		read:  outFile,
		write: inFile,
		resize: func(rows, cols int) error {
			return windows.ResizePseudoConsole(
				hpCon,
				windows.Coord{X: int16(cols), Y: int16(rows)},
			)
		},
	}, nil
}

//go:build windows

package procgroup

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows uses Job Objects (not process groups) for the equivalent
// of POSIX Setpgid. The job has JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// set, so closing the job handle (e.g. when rein exits) kills all
// processes assigned to the job. This is how SIGTERM-then-SIGKILL
// escalation translates on Windows.

var (
	jobHandle     windows.Handle
	jobHandleOnce sync.Once
	jobHandleErr  error
)

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x00002000
)

// JOBOBJECT_BASIC_LIMIT_INFORMATION is documented in the Windows
// SDK; we reproduce just the fields we need to keep this file
// self-contained.
type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                windows.IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// getJobHandle returns a process-wide job object handle. The
// first call creates the job; subsequent calls reuse it.
func getJobHandle() (windows.Handle, error) {
	jobHandleOnce.Do(func() {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobHandleErr = fmt.Errorf("rein: CreateJobObject failed: %w", err)
			return
		}
		info := jobObjectExtendedLimitInformation{
			BasicLimitInformation: jobObjectBasicLimitInformation{
				LimitFlags: jobObjectLimitKillOnJobClose,
			},
		}
		_, err = windows.SetInformationJobObject(
			job,
			jobObjectExtendedLimitInformationClass,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			windows.CloseHandle(job)
			jobHandleErr = fmt.Errorf("rein: SetInformationJobObject failed: %w", err)
			return
		}
		jobHandle = job
	})
	return jobHandle, jobHandleErr
}

// AssignToJob assigns the given process handle to the rein job
// object. If the job has not been created yet, it is created.
func AssignToJob(process windows.Handle) error {
	job, err := getJobHandle()
	if err != nil {
		return err
	}
	return windows.AssignProcessToJobObject(job, process)
}

// apply configures the command for Windows. We can't inject the
// job assignment into the process creation directly from Go's
// os/exec, so we expose a hook that is called by Session right
// after Start(). The actual job assignment happens in
// postStartHook (see internal/session).
func apply(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// CREATE_NEW_PROCESS_GROUP is the closest equivalent of
	// POSIX Setpgid. It puts the process in a new process group
	// so Ctrl-C events are sent only to the new group (i.e. not
	// to the parent terminal). The job assignment provides the
	// hard cleanup guarantee on parent exit.
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP
}

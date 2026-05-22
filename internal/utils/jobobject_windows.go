//go:build windows

package utils

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	globalJobOnce sync.Once
	globalJob     windows.Handle
)

// InitJobObject must be called once at app startup to create the global
// Job Object. All long-running subprocesses should then be started via
// StartCmd so they are assigned to it.
func InitJobObject() {
	initJobObject()
}

// initJobObject creates the global Job Object once at app startup.
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE causes Windows to automatically kill
// all assigned processes when the Job Object handle is closed — which happens
// when the VT process exits for any reason, including unclean crashes.
// This prevents zombie FFmpeg processes from persisting after VT closes.
func initJobObject() {
	globalJobOnce.Do(func() {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		ext := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err := windows.SetInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&ext)),
			uint32(unsafe.Sizeof(ext)),
		); err != nil {
			windows.CloseHandle(job)
			return
		}
		globalJob = job
	})
}

// assignToJobObject assigns the given process handle to the global Job Object.
// Silent no-op if the Job Object was not created successfully.
func assignToJobObject(ph windows.Handle) {
	if globalJob == 0 {
		return
	}
	_ = windows.AssignProcessToJobObject(globalJob, ph)
}

// StartCmd starts cmd and, on Windows, assigns the child process to the
// global Job Object so it is killed automatically if VT exits or crashes.
// Use this in place of cmd.Start() for long-running subprocesses.
func StartCmd(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process == nil {
		return nil
	}
	ph, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		return nil // non-fatal: process tracking best-effort
	}
	defer windows.CloseHandle(ph)
	assignToJobObject(ph)
	return nil
}

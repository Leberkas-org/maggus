package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// detachedProcess is the Windows DETACHED_PROCESS creation flag (0x00000008).
// It ensures the child process has no console window.
const detachedProcess = 0x00000008

// launchDaemon starts the daemon process on Windows, detached from the current
// console. DETACHED_PROCESS ensures no console window is inherited or created,
// and CREATE_NEW_PROCESS_GROUP puts the child in its own process group.
func launchDaemon(exe string, args []string, logFile *os.File, dir string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

// isProcessRunning returns true if the process with the given PID is still alive.
// On Windows we open a SYNCHRONIZE handle and do a zero-timeout wait:
// WAIT_TIMEOUT (258) means the process is still running.
func isProcessRunning(pid int) bool {
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	// WaitForSingleObject with timeout 0: returns WAIT_TIMEOUT if still running.
	const waitTimeout = 258 // WAIT_TIMEOUT
	r, _ := syscall.WaitForSingleObject(handle, 0)
	return r == waitTimeout
}

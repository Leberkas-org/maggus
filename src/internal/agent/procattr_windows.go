package agent

import (
	"os/exec"
	"syscall"
)

// setProcAttr puts the child process in a new process group on Windows.
// This prevents Ctrl+C from being sent directly to the child — Go handles
// the signal exclusively and kills the child via cmd.Cancel.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

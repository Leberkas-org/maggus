package agent

import (
	"os/exec"
	"syscall"
)

// setProcAttr configures Windows-specific process attributes for the child process.
// CREATE_NEW_PROCESS_GROUP prevents Ctrl+C from being sent directly to the child —
// Go handles the signal exclusively and kills the child via cmd.Cancel.
// CREATE_NO_WINDOW (0x08000000) suppresses the visible console window, keeping
// daemon and background task execution fully silent.
func setProcAttr(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

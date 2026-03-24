package gitcommit

import (
	"os/exec"
	"syscall"
)

// setProcAttr configures Windows-specific process attributes for git subprocesses.
// CREATE_NO_WINDOW (0x08000000) suppresses the visible console window that would
// otherwise flash briefly for each git invocation.
func setProcAttr(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
	}
}

package gitutil

import "os/exec"

// Command creates an *exec.Cmd for a git subprocess with the given arguments.
// On Windows it applies CREATE_NO_WINDOW to suppress console window flicker.
func Command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	setProcAttr(cmd)
	return cmd
}

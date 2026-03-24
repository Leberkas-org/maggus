//go:build !windows

package gitbranch

import "os/exec"

// setProcAttr is a no-op on non-Windows platforms.
func setProcAttr(_ *exec.Cmd) {}

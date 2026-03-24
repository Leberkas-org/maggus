//go:build !windows

package gitutil

import "os/exec"

// setProcAttr is a no-op on non-Windows platforms.
func setProcAttr(_ *exec.Cmd) {}

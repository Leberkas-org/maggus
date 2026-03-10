//go:build !windows

package runner

import "os/exec"

// setProcAttr is a no-op on non-Windows platforms.
// Unix signal handling works correctly without special process group setup.
func setProcAttr(_ *exec.Cmd) {}

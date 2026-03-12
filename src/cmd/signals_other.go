//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// shutdownSignals lists the OS signals used to trigger graceful shutdown.
// On Unix/macOS, both SIGINT (Ctrl+C) and SIGTERM (kill) are handled.
var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

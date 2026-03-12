package cmd

import "os"

// shutdownSignals lists the OS signals used to trigger graceful shutdown.
// On Windows, only os.Interrupt is available (SIGTERM is not supported).
var shutdownSignals = []os.Signal{os.Interrupt}

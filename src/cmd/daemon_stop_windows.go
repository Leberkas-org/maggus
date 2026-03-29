package cmd

import (
	"os"
	"os/exec"
	"strconv"
)

// sendGracefulSignal on Windows creates a stop signal file that the daemon
// watches for. The traditional taskkill/WM_CLOSE approach does not work because
// the daemon is launched with DETACHED_PROCESS (no console window).
func sendGracefulSignal(pid int, dir string) error {
	return os.WriteFile(daemonStopFilePath(dir), []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// forceKill on Windows uses taskkill /T /F to forcibly terminate the process
// and all of its child processes.
func forceKill(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

package cmd

import (
	"os/exec"
	"strconv"
)

// sendGracefulSignal on Windows uses taskkill without /F for a gentle
// termination request (sends WM_CLOSE to the process).
func sendGracefulSignal(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid)).Run()
}

// forceKill on Windows uses taskkill /T /F to forcibly terminate the process
// and all of its child processes.
func forceKill(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

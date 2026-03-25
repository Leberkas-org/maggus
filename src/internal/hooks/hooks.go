package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/leberkas-org/maggus/internal/config"
)

// Event is the JSON payload written to each hook command's stdin.
type Event struct {
	Type      string     `json:"event"`
	File      string     `json:"file"`
	MaggusID  string     `json:"maggus_id"`
	Title     string     `json:"title"`
	Action    string     `json:"action"`
	Tasks     []TaskInfo `json:"tasks"`
	Timestamp string     `json:"timestamp"`
}

// TaskInfo describes a single task within a feature or bug.
type TaskInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

const hookTimeout = 30 * time.Second

// Run executes each hook command sequentially, writing the JSON-encoded event
// to its stdin. Failures are logged as warnings; execution always continues.
func Run(commands []config.HookEntry, event Event, workDir string, logger *log.Logger) {
	if len(commands) == 0 {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		logger.Printf("[hooks] WARNING: failed to marshal event: %v", err)
		return
	}

	for _, entry := range commands {
		runOneWithTimeout(entry.Run, payload, workDir, logger, hookTimeout)
	}
}

func runOneWithTimeout(cmdStr string, payload []byte, workDir string, logger *log.Logger, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
			return kill.Run()
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 3 * time.Second

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := fmt.Sprintf("[hooks] WARNING: command %q failed: %v", cmdStr, err)
		if stderr.Len() > 0 {
			msg += fmt.Sprintf(" (stderr: %s)", stderr.String())
		}
		logger.Println(msg)
		return
	}

	if stderr.Len() > 0 {
		logger.Printf("[hooks] WARNING: command %q stderr: %s", cmdStr, stderr.String())
	}
}

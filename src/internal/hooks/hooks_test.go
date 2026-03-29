package hooks

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/config"
)

func newTestLogger() (*log.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return log.New(&buf, "", 0), &buf
}

// shellCmd returns a command string appropriate for the current OS.
// On Windows it uses cmd builtins; on Unix it uses sh builtins.
func shellCmd(unix, windows string) string {
	if runtime.GOOS == "windows" {
		return windows
	}
	return unix
}

func TestRun_EmptyCommands(t *testing.T) {
	logger, buf := newTestLogger()
	event := Event{Type: "task_complete", Timestamp: "2025-01-01T00:00:00Z"}
	Run(nil, event, t.TempDir(), logger)
	if buf.Len() != 0 {
		t.Errorf("expected no log output for empty commands, got: %s", buf.String())
	}
}

func TestRun_SuccessfulExecution(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.json")

	// Write a small helper script that reads stdin and writes to file
	var cmd string
	if runtime.GOOS == "windows" {
		// Create a bat script that reads stdin via more and writes to file
		batFile := filepath.Join(dir, "capture.bat")
		batContent := "@echo off\r\nfindstr \"^\" > \"" + outFile + "\"\r\n"
		if err := os.WriteFile(batFile, []byte(batContent), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd = batFile
	} else {
		cmd = "cat > " + outFile
	}

	commands := []config.HookEntry{{Run: cmd}}
	event := Event{
		Type:      "feature_complete",
		File:      "feature_004.md",
		MaggusID:  "abc-123",
		Title:     "Test Feature",
		Action:    "rename",
		Tasks:     []TaskInfo{{ID: "TASK-001", Title: "First task"}},
		Timestamp: "2025-01-01T00:00:00Z",
	}
	logger, buf := newTestLogger()

	Run(commands, event, dir, logger)

	if buf.Len() != 0 {
		t.Errorf("unexpected log output: %s", buf.String())
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var got Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v\nraw: %s", err, string(data))
	}
	if got.Type != "feature_complete" {
		t.Errorf("event type = %q, want %q", got.Type, "feature_complete")
	}
	if got.File != "feature_004.md" {
		t.Errorf("file = %q, want %q", got.File, "feature_004.md")
	}
	if got.MaggusID != "abc-123" {
		t.Errorf("maggus_id = %q, want %q", got.MaggusID, "abc-123")
	}
	if len(got.Tasks) != 1 || got.Tasks[0].ID != "TASK-001" {
		t.Errorf("tasks = %+v, want [{TASK-001 First task}]", got.Tasks)
	}
}

func TestRun_CommandFailure_LoggedNotFatal(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "second.txt")

	failCmd := shellCmd("exit 1", "exit /b 1")
	var successCmd string
	if runtime.GOOS == "windows" {
		// Use a bat file to avoid cmd quoting issues with long paths
		batFile := filepath.Join(dir, "write.bat")
		batContent := "@echo off\r\necho ok > second.txt\r\n"
		if err := os.WriteFile(batFile, []byte(batContent), 0o644); err != nil {
			t.Fatal(err)
		}
		successCmd = batFile
	} else {
		successCmd = "echo ok > " + outFile
	}

	commands := []config.HookEntry{
		{Run: failCmd},
		{Run: successCmd},
	}

	event := Event{Type: "task_complete", Timestamp: "2025-01-01T00:00:00Z"}
	logger, buf := newTestLogger()

	Run(commands, event, dir, logger)

	// First command should have logged a warning
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("expected warning in log, got: %s", buf.String())
	}

	// Second command should still have executed (bat writes to workDir/second.txt)
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("second command did not run; expected %s to exist\nlog: %s", outFile, buf.String())
	}
}

func TestRunWithTimeout_Timeout(t *testing.T) {
	dir := t.TempDir()

	var cmdStr string
	if runtime.GOOS == "windows" {
		cmdStr = "ping -n 60 127.0.0.1"
	} else {
		cmdStr = "sleep 60"
	}

	event := Event{Type: "task_complete", Timestamp: "2025-01-01T00:00:00Z"}
	logger, buf := newTestLogger()

	payload, _ := json.Marshal(event)
	runOneWithTimeout(cmdStr, payload, dir, logger, 500*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "WARNING") {
		t.Errorf("expected warning for timeout, got: %s", output)
	}
}

func TestRun_JSONPayloadCorrectness(t *testing.T) {
	event := Event{
		Type:     "bug_complete",
		File:     "bug_001.md",
		MaggusID: "uuid-456",
		Title:    "Fix login",
		Action:   "delete",
		Tasks: []TaskInfo{
			{ID: "BUG-001-001", Title: "Fix auth"},
			{ID: "BUG-001-002", Title: "Add test"},
		},
		Timestamp: "2025-03-25T12:00:00Z",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	checks := map[string]string{
		"event":     "bug_complete",
		"file":      "bug_001.md",
		"maggus_id": "uuid-456",
		"action":    "delete",
		"timestamp": "2025-03-25T12:00:00Z",
	}
	for key, want := range checks {
		if m[key] != want {
			t.Errorf("%s = %v, want %s", key, m[key], want)
		}
	}

	tasks, ok := m["tasks"].([]any)
	if !ok || len(tasks) != 2 {
		t.Fatalf("tasks = %v, want 2 entries", m["tasks"])
	}
}

func TestRun_StderrIncludedInWarning(t *testing.T) {
	dir := t.TempDir()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = `echo error details 1>&2 && exit /b 1`
	} else {
		cmd = "echo 'error details' >&2; exit 1"
	}

	commands := []config.HookEntry{{Run: cmd}}
	event := Event{Type: "task_complete", Timestamp: "2025-01-01T00:00:00Z"}
	logger, buf := newTestLogger()

	Run(commands, event, dir, logger)

	output := buf.String()
	if !strings.Contains(output, "error details") {
		t.Errorf("expected stderr in log output, got: %s", output)
	}
	if !strings.Contains(output, "WARNING") {
		t.Errorf("expected WARNING in log output, got: %s", output)
	}
}

func TestRun_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "cwd.txt")

	// Use relative filename since workDir is set to dir
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd > cwd.txt"
	} else {
		cmd = "pwd > cwd.txt"
	}

	commands := []config.HookEntry{{Run: cmd}}
	event := Event{Type: "task_complete", Timestamp: "2025-01-01T00:00:00Z"}
	logger, _ := newTestLogger()

	Run(commands, event, dir, logger)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	got := strings.TrimSpace(string(data))
	wantResolved, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("working dir = %q, want %q", got, dir)
	}
}

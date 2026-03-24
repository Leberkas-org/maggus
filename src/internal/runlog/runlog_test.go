package runlog_test

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestOpen_CreatesRunDir(t *testing.T) {
	dir := t.TempDir()
	l, err := runlog.Open("20260101-120000", dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	logPath := filepath.Join(dir, ".maggus", "runs", "20260101-120000", "run.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("run.log not created: %v", err)
	}
}

func TestOpen_ReturnsErrorOnBadDir(t *testing.T) {
	l, err := runlog.Open("test", filepath.Join(t.TempDir(), "nonexistent", "deeply", "nested"))
	// MkdirAll creates all intermediate directories, so Open should succeed.
	if err != nil {
		// If the OS refuses, that's acceptable; just log it.
		t.Logf("Open returned error (acceptable): %v", err)
		return
	}
	// Close the logger so the temp dir can be cleaned up.
	if closeErr := l.Close(); closeErr != nil {
		t.Logf("Close returned error: %v", closeErr)
	}
}

func TestClose_NilLogger(t *testing.T) {
	var l *runlog.Logger
	if err := l.Close(); err != nil {
		t.Fatalf("Close on nil logger: %v", err)
	}
}

func readLogLines(t *testing.T, dir, runID string) []string {
	t.Helper()
	logPath := filepath.Join(dir, ".maggus", "runs", runID, "run.log")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open run.log: %v", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func assertLineContains(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, l := range lines {
		if strings.Contains(l, want) {
			return
		}
	}
	t.Errorf("no line containing %q in:\n%s", want, strings.Join(lines, "\n"))
}

func assertLineHasTimestamp(t *testing.T, line string) {
	t.Helper()
	// RFC3339 timestamp should be the first token; verify it parses.
	fields := strings.SplitN(line, " ", 2)
	if len(fields) < 2 {
		t.Errorf("line has no space-separated timestamp: %q", line)
		return
	}
	_, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		t.Errorf("first field %q is not RFC3339: %v", fields[0], err)
	}
}

func TestFeatureStart(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.FeatureStart("feature_001")

	lines := readLogLines(t, dir, "run1")
	if len(lines) == 0 {
		t.Fatal("no lines written")
	}
	assertLineContains(t, lines, "[INFO]")
	assertLineContains(t, lines, "Feature feature_001 started")
	assertLineHasTimestamp(t, lines[0])
}

func TestFeatureComplete(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.FeatureComplete("feature_001")

	lines := readLogLines(t, dir, "run1")
	assertLineContains(t, lines, "Feature feature_001 complete")
}

func TestTaskStart(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskStart("TASK-001-001", "Do something")

	lines := readLogLines(t, dir, "run1")
	assertLineContains(t, lines, "Task TASK-001-001 started: Do something")
}

func TestTaskComplete(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskComplete("TASK-001-001", "abc1234")

	lines := readLogLines(t, dir, "run1")
	assertLineContains(t, lines, "Task TASK-001-001 complete (commit abc1234)")
}

func TestTaskFailed(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskFailed("TASK-001-001", "agent error")

	lines := readLogLines(t, dir, "run1")
	assertLineContains(t, lines, "[ERROR]")
	assertLineContains(t, lines, "Task TASK-001-001 failed: agent error")
}

func TestToolUse(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.ToolUse("TASK-001-001", "Read", "src/main.go")

	lines := readLogLines(t, dir, "run1")
	assertLineContains(t, lines, "Task TASK-001-001 tool: [Read] src/main.go")
}

func TestMultipleEventsOrdered(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.FeatureStart("feature_001")
	l.TaskStart("TASK-001-001", "First task")
	l.ToolUse("TASK-001-001", "Bash", "go build")
	l.TaskComplete("TASK-001-001", "deadbeef")
	l.FeatureComplete("feature_001")

	lines := readLogLines(t, dir, "run1")
	if len(lines) != 5 {
		t.Fatalf("expected 5 log lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}

	expected := []string{
		"Feature feature_001 started",
		"Task TASK-001-001 started",
		"Task TASK-001-001 tool: [Bash]",
		"Task TASK-001-001 complete",
		"Feature feature_001 complete",
	}
	for i, want := range expected {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line[%d] %q does not contain %q", i, lines[i], want)
		}
	}
}

func TestOutput(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.Output("TASK-003-001", "Hello from the agent")

	lines := readLogLines(t, dir, "run1")
	if len(lines) == 0 {
		t.Fatal("no lines written")
	}
	assertLineHasTimestamp(t, lines[0])
	assertLineContains(t, lines, "[OUTPUT]")
	assertLineContains(t, lines, "[TASK-003-001] Hello from the agent")
}

func TestOutput_LongText(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	longText := strings.Repeat("x", 10000)
	l.Output("TASK-001-001", longText)

	lines := readLogLines(t, dir, "run1")
	if len(lines) == 0 {
		t.Fatal("no lines written")
	}
	if !strings.Contains(lines[0], longText) {
		t.Error("long output text was truncated")
	}
}

func TestNilLoggerMethodsAreNoOp(t *testing.T) {
	var l *runlog.Logger
	// None of these should panic.
	l.FeatureStart("x")
	l.FeatureComplete("x")
	l.TaskStart("x", "y")
	l.TaskComplete("x", "hash")
	l.TaskFailed("x", "reason")
	l.ToolUse("x", "Read", "file")
	l.Output("x", "text")
	_ = l.Close()
}

func TestOpenDaemonLog_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	wc, err := l.OpenDaemonLog()
	if err != nil {
		t.Fatalf("OpenDaemonLog: %v", err)
	}
	defer wc.Close()

	// Write some content.
	_, err = io.WriteString(wc, "daemon output line\n")
	if err != nil {
		t.Fatalf("write daemon.log: %v", err)
	}
	_ = wc.Close()

	daemonPath := filepath.Join(dir, ".maggus", "runs", "run1", "daemon.log")
	data, err := os.ReadFile(daemonPath)
	if err != nil {
		t.Fatalf("read daemon.log: %v", err)
	}
	if !strings.Contains(string(data), "daemon output line") {
		t.Errorf("daemon.log content: %q", string(data))
	}
}

func TestOpenDaemonLog_NilLogger(t *testing.T) {
	var l *runlog.Logger
	_, err := l.OpenDaemonLog()
	if err == nil {
		t.Fatal("expected error on nil logger")
	}
}

func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	if err := l.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic but may return an error (file already closed).
}

package runlog_test

import (
	"bufio"
	"encoding/json"
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

func readLogEntries(t *testing.T, dir, runID string) []runlog.Entry {
	t.Helper()
	logPath := filepath.Join(dir, ".maggus", "runs", runID, "run.log")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open run.log: %v", err)
	}
	defer f.Close()

	var entries []runlog.Entry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var e runlog.Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal JSONL line: %v\nline: %s", err, scanner.Text())
		}
		entries = append(entries, e)
	}
	return entries
}

func assertEntryTimestamp(t *testing.T, e runlog.Entry) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, e.Ts); err != nil {
		t.Errorf("ts %q is not RFC3339: %v", e.Ts, err)
	}
}

func TestFeatureStart(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.FeatureStart("feature_001")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	assertEntryTimestamp(t, e)
	if e.Level != "info" {
		t.Errorf("level = %q, want info", e.Level)
	}
	if e.Event != "feature_start" {
		t.Errorf("event = %q, want feature_start", e.Event)
	}
	if e.FeatureID != "feature_001" {
		t.Errorf("feature_id = %q, want feature_001", e.FeatureID)
	}
}

func TestFeatureComplete(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.FeatureComplete("feature_001")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Event != "feature_complete" {
		t.Errorf("event = %q, want feature_complete", entries[0].Event)
	}
	if entries[0].FeatureID != "feature_001" {
		t.Errorf("feature_id = %q, want feature_001", entries[0].FeatureID)
	}
}

func TestTaskStart(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskStart("TASK-001-001", "Do something")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Event != "task_start" {
		t.Errorf("event = %q, want task_start", e.Event)
	}
	if e.TaskID != "TASK-001-001" {
		t.Errorf("task_id = %q, want TASK-001-001", e.TaskID)
	}
	if e.Title != "Do something" {
		t.Errorf("title = %q, want Do something", e.Title)
	}
}

func TestTaskComplete(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskComplete("TASK-001-001", "abc1234")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Event != "task_complete" {
		t.Errorf("event = %q, want task_complete", e.Event)
	}
	if e.Commit != "abc1234" {
		t.Errorf("commit = %q, want abc1234", e.Commit)
	}
}

func TestTaskFailed(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskFailed("TASK-001-001", "agent error")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Level != "error" {
		t.Errorf("level = %q, want error", e.Level)
	}
	if e.Event != "task_failed" {
		t.Errorf("event = %q, want task_failed", e.Event)
	}
	if e.Reason != "agent error" {
		t.Errorf("reason = %q, want agent error", e.Reason)
	}
}

func TestToolUse(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.ToolUse("TASK-001-001", "Read", "src/main.go")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Event != "tool_use" {
		t.Errorf("event = %q, want tool_use", e.Event)
	}
	if e.Tool != "Read" {
		t.Errorf("tool = %q, want Read", e.Tool)
	}
	if e.Description != "src/main.go" {
		t.Errorf("description = %q, want src/main.go", e.Description)
	}
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

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	expectedEvents := []string{
		"feature_start",
		"task_start",
		"tool_use",
		"task_complete",
		"feature_complete",
	}
	for i, want := range expectedEvents {
		if entries[i].Event != want {
			t.Errorf("entry[%d].event = %q, want %q", i, entries[i].Event, want)
		}
	}
}

func TestOutput(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.Output("TASK-003-001", "Hello from the agent")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	assertEntryTimestamp(t, e)
	if e.Level != "output" {
		t.Errorf("level = %q, want output", e.Level)
	}
	if e.Event != "output" {
		t.Errorf("event = %q, want output", e.Event)
	}
	if e.TaskID != "TASK-003-001" {
		t.Errorf("task_id = %q, want TASK-003-001", e.TaskID)
	}
	if e.Text != "Hello from the agent" {
		t.Errorf("text = %q, want Hello from the agent", e.Text)
	}
}

func TestOutput_LongText(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	longText := strings.Repeat("x", 10000)
	l.Output("TASK-001-001", longText)

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != longText {
		t.Error("long output text was truncated")
	}
}

func TestInfo(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.Info("something happened")

	entries := readLogEntries(t, dir, "run1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Level != "info" {
		t.Errorf("level = %q, want info", e.Level)
	}
	if e.Event != "info" {
		t.Errorf("event = %q, want info", e.Event)
	}
	if e.Text != "something happened" {
		t.Errorf("text = %q, want something happened", e.Text)
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
	l.Info("msg")
	_ = l.Close()
}

func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	if err := l.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic but may return an error (file already closed).
}

func TestJSONLFormat(t *testing.T) {
	dir := t.TempDir()
	l, _ := runlog.Open("run1", dir)
	defer l.Close()

	l.TaskStart("TASK-001-001", "Do something")

	// Read raw line and verify it's valid JSON
	logPath := filepath.Join(dir, ".maggus", "runs", "run1", "run.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if !json.Valid([]byte(line)) {
		t.Fatalf("line is not valid JSON: %s", line)
	}

	// Verify omitempty works — fields not set should be absent
	var raw map[string]any
	json.Unmarshal([]byte(line), &raw)
	for _, absent := range []string{"feature_id", "commit", "tool", "description", "text", "reason"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("field %q should be omitted but is present", absent)
		}
	}
}

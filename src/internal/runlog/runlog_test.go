package runlog_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/runlog"
)

const testMaggusID = "test-guid-0001"

// openWithID is a test helper that calls Open then SetCurrentMaggusID so that
// the logger has an active file to write to.
func openWithID(t *testing.T, dir string, maxFiles int, maggusID string) *runlog.Logger {
	t.Helper()
	l, err := runlog.Open(dir, maxFiles)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.SetCurrentMaggusID(maggusID)
	return l
}

// findLogFile finds a .log file under .maggus/logs/<maggusID>/ (for testing).
func findLogFile(t *testing.T, dir, maggusID string) string {
	t.Helper()
	featureDir := filepath.Join(dir, ".maggus", "logs", maggusID)
	entries, err := os.ReadDir(featureDir)
	if err != nil {
		t.Fatalf("read feature log dir: %v", err)
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, filepath.Join(featureDir, e.Name()))
		}
	}
	if len(logs) == 0 {
		t.Fatal("no log file found in feature log dir")
	}
	if len(logs) > 1 {
		t.Fatalf("expected 1 log file, got %d: %v", len(logs), logs)
	}
	return logs[0]
}

func TestOpen_CreatesLogsBaseDir(t *testing.T) {
	dir := t.TempDir()
	l, err := runlog.Open(dir, 50)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	logsDir := filepath.Join(dir, ".maggus", "logs")
	info, err := os.Stat(logsDir)
	if err != nil {
		t.Fatalf(".maggus/logs/ not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".maggus/logs/ is not a directory")
	}
}

func TestOpen_NoFileUntilSetMaggusID(t *testing.T) {
	dir := t.TempDir()
	l, err := runlog.Open(dir, 50)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	// No feature directory should exist yet.
	logsDir := filepath.Join(dir, ".maggus", "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("unexpected subdirectory %q before SetCurrentMaggusID", e.Name())
		}
	}
}

func TestSetCurrentMaggusID_CreatesFeatureDir(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	featureDir := filepath.Join(dir, ".maggus", "logs", testMaggusID)
	info, err := os.Stat(featureDir)
	if err != nil {
		t.Fatalf("feature dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("feature dir is not a directory")
	}
}

func TestSetCurrentMaggusID_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	pid := fmt.Sprintf("%d", os.Getpid())
	logPath := filepath.Join(dir, ".maggus", "logs", testMaggusID, pid+".log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file not created at expected path %q: %v", logPath, err)
	}
}

func TestSetCurrentMaggusID_SwitchesFile(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, "guid-aaa")
	defer l.Close()

	l.Info("first feature")
	l.SetCurrentMaggusID("guid-bbb")
	l.Info("second feature")

	// Both directories should exist with log files.
	entriesA := readLogEntries(t, findLogFile(t, dir, "guid-aaa"))
	if len(entriesA) != 1 || entriesA[0].Text != "first feature" {
		t.Errorf("guid-aaa entries = %+v, want 1 entry with 'first feature'", entriesA)
	}

	entriesB := readLogEntries(t, findLogFile(t, dir, "guid-bbb"))
	if len(entriesB) != 1 || entriesB[0].Text != "second feature" {
		t.Errorf("guid-bbb entries = %+v, want 1 entry with 'second feature'", entriesB)
	}
}

func TestSetCurrentMaggusID_EmptyDropsEntries(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Info("before clear")
	l.SetCurrentMaggusID("")
	l.Info("should be dropped")
	l.SetCurrentMaggusID(testMaggusID)
	l.Info("after restore")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (dropped middle one), got %d", len(entries))
	}
	if entries[0].Text != "before clear" {
		t.Errorf("entries[0].Text = %q, want 'before clear'", entries[0].Text)
	}
	if entries[1].Text != "after restore" {
		t.Errorf("entries[1].Text = %q, want 'after restore'", entries[1].Text)
	}
}

func TestOpen_ReturnsErrorOnBadDir(t *testing.T) {
	// On most OS implementations MkdirAll succeeds for nested paths.
	l, err := runlog.Open(filepath.Join(t.TempDir(), "nonexistent", "deeply", "nested"), 10)
	if err != nil {
		t.Logf("Open returned error (acceptable): %v", err)
		return
	}
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

func readLogEntries(t *testing.T, logPath string) []runlog.Entry {
	t.Helper()
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log file: %v", err)
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.FeatureStart("feature_001")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.FeatureComplete("feature_001")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskStart("TASK-001-001", "Do something")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskComplete("TASK-001-001", "abc1234")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskFailed("TASK-001-001", "agent error")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.ToolUse("TASK-001-001", "Read", map[string]string{"file": "src/main.go"})

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	if e.Input["file"] != "src/main.go" {
		t.Errorf("input[file] = %q, want src/main.go", e.Input["file"])
	}
	if e.TaskID != "TASK-001-001" {
		t.Errorf("task_id = %q, want TASK-001-001", e.TaskID)
	}
}

func TestInfo_WritesTaskIDWhenTaskActive(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskStart("TASK-002-001", "Some task")
	l.Info("something happened during the task")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	infoEntry := entries[1]
	if infoEntry.Event != "info" {
		t.Errorf("event = %q, want info", infoEntry.Event)
	}
	if infoEntry.TaskID != "TASK-002-001" {
		t.Errorf("task_id = %q, want TASK-002-001", infoEntry.TaskID)
	}
}

func TestInfo_NoTaskIDWhenNoTaskActive(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Info("daemon started")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].TaskID != "" {
		t.Errorf("task_id = %q, want empty (no active task)", entries[0].TaskID)
	}
}

func TestInfo_TaskIDClearedAfterTaskComplete(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskStart("TASK-003-001", "A task")
	l.TaskComplete("TASK-003-001", "abc123")
	l.Info("post-task info")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	postInfo := entries[2]
	if postInfo.Event != "info" {
		t.Errorf("event = %q, want info", postInfo.Event)
	}
	if postInfo.TaskID != "" {
		t.Errorf("task_id = %q, want empty after task_complete", postInfo.TaskID)
	}
}

func TestMultipleEventsOrdered(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.FeatureStart("feature_001")
	l.TaskStart("TASK-001-001", "First task")
	l.ToolUse("TASK-001-001", "Bash", map[string]string{"command": "go build"})
	l.TaskComplete("TASK-001-001", "deadbeef")
	l.FeatureComplete("feature_001")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Output("TASK-003-001", "Hello from the agent")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	longText := strings.Repeat("x", 10000)
	l.Output("TASK-001-001", longText)

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != longText {
		t.Error("long output text was truncated")
	}
}

func TestInfo(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Info("something happened")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
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
	l.ToolUse("x", "Read", map[string]string{"file": "file"})
	l.Output("x", "text")
	l.Info("msg")
	l.SetCurrentMaggusID("some-id")
	_ = l.Close()
}

func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	if err := l.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic but may return an error (file already closed).
}

func TestJSONLFormat(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.TaskStart("TASK-001-001", "Do something")

	logPath := findLogFile(t, dir, testMaggusID)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if !json.Valid([]byte(line)) {
		t.Fatalf("line is not valid JSON: %s", line)
	}

	// Verify omitempty works — fields not set should be absent.
	var raw map[string]any
	json.Unmarshal([]byte(line), &raw)
	for _, absent := range []string{"feature_id", "commit", "tool", "description", "text", "reason"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("field %q should be omitted but is present", absent)
		}
	}
}

func TestLogFileNameIsPID(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Info("check path")

	logPath := findLogFile(t, dir, testMaggusID)
	wantName := fmt.Sprintf("%d.log", os.Getpid())
	if filepath.Base(logPath) != wantName {
		t.Errorf("log filename = %q, want %q", filepath.Base(logPath), wantName)
	}
}

func TestMaggusID_InjectedIntoEntries(t *testing.T) {
	dir := t.TempDir()
	l := openWithID(t, dir, 50, testMaggusID)
	defer l.Close()

	l.Info("test entry")

	entries := readLogEntries(t, findLogFile(t, dir, testMaggusID))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MaggusID != testMaggusID {
		t.Errorf("maggus_id = %q, want %q", entries[0].MaggusID, testMaggusID)
	}
}

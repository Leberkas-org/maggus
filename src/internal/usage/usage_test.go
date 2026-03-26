package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// redirectHome sets HOME/USERPROFILE to a temp dir for the duration of the test
// so that Append writes to a controlled location.
func redirectHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return tmpDir
}

// usageDir returns ~/.maggus/usage/ relative to the given home dir.
func usageDir(home string) string {
	return filepath.Join(home, ".maggus", "usage")
}

// --- AppendTo tests ---

func TestAppendToCreatesFileWithJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	records := []Record{
		{
			RunID:        "run-1",
			TaskShort:    "TASK-001",
			TaskTitle:    "First task",
			Model:        "opus",
			Agent:        "claude",
			InputTokens:  100,
			OutputTokens: 200,
			StartTime:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			EndTime:      time.Date(2026, 1, 1, 10, 5, 30, 0, time.UTC),
		},
	}

	if err := AppendTo(path, records); err != nil {
		t.Fatalf("AppendTo returned error: %v", err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if rec.RunID != "run-1" {
		t.Errorf("RunID = %q, want %q", rec.RunID, "run-1")
	}
	if rec.TaskShort != "TASK-001" {
		t.Errorf("TaskShort = %q, want %q", rec.TaskShort, "TASK-001")
	}
}

func TestAppendToMultipleCallsAppendsLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	rec := Record{
		RunID:        "run-1",
		TaskShort:    "TASK-001",
		TaskTitle:    "Task",
		Model:        "sonnet",
		Agent:        "claude",
		InputTokens:  10,
		OutputTokens: 20,
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
	}

	if err := AppendTo(path, []Record{rec}); err != nil {
		t.Fatalf("first AppendTo: %v", err)
	}
	if err := AppendTo(path, []Record{rec}); err != nil {
		t.Fatalf("second AppendTo: %v", err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestAppendToEmptyRecordsIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := AppendTo(path, []Record{}); err != nil {
		t.Fatalf("AppendTo with empty records returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no file to be created for empty records")
	}
}

func TestAppendToNilRecordsIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	if err := AppendTo(path, nil); err != nil {
		t.Fatalf("AppendTo with nil records returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no file to be created for nil records")
	}
}

func TestAppendToWritesCorrectFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	start := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	end := time.Date(2026, 3, 15, 14, 35, 45, 0, time.UTC)

	records := []Record{
		{
			RunID:        "run-42",
			TaskShort:    "TASK-007",
			TaskTitle:    "Secret task",
			Model:        "claude-opus-4-6",
			Agent:        "claude",
			InputTokens:  5000,
			OutputTokens: 3000,
			StartTime:    start,
			EndTime:      end,
		},
	}

	if err := AppendTo(path, records); err != nil {
		t.Fatalf("AppendTo returned error: %v", err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if rec.RunID != "run-42" {
		t.Errorf("RunID = %q, want %q", rec.RunID, "run-42")
	}
	if rec.TaskShort != "TASK-007" {
		t.Errorf("TaskShort = %q, want %q", rec.TaskShort, "TASK-007")
	}
	if rec.TaskTitle != "Secret task" {
		t.Errorf("TaskTitle = %q, want %q", rec.TaskTitle, "Secret task")
	}
	if rec.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", rec.InputTokens)
	}
	if rec.OutputTokens != 3000 {
		t.Errorf("OutputTokens = %d, want 3000", rec.OutputTokens)
	}
}

func TestAppendToReturnsErrorForMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "deep", "test.jsonl")

	records := []Record{
		{
			RunID:     "run-1",
			TaskShort: "TASK-001",
			StartTime: time.Now(),
			EndTime:   time.Now(),
		},
	}

	err := AppendTo(path, records)
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

func TestAppendToWritesCacheAndModelUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	start := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC)

	modelUsage := map[string]agent.ModelTokens{
		"claude-opus-4-6[1m]": {
			InputTokens:              3,
			OutputTokens:             24,
			CacheCreationInputTokens: 13055,
			CacheReadInputTokens:     6692,
			CostUSD:                  0.0855,
		},
	}

	records := []Record{
		{
			RunID:                    "run-99",
			TaskShort:                "TASK-008",
			TaskTitle:                "Cache test",
			Model:                    "claude-opus-4-6",
			Agent:                    "claude",
			InputTokens:              3,
			OutputTokens:             24,
			CacheCreationInputTokens: 13055,
			CacheReadInputTokens:     6692,
			CostUSD:                  0.0855,
			ModelUsage:               modelUsage,
			StartTime:                start,
			EndTime:                  end,
		},
	}

	if err := AppendTo(path, records); err != nil {
		t.Fatalf("AppendTo returned error: %v", err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if rec.CacheCreationInputTokens != 13055 {
		t.Errorf("CacheCreationInputTokens = %d, want 13055", rec.CacheCreationInputTokens)
	}
	if rec.CacheReadInputTokens != 6692 {
		t.Errorf("CacheReadInputTokens = %d, want 6692", rec.CacheReadInputTokens)
	}
	if rec.CostUSD != 0.0855 {
		t.Errorf("CostUSD = %f, want 0.0855", rec.CostUSD)
	}

	opus, ok := rec.ModelUsage["claude-opus-4-6[1m]"]
	if !ok {
		t.Fatal("ModelUsage missing claude-opus-4-6[1m] entry")
	}
	if opus.InputTokens != 3 {
		t.Errorf("model InputTokens = %d, want 3", opus.InputTokens)
	}
	if opus.OutputTokens != 24 {
		t.Errorf("model OutputTokens = %d, want 24", opus.OutputTokens)
	}
	if opus.CacheCreationInputTokens != 13055 {
		t.Errorf("model CacheCreationInputTokens = %d, want 13055", opus.CacheCreationInputTokens)
	}
	if opus.CacheReadInputTokens != 6692 {
		t.Errorf("model CacheReadInputTokens = %d, want 6692", opus.CacheReadInputTokens)
	}
	if opus.CostUSD != 0.0855 {
		t.Errorf("model CostUSD = %f, want 0.0855", opus.CostUSD)
	}
}

func TestAppendToWritesEmptyModelUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	records := []Record{
		{
			RunID:     "run-1",
			TaskShort: "TASK-001",
			StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		},
	}

	if err := AppendTo(path, records); err != nil {
		t.Fatalf("AppendTo returned error: %v", err)
	}

	lines := readJSONL(t, path)
	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if rec.ModelUsage != nil && len(rec.ModelUsage) != 0 {
		t.Errorf("ModelUsage = %v, want nil or empty", rec.ModelUsage)
	}
}

func TestEachLineIsIndependentlyParseable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	records := []Record{
		{RunID: "run-1", TaskShort: "TASK-001", StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)},
		{RunID: "run-2", TaskShort: "TASK-002", StartTime: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 1, 2, 0, 0, time.UTC)},
		{RunID: "run-3", TaskShort: "TASK-003", StartTime: time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 2, 3, 0, 0, time.UTC)},
	}

	if err := AppendTo(path, records); err != nil {
		t.Fatalf("AppendTo returned error: %v", err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("line %d is not independently parseable: %v", i, err)
		}
	}
}

// --- Append tests ---

func TestAppendWorkRecordGoesToWorkJSONL(t *testing.T) {
	home := redirectHome(t)
	records := []Record{
		{
			RunID:        "run-1",
			TaskShort:    "TASK-001",
			InputTokens:  100,
			OutputTokens: 200,
			StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:      time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		},
	}

	if err := Append(records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	workPath := filepath.Join(usageDir(home), "work.jsonl")
	lines := readJSONL(t, workPath)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in work.jsonl, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if rec.RunID != "run-1" {
		t.Errorf("RunID = %q, want %q", rec.RunID, "run-1")
	}

	// sessions.jsonl should not exist
	sessionsPath := filepath.Join(usageDir(home), "sessions.jsonl")
	if _, err := os.Stat(sessionsPath); !os.IsNotExist(err) {
		t.Error("sessions.jsonl should not exist for work records")
	}
}

func TestAppendSessionRecordGoesToSessionsJSONL(t *testing.T) {
	home := redirectHome(t)
	records := []Record{
		{
			RunID:     "run-1",
			Kind:      "plan",
			TaskShort: "TASK-001",
			StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		},
	}

	if err := Append(records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	sessionsPath := filepath.Join(usageDir(home), "sessions.jsonl")
	lines := readJSONL(t, sessionsPath)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in sessions.jsonl, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if rec.Kind != "plan" {
		t.Errorf("Kind = %q, want %q", rec.Kind, "plan")
	}

	// work.jsonl should not exist
	workPath := filepath.Join(usageDir(home), "work.jsonl")
	if _, err := os.Stat(workPath); !os.IsNotExist(err) {
		t.Error("work.jsonl should not exist for session records")
	}
}

func TestAppendMixedRecordsRoutedCorrectly(t *testing.T) {
	home := redirectHome(t)
	records := []Record{
		{RunID: "run-work", Kind: "", StartTime: time.Now(), EndTime: time.Now()},
		{RunID: "run-session", Kind: "bugreport", StartTime: time.Now(), EndTime: time.Now()},
	}

	if err := Append(records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	workLines := readJSONL(t, filepath.Join(usageDir(home), "work.jsonl"))
	if len(workLines) != 1 {
		t.Fatalf("expected 1 work record, got %d", len(workLines))
	}

	sessionLines := readJSONL(t, filepath.Join(usageDir(home), "sessions.jsonl"))
	if len(sessionLines) != 1 {
		t.Fatalf("expected 1 session record, got %d", len(sessionLines))
	}
}

func TestAppendCreatesUsageDirectory(t *testing.T) {
	home := redirectHome(t)
	dir := usageDir(home)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("usage dir should not exist yet")
	}

	records := []Record{
		{RunID: "run-1", StartTime: time.Now(), EndTime: time.Now()},
	}
	if err := Append(records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("usage dir was not created: %v", err)
	}
}

func TestAppendEmptyRecordsIsNoOp(t *testing.T) {
	home := redirectHome(t)

	if err := Append([]Record{}); err != nil {
		t.Fatalf("Append with empty records returned error: %v", err)
	}

	// No files should be created.
	if _, err := os.Stat(usageDir(home)); !os.IsNotExist(err) {
		t.Error("expected no directory to be created for empty records")
	}
}

func TestAppendNilRecordsIsNoOp(t *testing.T) {
	home := redirectHome(t)

	if err := Append(nil); err != nil {
		t.Fatalf("Append with nil records returned error: %v", err)
	}

	if _, err := os.Stat(usageDir(home)); !os.IsNotExist(err) {
		t.Error("expected no directory to be created for nil records")
	}
}

func TestAppendRecordHasRepositoryAndKindFields(t *testing.T) {
	home := redirectHome(t)
	records := []Record{
		{
			RunID:      "run-1",
			Repository: "https://github.com/leberkas-org/maggus",
			Kind:       "prompt",
			ItemID:     "abc-123",
			ItemShort:  "feature_001",
			ItemTitle:  "Global Usage Tracking",
			TaskShort:  "TASK-001-002",
			StartTime:  time.Now(),
			EndTime:    time.Now(),
		},
	}

	if err := Append(records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(usageDir(home), "sessions.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if rec.Repository != "https://github.com/leberkas-org/maggus" {
		t.Errorf("Repository = %q, want %q", rec.Repository, "https://github.com/leberkas-org/maggus")
	}
	if rec.Kind != "prompt" {
		t.Errorf("Kind = %q, want %q", rec.Kind, "prompt")
	}
	if rec.ItemID != "abc-123" {
		t.Errorf("ItemID = %q, want %q", rec.ItemID, "abc-123")
	}
}

// readJSONL reads all non-empty lines from a file.
func readJSONL(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open JSONL: %v", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read JSONL: %v", err)
	}
	return lines
}

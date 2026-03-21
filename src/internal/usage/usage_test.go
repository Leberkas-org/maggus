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

func TestAppendCreatesFileWithJSONL(t *testing.T) {
	dir := setupDir(t)
	records := []Record{
		{
			RunID:        "run-1",
			TaskID:       "TASK-001",
			TaskTitle:    "First task",
			FeatureFile:  "plan_1.md",
			Model:        "opus",
			Agent:        "claude",
			InputTokens:  100,
			OutputTokens: 200,
			StartTime:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			EndTime:      time.Date(2026, 1, 1, 10, 5, 30, 0, time.UTC),
		},
	}

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
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
	if rec.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", rec.TaskID, "TASK-001")
	}
}

func TestAppendMultipleCallsAppendsLines(t *testing.T) {
	dir := setupDir(t)
	rec := Record{
		RunID:        "run-1",
		TaskID:       "TASK-001",
		TaskTitle:    "Task",
		FeatureFile:  "plan.md",
		Model:        "sonnet",
		Agent:        "claude",
		InputTokens:  10,
		OutputTokens: 20,
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
	}

	if err := Append(dir, []Record{rec}); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := Append(dir, []Record{rec}); err != nil {
		t.Fatalf("second Append: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Each line must be valid JSON.
	for i, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestAppendEmptyRecordsIsNoOp(t *testing.T) {
	dir := t.TempDir()

	if err := Append(dir, []Record{}); err != nil {
		t.Fatalf("Append with empty records returned error: %v", err)
	}

	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no file to be created for empty records")
	}
}

func TestAppendNilRecordsIsNoOp(t *testing.T) {
	dir := t.TempDir()

	if err := Append(dir, nil); err != nil {
		t.Fatalf("Append with nil records returned error: %v", err)
	}

	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no file to be created for nil records")
	}
}

func TestAppendWritesCorrectFields(t *testing.T) {
	dir := setupDir(t)
	start := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	end := time.Date(2026, 3, 15, 14, 35, 45, 0, time.UTC)

	records := []Record{
		{
			RunID:        "run-42",
			TaskID:       "TASK-007",
			TaskTitle:    "Secret task",
			FeatureFile:  "plan_3.md",
			Model:        "claude-opus-4-6",
			Agent:        "claude",
			InputTokens:  5000,
			OutputTokens: 3000,
			StartTime:    start,
			EndTime:      end,
		},
	}

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
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
	if rec.TaskID != "TASK-007" {
		t.Errorf("TaskID = %q, want %q", rec.TaskID, "TASK-007")
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
	if rec.Elapsed != "5m45s" {
		t.Errorf("Elapsed = %q, want %q", rec.Elapsed, "5m45s")
	}
}

func TestElapsedTimeTruncatedToSeconds(t *testing.T) {
	dir := setupDir(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 2, 30, 999_000_000, time.UTC)

	records := []Record{
		{
			RunID:     "run-1",
			TaskID:    "TASK-001",
			StartTime: start,
			EndTime:   end,
		},
	}

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if rec.Elapsed != "2m30s" {
		t.Errorf("Elapsed = %q, want %q", rec.Elapsed, "2m30s")
	}
}

func TestAppendReturnsErrorForMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent", "deep")

	records := []Record{
		{
			RunID:     "run-1",
			TaskID:    "TASK-001",
			StartTime: time.Now(),
			EndTime:   time.Now(),
		},
	}

	err := Append(dir, records)
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

func TestAppendWritesCacheAndModelUsage(t *testing.T) {
	dir := setupDir(t)
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
			TaskID:                   "TASK-008",
			TaskTitle:                "Cache test",
			FeatureFile:              "plan_2.md",
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

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
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

func TestAppendWritesEmptyModelUsage(t *testing.T) {
	dir := setupDir(t)
	records := []Record{
		{
			RunID:     "run-1",
			TaskID:    "TASK-001",
			StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		},
	}

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if rec.ModelUsage != nil && len(rec.ModelUsage) != 0 {
		t.Errorf("ModelUsage = %v, want nil or empty", rec.ModelUsage)
	}
}

func TestEachLineIsIndependentlyParseable(t *testing.T) {
	dir := setupDir(t)
	records := []Record{
		{RunID: "run-1", TaskID: "TASK-001", StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)},
		{RunID: "run-2", TaskID: "TASK-002", StartTime: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 1, 2, 0, 0, time.UTC)},
		{RunID: "run-3", TaskID: "TASK-003", StartTime: time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 1, 2, 3, 0, 0, time.UTC)},
	}

	if err := Append(dir, records); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	lines := readJSONL(t, filepath.Join(dir, fileName))
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

// setupDir creates a temp dir with the .maggus subdirectory that Append expects.
func setupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0755); err != nil {
		t.Fatalf("create .maggus dir: %v", err)
	}
	return dir
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

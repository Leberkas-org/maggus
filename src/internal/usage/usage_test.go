package usage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendCreatesFileWithHeader(t *testing.T) {
	dir := setupDir(t)
	records := []Record{
		{
			RunID:        "run-1",
			TaskID:       "TASK-001",
			TaskTitle:    "First task",
			PlanFile:     "plan_1.md",
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

	rows := readCSV(t, filepath.Join(dir, fileName))
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d", len(rows))
	}

	// First row should be the header.
	wantHeader := header()
	for i, col := range wantHeader {
		if rows[0][i] != col {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], col)
		}
	}
}

func TestAppendDoesNotDuplicateHeader(t *testing.T) {
	dir := setupDir(t)
	rec := Record{
		RunID:        "run-1",
		TaskID:       "TASK-001",
		TaskTitle:    "Task",
		PlanFile:     "plan.md",
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

	rows := readCSV(t, filepath.Join(dir, fileName))
	// Expect header + 2 data rows = 3 rows total.
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(rows))
	}

	// Only the first row should be the header.
	if rows[1][0] == "run_id" {
		t.Error("second row is a duplicate header")
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

func TestAppendWritesCorrectColumns(t *testing.T) {
	dir := setupDir(t)
	start := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	end := time.Date(2026, 3, 15, 14, 35, 45, 0, time.UTC)

	records := []Record{
		{
			RunID:        "run-42",
			TaskID:       "TASK-007",
			TaskTitle:    "Secret task",
			PlanFile:     "plan_3.md",
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

	rows := readCSV(t, filepath.Join(dir, fileName))
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	data := rows[1]
	wantColumns := []string{
		"run-42",
		"TASK-007",
		"Secret task",
		"plan_3.md",
		"claude-opus-4-6",
		"claude",
		"5000",
		"3000",
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		"5m45s",
	}

	if len(data) != len(wantColumns) {
		t.Fatalf("got %d columns, want %d", len(data), len(wantColumns))
	}
	for i, want := range wantColumns {
		if data[i] != want {
			t.Errorf("column %d (%s) = %q, want %q", i, header()[i], data[i], want)
		}
	}
}

func TestElapsedTimeTruncatedToSeconds(t *testing.T) {
	dir := setupDir(t)
	// StartTime and EndTime differ by 2m30.999s — should truncate to 2m30s.
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

	rows := readCSV(t, filepath.Join(dir, fileName))
	elapsed := rows[1][10] // elapsed is the last column
	want := "2m30s"
	if elapsed != want {
		t.Errorf("elapsed = %q, want %q", elapsed, want)
	}
}

func TestAppendReturnsErrorForMissingDirectory(t *testing.T) {
	// Use a directory path that does not exist.
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

// setupDir creates a temp dir with the .maggus subdirectory that Append expects.
func setupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0755); err != nil {
		t.Fatalf("create .maggus dir: %v", err)
	}
	return dir
}

// readCSV is a test helper that reads all rows from a CSV file.
func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	return rows
}

package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeLegacyLine returns a JSON line in the old per-project usage format.
func makeLegacyLine(runID, taskID, taskTitle, kind string) string {
	type legacy struct {
		RunID        string    `json:"run_id"`
		TaskID       string    `json:"task_id"`
		TaskTitle    string    `json:"task_title"`
		FeatureFile  string    `json:"feature_file,omitempty"`
		Model        string    `json:"model"`
		Agent        string    `json:"agent"`
		InputTokens  int       `json:"input_tokens"`
		OutputTokens int       `json:"output_tokens"`
		CostUSD      float64   `json:"cost_usd"`
		StartTime    time.Time `json:"start_time"`
		EndTime      time.Time `json:"end_time"`
	}
	_ = kind // kind is not stored in legacy records; it's derived from filename
	b, _ := json.Marshal(legacy{
		RunID:        runID,
		TaskID:       taskID,
		TaskTitle:    taskTitle,
		FeatureFile:  ".maggus/features/feature_001.md",
		Model:        "claude-opus-4-6",
		Agent:        "claude",
		InputTokens:  100,
		OutputTokens: 200,
		CostUSD:      0.05,
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
	})
	return string(b)
}

// writeLines writes lines to path, one per line.
func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("write lines: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatalf("write line: %v", err)
		}
	}
}

// setupProject creates a .maggus directory under a temp dir and returns the project dir.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatalf("create .maggus: %v", err)
	}
	return dir
}

// TestMigrateProject_WorkFile verifies that usage_work.jsonl is migrated to work.jsonl.
func TestMigrateProject_WorkFile(t *testing.T) {
	home := redirectHome(t)
	projectDir := setupProject(t)

	maggusDir := filepath.Join(projectDir, ".maggus")
	writeLines(t, filepath.Join(maggusDir, "usage_work.jsonl"), []string{
		makeLegacyLine("run-1", "TASK-001", "First task", ""),
		makeLegacyLine("run-2", "TASK-002", "Second task", ""),
	})

	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}

	// Records should appear in global work.jsonl.
	lines := readJSONL(t, filepath.Join(usageDir(home), "work.jsonl"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in work.jsonl, got %d", len(lines))
	}

	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("parse record: %v", err)
	}
	if rec.TaskShort != "TASK-001" {
		t.Errorf("TaskShort = %q, want TASK-001", rec.TaskShort)
	}
	if rec.Kind != "" {
		t.Errorf("Kind = %q, want empty for work records", rec.Kind)
	}

	// Source file should be renamed.
	if _, err := os.Stat(filepath.Join(maggusDir, "usage_work.jsonl")); !os.IsNotExist(err) {
		t.Error("usage_work.jsonl should have been renamed")
	}
	if _, err := os.Stat(filepath.Join(maggusDir, "usage_work.jsonl.migrated")); err != nil {
		t.Error("usage_work.jsonl.migrated should exist")
	}
}

// TestMigrateProject_SessionFiles verifies that session files are migrated to sessions.jsonl
// with the correct Kind derived from the filename.
func TestMigrateProject_SessionFiles(t *testing.T) {
	tests := []struct {
		filename string
		wantKind string
	}{
		{"usage_plan.jsonl", "plan"},
		{"usage_prompt.jsonl", "prompt"},
		{"usage_bugreport.jsonl", "bugreport"},
		{"usage_vision.jsonl", "vision"},
		{"usage_architecture.jsonl", "architecture"},
		{"usage_bryan_plan.jsonl", "bryan_plan"},
		{"usage_bryan_bugreport.jsonl", "bryan_bugreport"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			home := redirectHome(t)
			projectDir := setupProject(t)
			maggusDir := filepath.Join(projectDir, ".maggus")

			writeLines(t, filepath.Join(maggusDir, tc.filename), []string{
				makeLegacyLine("run-1", "", "", tc.wantKind),
			})

			if err := MigrateProject(projectDir); err != nil {
				t.Fatalf("MigrateProject: %v", err)
			}

			lines := readJSONL(t, filepath.Join(usageDir(home), "sessions.jsonl"))
			if len(lines) != 1 {
				t.Fatalf("expected 1 line in sessions.jsonl, got %d", len(lines))
			}

			var rec Record
			if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
				t.Fatalf("parse record: %v", err)
			}
			if rec.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", rec.Kind, tc.wantKind)
			}

			// Source file should be renamed.
			if _, err := os.Stat(filepath.Join(maggusDir, tc.filename)); !os.IsNotExist(err) {
				t.Errorf("%s should have been renamed", tc.filename)
			}
		})
	}
}

// TestMigrateProject_SkipsMissingFiles verifies that missing source files don't cause errors.
func TestMigrateProject_SkipsMissingFiles(t *testing.T) {
	redirectHome(t)
	projectDir := setupProject(t)

	// No files exist — should succeed silently.
	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("MigrateProject with no files: %v", err)
	}
}

// TestMigrateProject_Idempotent verifies that running migration twice is safe.
func TestMigrateProject_Idempotent(t *testing.T) {
	home := redirectHome(t)
	projectDir := setupProject(t)
	maggusDir := filepath.Join(projectDir, ".maggus")

	writeLines(t, filepath.Join(maggusDir, "usage_work.jsonl"), []string{
		makeLegacyLine("run-1", "TASK-001", "Task", ""),
	})

	// First migration.
	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("first MigrateProject: %v", err)
	}

	// Second migration should be a no-op (file no longer exists).
	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("second MigrateProject: %v", err)
	}

	// Still only one record.
	lines := readJSONL(t, filepath.Join(usageDir(home), "work.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after idempotent migration, got %d", len(lines))
	}
}

// TestMigrateProject_TaskIDMapsToTaskShort verifies field mapping from old to new format.
func TestMigrateProject_TaskIDMapsToTaskShort(t *testing.T) {
	home := redirectHome(t)
	projectDir := setupProject(t)
	maggusDir := filepath.Join(projectDir, ".maggus")

	writeLines(t, filepath.Join(maggusDir, "usage_work.jsonl"), []string{
		makeLegacyLine("run-42", "TASK-007", "The title", ""),
	})

	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}

	lines := readJSONL(t, filepath.Join(usageDir(home), "work.jsonl"))
	var rec Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.TaskShort != "TASK-007" {
		t.Errorf("TaskShort = %q, want TASK-007", rec.TaskShort)
	}
	if rec.TaskTitle != "The title" {
		t.Errorf("TaskTitle = %q, want 'The title'", rec.TaskTitle)
	}
	if rec.RunID != "run-42" {
		t.Errorf("RunID = %q, want run-42", rec.RunID)
	}
}

// TestMigrateProject_LegacyCSVFilesRenamed verifies that CSV files are renamed without migration.
func TestMigrateProject_LegacyCSVFilesRenamed(t *testing.T) {
	redirectHome(t)
	projectDir := setupProject(t)
	maggusDir := filepath.Join(projectDir, ".maggus")

	for _, name := range []string{"usage.csv", "usage_v3.csv"} {
		if err := os.WriteFile(filepath.Join(maggusDir, name), []byte("old,csv,data\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}

	for _, name := range []string{"usage.csv", "usage_v3.csv"} {
		if _, err := os.Stat(filepath.Join(maggusDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should have been renamed", name)
		}
		if _, err := os.Stat(filepath.Join(maggusDir, name+".migrated")); err != nil {
			t.Errorf("%s.migrated should exist", name)
		}
	}
}

// TestMigrateProject_EmptyFileNoGlobalDirCreated verifies that an empty source file
// does not create the global usage directory.
func TestMigrateProject_EmptyFileNoGlobalDirCreated(t *testing.T) {
	home := redirectHome(t)
	projectDir := setupProject(t)
	maggusDir := filepath.Join(projectDir, ".maggus")

	// Write an empty file.
	if err := os.WriteFile(filepath.Join(maggusDir, "usage_work.jsonl"), []byte(""), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := MigrateProject(projectDir); err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}

	// Global usage dir should NOT have been created (no records to write).
	if _, err := os.Stat(usageDir(home)); !os.IsNotExist(err) {
		t.Error("global usage dir should not be created for empty source file")
	}

	// Source file should still be renamed.
	if _, err := os.Stat(filepath.Join(maggusDir, "usage_work.jsonl.migrated")); err != nil {
		t.Error("usage_work.jsonl.migrated should exist even for empty source")
	}
}

package hooks_test

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/hooks"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/stores"
)

func skipIfNoShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(`C:\Windows\System32\cmd.exe`); err != nil {
			t.Skip("shell execution unavailable: cmd.exe not found")
		}
	} else {
		if _, err := os.Stat("/bin/sh"); err != nil {
			t.Skip("shell execution unavailable: /bin/sh not found")
		}
	}
}

// writeScript creates a platform-appropriate script that captures stdin JSON to outFile.
func writeScript(t *testing.T, dir, name, outFile string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		batPath := filepath.Join(dir, name+".bat")
		content := "@echo off\r\nfindstr \"^\" > \"" + outFile + "\"\r\n"
		if err := os.WriteFile(batPath, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return batPath
	}
	shPath := filepath.Join(dir, name+".sh")
	content := "#!/bin/sh\ncat > '" + outFile + "'\n"
	if err := os.WriteFile(shPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return shPath
}

// writeFailScript creates a script that always exits with code 1.
func writeFailScript(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		batPath := filepath.Join(dir, name+".bat")
		content := "@echo off\r\nexit /b 1\r\n"
		if err := os.WriteFile(batPath, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return batPath
	}
	shPath := filepath.Join(dir, name+".sh")
	content := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(shPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return shPath
}

// setupProject creates a temp dir with .maggus/features/ and .maggus/bugs/ subdirs.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{".maggus/features", ".maggus/bugs"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeFeatureFile creates a feature file with all tasks completed.
func writeFeatureFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".maggus", "features", filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeBugFile creates a bug file with all tasks completed.
func writeBugFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".maggus", "bugs", filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestLogger() (*log.Logger, *strings.Builder) {
	var buf strings.Builder
	return log.New(&buf, "", 0), &buf
}

// assertHookPayload reads the output file and validates the JSON payload.
func assertHookPayload(t *testing.T, outFile string, wantEvent, wantFile, wantTitle string, wantTaskCount int) map[string]any {
	t.Helper()
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read hook output file %s: %v", outFile, err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("hook output is not valid JSON: %v\nraw: %s", err, string(data))
	}

	if got, _ := m["event"].(string); got != wantEvent {
		t.Errorf("event = %q, want %q", got, wantEvent)
	}
	if got, _ := m["file"].(string); got != wantFile {
		t.Errorf("file = %q, want %q", got, wantFile)
	}
	if got, _ := m["title"].(string); got != wantTitle {
		t.Errorf("title = %q, want %q", got, wantTitle)
	}
	if _, ok := m["timestamp"].(string); !ok {
		t.Error("timestamp field missing or not a string")
	} else {
		// Validate it's valid RFC3339
		ts := m["timestamp"].(string)
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Errorf("timestamp %q is not valid RFC3339: %v", ts, err)
		}
	}
	tasks, ok := m["tasks"].([]any)
	if !ok {
		t.Fatalf("tasks field missing or not an array, got %T", m["tasks"])
	}
	if len(tasks) != wantTaskCount {
		t.Errorf("tasks count = %d, want %d", len(tasks), wantTaskCount)
	}

	return m
}

// TestIntegration_FeatureCompleteHook verifies the full flow:
// config → MarkCompletedFeatures → hooks.Run → JSON payload.
func TestIntegration_FeatureCompleteHook(t *testing.T) {
	skipIfNoShell(t)

	dir := setupProject(t)
	outFile := filepath.Join(dir, "feature_hook_out.json")
	scriptPath := writeScript(t, dir, "capture_feature", outFile)

	// Write config with hook
	configContent := fmt.Sprintf("hooks:\n  on_feature_complete:\n    - run: \"%s\"\n", filepath.ToSlash(scriptPath))
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "config.yml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a fully completed feature file
	featureContent := `<!-- maggus-id: f47ac10b-58cc-4372-a567-0e02b2c3d479 -->
# Feature 099: Test Hooks Feature

### TASK-099-001: First task
- [x] Criterion A
- [x] Criterion B

### TASK-099-002: Second task
- [x] Criterion C
`
	featurePath := writeFeatureFile(t, dir, "feature_099.md", featureContent)

	// Load config
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Snapshot file metadata before marking complete (mirrors work_task.go flow)
	maggusID := parser.ParseMaggusID(featurePath)
	title := parser.ParseFileTitle(featurePath)
	tasks, err := parser.ParseFile(featurePath)
	if err != nil {
		t.Fatalf("failed to parse feature file: %v", err)
	}

	var taskInfos []hooks.TaskInfo
	for _, task := range tasks {
		taskInfos = append(taskInfos, hooks.TaskInfo{ID: task.ID, Title: task.Title})
	}

	// Mark completed (renames the file)
	completed, err := stores.NewFileFeatureStore(dir).MarkCompleted("rename")
	if err != nil {
		t.Fatalf("MarkCompletedFeatures failed: %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed feature, got %d", len(completed))
	}

	// Fire hooks (as work_task.go does)
	event := hooks.Event{
		Type:      "feature_complete",
		File:      filepath.Base(completed[0]),
		MaggusID:  maggusID,
		Title:     title,
		Action:    "rename",
		Tasks:     taskInfos,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	logger, _ := newTestLogger()
	hooks.Run(cfg.Hooks.OnFeatureComplete, event, dir, logger)

	// Assert payload
	m := assertHookPayload(t, outFile, "feature_complete", "feature_099.md", "Test Hooks Feature", 2)

	if got, _ := m["maggus_id"].(string); got != "f47ac10b-58cc-4372-a567-0e02b2c3d479" {
		t.Errorf("maggus_id = %q, want %q", got, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	}
	if got, _ := m["action"].(string); got != "rename" {
		t.Errorf("action = %q, want %q", got, "rename")
	}
}

// TestIntegration_BugCompleteHook verifies the flow for bug completion.
func TestIntegration_BugCompleteHook(t *testing.T) {
	skipIfNoShell(t)

	dir := setupProject(t)
	outFile := filepath.Join(dir, "bug_hook_out.json")
	scriptPath := writeScript(t, dir, "capture_bug", outFile)

	configContent := fmt.Sprintf("hooks:\n  on_bug_complete:\n    - run: \"%s\"\n", filepath.ToSlash(scriptPath))
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "config.yml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	bugContent := `<!-- maggus-id: a1b2c3d4-e5f6-7890-abcd-ef1234567890 -->
# Bug 042: Login Crash

### BUG-042-001: Fix null pointer
- [x] Handle nil user gracefully
`
	bugPath := writeBugFile(t, dir, "bug_042.md", bugContent)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Snapshot before marking
	maggusID := parser.ParseMaggusID(bugPath)
	title := parser.ParseFileTitle(bugPath)
	tasks, err := parser.ParseFile(bugPath)
	if err != nil {
		t.Fatalf("failed to parse bug file: %v", err)
	}

	var taskInfos []hooks.TaskInfo
	for _, task := range tasks {
		taskInfos = append(taskInfos, hooks.TaskInfo{ID: task.ID, Title: task.Title})
	}

	completed, err := parser.MarkCompletedBugs(dir, "rename")
	if err != nil {
		t.Fatalf("MarkCompletedBugs failed: %v", err)
	}
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed bug, got %d", len(completed))
	}

	event := hooks.Event{
		Type:      "bug_complete",
		File:      filepath.Base(completed[0]),
		MaggusID:  maggusID,
		Title:     title,
		Action:    "rename",
		Tasks:     taskInfos,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	logger, _ := newTestLogger()
	hooks.Run(cfg.Hooks.OnBugComplete, event, dir, logger)

	m := assertHookPayload(t, outFile, "bug_complete", "bug_042.md", "Login Crash", 1)

	if got, _ := m["maggus_id"].(string); got != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("maggus_id = %q, want %q", got, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}
}

// TestIntegration_TaskCompleteHook verifies task-level completion hooks.
func TestIntegration_TaskCompleteHook(t *testing.T) {
	skipIfNoShell(t)

	dir := setupProject(t)
	outFile := filepath.Join(dir, "task_hook_out.json")
	scriptPath := writeScript(t, dir, "capture_task", outFile)

	configContent := fmt.Sprintf("hooks:\n  on_task_complete:\n    - run: \"%s\"\n", filepath.ToSlash(scriptPath))
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "config.yml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Simulate task completion (as work_task.go does after a commit)
	event := hooks.Event{
		Type:      "task_complete",
		File:      "feature_099.md",
		MaggusID:  "some-uuid-here",
		Title:     "Implement widget",
		Action:    "",
		Tasks:     []hooks.TaskInfo{{ID: "TASK-099-001", Title: "Implement widget"}},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	logger, _ := newTestLogger()
	hooks.Run(cfg.Hooks.OnTaskComplete, event, dir, logger)

	m := assertHookPayload(t, outFile, "task_complete", "feature_099.md", "Implement widget", 1)

	// task_complete action should be empty
	if got, _ := m["action"].(string); got != "" {
		t.Errorf("action = %q, want empty string for task_complete", got)
	}
}

// TestIntegration_FailingHookDoesNotBlockSubsequent verifies that a failing hook
// does not prevent later hooks from running.
func TestIntegration_FailingHookDoesNotBlockSubsequent(t *testing.T) {
	skipIfNoShell(t)

	dir := setupProject(t)
	outFile := filepath.Join(dir, "after_fail.json")
	failScriptPath := writeFailScript(t, dir, "fail_hook")
	captureScriptPath := writeScript(t, dir, "capture_after_fail", outFile)

	configContent := fmt.Sprintf(
		"hooks:\n  on_feature_complete:\n    - run: \"%s\"\n    - run: \"%s\"\n",
		filepath.ToSlash(failScriptPath),
		filepath.ToSlash(captureScriptPath),
	)
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "config.yml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	event := hooks.Event{
		Type:      "feature_complete",
		File:      "feature_001.md",
		Title:     "Some Feature",
		Tasks:     []hooks.TaskInfo{{ID: "TASK-001-001", Title: "A task"}},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	logger, logBuf := newTestLogger()
	hooks.Run(cfg.Hooks.OnFeatureComplete, event, dir, logger)

	// First hook should have failed and logged a warning
	if !strings.Contains(logBuf.String(), "WARNING") {
		t.Errorf("expected warning from failing hook, got: %s", logBuf.String())
	}

	// Second hook should still have run and produced output
	assertHookPayload(t, outFile, "feature_complete", "feature_001.md", "Some Feature", 1)
}

// TestIntegration_ConfigLoadsHooks verifies that config.Load correctly parses hook entries
// and they can be used to execute hooks end-to-end.
func TestIntegration_ConfigLoadsHooks(t *testing.T) {
	skipIfNoShell(t)

	dir := setupProject(t)
	outFile := filepath.Join(dir, "config_hook.json")
	scriptPath := writeScript(t, dir, "from_config", outFile)

	configYAML := fmt.Sprintf(`hooks:
  on_feature_complete:
    - run: "%s"
  on_bug_complete:
    - run: "%s"
  on_task_complete:
    - run: "%s"
`, filepath.ToSlash(scriptPath), filepath.ToSlash(scriptPath), filepath.ToSlash(scriptPath))

	if err := os.WriteFile(filepath.Join(dir, ".maggus", "config.yml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if len(cfg.Hooks.OnFeatureComplete) != 1 {
		t.Errorf("OnFeatureComplete hooks = %d, want 1", len(cfg.Hooks.OnFeatureComplete))
	}
	if len(cfg.Hooks.OnBugComplete) != 1 {
		t.Errorf("OnBugComplete hooks = %d, want 1", len(cfg.Hooks.OnBugComplete))
	}
	if len(cfg.Hooks.OnTaskComplete) != 1 {
		t.Errorf("OnTaskComplete hooks = %d, want 1", len(cfg.Hooks.OnTaskComplete))
	}

	// Verify one of them actually works end-to-end
	event := hooks.Event{
		Type:      "feature_complete",
		File:      "feature_test.md",
		Title:     "Config Test",
		Tasks:     []hooks.TaskInfo{{ID: "TASK-TEST-001", Title: "Test"}},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	logger, _ := newTestLogger()
	hooks.Run(cfg.Hooks.OnFeatureComplete, event, dir, logger)

	assertHookPayload(t, outFile, "feature_complete", "feature_test.md", "Config Test", 1)
}

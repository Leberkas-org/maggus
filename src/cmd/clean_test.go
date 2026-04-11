package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to set up .maggus dir structure for clean tests
func setupCleanDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	featuresDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bugsDir := filepath.Join(dir, ".maggus", "bugs")
	if err := os.MkdirAll(bugsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeBugFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".maggus", "bugs", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFeatureFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".maggus", "features", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCleanCmd(t *testing.T, dir string, flags ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := *cleanCmd
	cmd.ResetFlags()
	cmd.Flags().Bool("dry-run", false, "Dry run")
	if err := cmd.ParseFlags(flags); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	cmd.SetOut(&buf)

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if err := runClean(&cmd, dir, dryRun); err != nil {
		t.Fatalf("runClean: %v", err)
	}
	return buf.String()
}

func TestCleanRemovesCompletedFeatures(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001_completed.md", "# completed feature")
	writeFeatureFile(t, dir, "feature_002_completed.md", "# another completed feature")
	writeFeatureFile(t, dir, "feature_003.md", "# active feature")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "2 completed feature file(s) and 0 completed bug file(s)") {
		t.Errorf("expected '2 completed feature file(s) and 0 completed bug file(s)' in output, got:\n%s", out)
	}

	// Completed features should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); !os.IsNotExist(err) {
		t.Error("feature_001_completed.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_002_completed.md")); !os.IsNotExist(err) {
		t.Error("feature_002_completed.md should have been removed")
	}

	// Active feature should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_003.md")); err != nil {
		t.Error("feature_003.md should still exist")
	}
}

func TestCleanDryRun(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001_completed.md", "# completed")

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected 'Dry run' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Would remove") {
		t.Errorf("expected 'Would remove' in output, got:\n%s", out)
	}

	// Files should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); err != nil {
		t.Error("feature_001_completed.md should still exist in dry-run mode")
	}
}

func TestCleanNothingToClean(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001.md", "# active feature")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' in output, got:\n%s", out)
	}
}

func TestCleanEmptyMaggusDir(t *testing.T) {
	dir := setupCleanDir(t)

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' in output, got:\n%s", out)
	}
}

func TestCleanDryRunShowsPaths(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_005_completed.md", "# done")

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "feature_005_completed.md") {
		t.Errorf("expected feature filename in dry-run output, got:\n%s", out)
	}
}

func TestCleanRemovesCompletedBugs(t *testing.T) {
	dir := setupCleanDir(t)
	writeBugFile(t, dir, "bug_001_completed.md", "# completed bug")
	writeBugFile(t, dir, "bug_002.md", "# active bug")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "0 completed feature file(s) and 1 completed bug file(s)") {
		t.Errorf("expected '0 completed feature file(s) and 1 completed bug file(s)' in output, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")); !os.IsNotExist(err) {
		t.Error("bug_001_completed.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_002.md")); err != nil {
		t.Error("bug_002.md should still exist")
	}
}

func TestCleanDryRunShowsBugPaths(t *testing.T) {
	dir := setupCleanDir(t)
	writeBugFile(t, dir, "bug_003_completed.md", "# done")

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "bug_003_completed.md") {
		t.Errorf("expected bug filename in dry-run output, got:\n%s", out)
	}
	if !strings.Contains(out, "Would remove") {
		t.Errorf("expected 'Would remove' in dry-run output, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_003_completed.md")); err != nil {
		t.Error("bug_003_completed.md should still exist in dry-run mode")
	}
}

func TestCleanMixedFeaturesAndBugs(t *testing.T) {
	dir := setupCleanDir(t)
	writeFeatureFile(t, dir, "feature_001_completed.md", "# done feature")
	writeBugFile(t, dir, "bug_001_completed.md", "# done bug")
	writeBugFile(t, dir, "bug_002_completed.md", "# done bug 2")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "1 completed feature file(s) and 2 completed bug file(s)") {
		t.Errorf("expected '1 completed feature file(s) and 2 completed bug file(s)' in output, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_completed.md")); !os.IsNotExist(err) {
		t.Error("feature_001_completed.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_completed.md")); !os.IsNotExist(err) {
		t.Error("bug_001_completed.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_002_completed.md")); !os.IsNotExist(err) {
		t.Error("bug_002_completed.md should have been removed")
	}
}

func TestCleanNothingToCleanNoBugsOrFeatures(t *testing.T) {
	dir := setupCleanDir(t)
	writeBugFile(t, dir, "bug_001.md", "# active bug")
	writeFeatureFile(t, dir, "feature_001.md", "# active feature")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' in output, got:\n%s", out)
	}
}

func TestCleanDispatchWorkers(t *testing.T) {
	dir := setupCleanDir(t)
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a workers index with a done and a failed entry.
	writeWorkersIndex(t, dir, []map[string]string{
		{"task_id": "TASK-001", "status": "done"},
		{"task_id": "TASK-002", "status": "failed"},
		{"task_id": "TASK-003", "status": "working"},
	})

	// Write per-worker snapshots for the finished workers.
	writeWorkerSnapshot(t, dir, "TASK-001")
	writeWorkerSnapshot(t, dir, "TASK-002")

	// Create fake worktree directories (no git, but directories exist).
	worktreesDir := filepath.Join(dir, ".maggus", "worktrees")
	for _, id := range []string{"TASK-001", "TASK-002"} {
		if err := os.MkdirAll(filepath.Join(worktreesDir, id), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "dispatch worker") {
		t.Errorf("expected dispatch worker cleanup in output, got:\n%s", out)
	}

	// Snapshots for done/failed workers should be removed.
	for _, id := range []string{"TASK-001", "TASK-002"} {
		snapPath := filepath.Join(runsDir, "state-"+id+".json")
		if _, err := os.Stat(snapPath); !os.IsNotExist(err) {
			t.Errorf("snapshot for %s should have been removed", id)
		}
	}

	// Workers index should no longer contain done/failed entries.
	indexPath := filepath.Join(runsDir, "state-workers.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("workers index should still exist (working entry remains): %v", err)
	}
	if strings.Contains(string(data), "TASK-001") || strings.Contains(string(data), "TASK-002") {
		t.Errorf("workers index should not contain cleaned entries; got: %s", data)
	}
	if !strings.Contains(string(data), "TASK-003") {
		t.Errorf("workers index should still contain active TASK-003; got: %s", data)
	}
}

func TestCleanDispatchWorkersDryRun(t *testing.T) {
	dir := setupCleanDir(t)
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeWorkersIndex(t, dir, []map[string]string{
		{"task_id": "TASK-001", "status": "done"},
	})
	writeWorkerSnapshot(t, dir, "TASK-001")

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected 'Dry run' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "TASK-001") {
		t.Errorf("expected TASK-001 in dry-run output, got:\n%s", out)
	}

	// Snapshot should still exist after dry run.
	snapPath := filepath.Join(runsDir, "state-TASK-001.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Error("snapshot should still exist after dry run")
	}
}

func TestCleanNothingToCleanWithWorkingWorker(t *testing.T) {
	dir := setupCleanDir(t)
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Only a "working" entry — not a candidate for cleanup.
	writeWorkersIndex(t, dir, []map[string]string{
		{"task_id": "TASK-001", "status": "working"},
	})

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' for active worker, got:\n%s", out)
	}
}

// writeWorkersIndex writes a fake workers index JSON for testing.
func writeWorkersIndex(t *testing.T, dir string, workers []map[string]string) {
	t.Helper()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	buf.WriteString(`{"workers":[`)
	for i, w := range workers {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(`{"task_id":"` + w["task_id"] + `","task_title":"` + w["task_id"] + `","status":"` + w["status"] + `"}`)
	}
	buf.WriteString(`]}`)

	path := filepath.Join(runsDir, "state-workers.json")
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeWorkerSnapshot writes a minimal per-worker snapshot for testing.
func writeWorkerSnapshot(t *testing.T, dir, taskID string) {
	t.Helper()
	runsDir := filepath.Join(dir, ".maggus", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"task_id":"` + taskID + `","status":"Done"}`
	path := filepath.Join(runsDir, "state-"+taskID+".json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

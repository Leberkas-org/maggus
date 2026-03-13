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
	maggusDir := filepath.Join(dir, ".maggus")
	runsDir := filepath.Join(maggusDir, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writePlanFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".maggus", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRunDir(t *testing.T, dir, runID, runMdContent string) {
	t.Helper()
	runDir := filepath.Join(dir, ".maggus", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.md"), []byte(runMdContent), 0o644); err != nil {
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

func TestCleanRemovesCompletedPlans(t *testing.T) {
	dir := setupCleanDir(t)
	writePlanFile(t, dir, "plan_1_completed.md", "# completed plan")
	writePlanFile(t, dir, "plan_2_completed.md", "# another completed plan")
	writePlanFile(t, dir, "plan_3.md", "# active plan")

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "2 completed plan(s)") {
		t.Errorf("expected '2 completed plan(s)' in output, got:\n%s", out)
	}

	// Completed plans should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1_completed.md")); !os.IsNotExist(err) {
		t.Error("plan_1_completed.md should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_2_completed.md")); !os.IsNotExist(err) {
		t.Error("plan_2_completed.md should have been removed")
	}

	// Active plan should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_3.md")); err != nil {
		t.Error("plan_3.md should still exist")
	}
}

func TestCleanRemovesCompletedRuns(t *testing.T) {
	dir := setupCleanDir(t)

	finishedRunMd := "# Run Log\n\n- **RUN_ID:** 20260101-120000\n\n## End\n\n- **End Time:** 2026-01-01T13:00:00Z\n"
	inProgressRunMd := "# Run Log\n\n- **RUN_ID:** 20260102-120000\n"

	writeRunDir(t, dir, "20260101-120000", finishedRunMd)
	writeRunDir(t, dir, "20260102-120000", inProgressRunMd)

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "1 run directory(ies)") {
		t.Errorf("expected '1 run directory(ies)' in output, got:\n%s", out)
	}

	// Finished run should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "runs", "20260101-120000")); !os.IsNotExist(err) {
		t.Error("finished run directory should have been removed")
	}

	// In-progress run should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "runs", "20260102-120000")); err != nil {
		t.Error("in-progress run directory should still exist")
	}
}

func TestCleanDryRun(t *testing.T) {
	dir := setupCleanDir(t)
	writePlanFile(t, dir, "plan_1_completed.md", "# completed")

	finishedRunMd := "# Run Log\n\n## End\n"
	writeRunDir(t, dir, "20260101-120000", finishedRunMd)

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected 'Dry run' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Would remove") {
		t.Errorf("expected 'Would remove' in output, got:\n%s", out)
	}

	// Files should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "plan_1_completed.md")); err != nil {
		t.Error("plan_1_completed.md should still exist in dry-run mode")
	}
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "runs", "20260101-120000")); err != nil {
		t.Error("run directory should still exist in dry-run mode")
	}
}

func TestCleanNothingToClean(t *testing.T) {
	dir := setupCleanDir(t)
	writePlanFile(t, dir, "plan_1.md", "# active plan")

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

func TestCleanKeepsInProgressRuns(t *testing.T) {
	dir := setupCleanDir(t)

	inProgressRunMd := "# Run Log\n\n- **RUN_ID:** 20260102-120000\n"
	writeRunDir(t, dir, "20260102-120000", inProgressRunMd)

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' with only in-progress runs, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".maggus", "runs", "20260102-120000")); err != nil {
		t.Error("in-progress run directory should still exist")
	}
}

func TestCleanDryRunShowsPaths(t *testing.T) {
	dir := setupCleanDir(t)
	writePlanFile(t, dir, "plan_5_completed.md", "# done")

	out := runCleanCmd(t, dir, "--dry-run")

	if !strings.Contains(out, "plan_5_completed.md") {
		t.Errorf("expected plan filename in dry-run output, got:\n%s", out)
	}
}

func TestCleanCombinedPlansAndRuns(t *testing.T) {
	dir := setupCleanDir(t)
	writePlanFile(t, dir, "plan_1_completed.md", "# done 1")
	writePlanFile(t, dir, "plan_2_completed.md", "# done 2")
	writePlanFile(t, dir, "plan_3_completed.md", "# done 3")

	writeRunDir(t, dir, "run-a", "# Run\n\n## End\n")
	writeRunDir(t, dir, "run-b", "# Run\n\n## End\n")
	writeRunDir(t, dir, "run-c", "# Run\n") // in progress

	out := runCleanCmd(t, dir)

	if !strings.Contains(out, "3 completed plan(s)") {
		t.Errorf("expected '3 completed plan(s)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2 run directory(ies)") {
		t.Errorf("expected '2 run directory(ies)' in output, got:\n%s", out)
	}

	// In-progress run should survive
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "runs", "run-c")); err != nil {
		t.Error("in-progress run-c should still exist")
	}
}

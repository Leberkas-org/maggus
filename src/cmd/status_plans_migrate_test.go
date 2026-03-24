package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
)

// setupMigrateDir creates a temp dir with .maggus/features/ and .maggus/bugs/ subdirs.
func setupMigrateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"features", "bugs"} {
		if err := os.MkdirAll(filepath.Join(dir, ".maggus", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func writeIgnoredFeature(t *testing.T, dir, filename string) {
	t.Helper()
	content := "# Feature\n\n### TASK-001-001: Sample\n- [ ] Do something\n"
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "features", filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeIgnoredBug(t *testing.T, dir, filename string) {
	t.Helper()
	content := "# Bug\n\n### BUG-001-001: Sample\n- [ ] Do something\n"
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "bugs", filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateIgnoredFiles_NoIgnoredFiles(t *testing.T) {
	dir := setupMigrateDir(t)
	// Write a normal feature file (not _ignored)
	writeIgnoredFeature(t, dir, "feature_001.md")

	if err := migrateIgnoredFiles(dir, "features"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Normal file should still exist unchanged
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); err != nil {
		t.Error("feature_001.md should still exist")
	}

	// No approval entries should have been written
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 0 {
		t.Errorf("expected no approval entries, got %v", a)
	}
}

func TestMigrateIgnoredFiles_OneIgnoredFeature(t *testing.T) {
	dir := setupMigrateDir(t)
	writeIgnoredFeature(t, dir, "feature_001_ignored.md")

	if err := migrateIgnoredFiles(dir, "features"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _ignored.md should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001_ignored.md")); !os.IsNotExist(err) {
		t.Error("feature_001_ignored.md should have been renamed")
	}
	// Renamed file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); err != nil {
		t.Error("feature_001.md should exist after migration")
	}

	// Should be marked unapproved
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	approved, ok := a["feature_001"]
	if !ok {
		t.Error("expected feature_001 to have an approval entry")
	}
	if approved {
		t.Error("expected feature_001 to be unapproved (false)")
	}
}

func TestMigrateIgnoredFiles_IdempotentWhenAlreadyMigrated(t *testing.T) {
	dir := setupMigrateDir(t)
	// Simulate state where _ignored.md already coexists with the plain .md
	writeIgnoredFeature(t, dir, "feature_001_ignored.md")
	writeIgnoredFeature(t, dir, "feature_001.md")

	if err := migrateIgnoredFiles(dir, "features"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both files may exist (rename skipped), but the plain .md must exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); err != nil {
		t.Error("feature_001.md should still exist")
	}

	// Should be marked unapproved
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	approved, ok := a["feature_001"]
	if !ok {
		t.Error("expected feature_001 to have an approval entry")
	}
	if approved {
		t.Error("expected feature_001 to be unapproved (false)")
	}
}

func TestMigrateIgnoredFiles_BugFiles(t *testing.T) {
	dir := setupMigrateDir(t)
	writeIgnoredBug(t, dir, "bug_001_ignored.md")

	if err := migrateIgnoredFiles(dir, "bugs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _ignored.md should be gone
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001_ignored.md")); !os.IsNotExist(err) {
		t.Error("bug_001_ignored.md should have been renamed")
	}
	// Renamed file should exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001.md")); err != nil {
		t.Error("bug_001.md should exist after migration")
	}

	// Should be marked unapproved
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	approved, ok := a["bug_001"]
	if !ok {
		t.Error("expected bug_001 to have an approval entry")
	}
	if approved {
		t.Error("expected bug_001 to be unapproved (false)")
	}
}

func TestParseFeatures_MigratesIgnoredOnLoad(t *testing.T) {
	dir := setupMigrateDir(t)
	writeIgnoredFeature(t, dir, "feature_001_ignored.md")

	features, err := parseFeatures(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be renamed
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "features", "feature_001.md")); err != nil {
		t.Error("feature_001.md should exist after parseFeatures migration")
	}

	// Should be included in result with renamed filename
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	if features[0].filename != "feature_001.md" {
		t.Errorf("expected filename feature_001.md, got %s", features[0].filename)
	}

	// Should be marked unapproved in approval store
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a["feature_001"] {
		t.Error("expected feature_001 to be unapproved")
	}
}

func TestParseBugs_MigratesIgnoredOnLoad(t *testing.T) {
	dir := setupMigrateDir(t)
	writeIgnoredBug(t, dir, "bug_001_ignored.md")

	bugs, err := parseBugs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be renamed
	if _, err := os.Stat(filepath.Join(dir, ".maggus", "bugs", "bug_001.md")); err != nil {
		t.Error("bug_001.md should exist after parseBugs migration")
	}

	// Should be included in result with renamed filename
	if len(bugs) != 1 {
		t.Fatalf("expected 1 bug, got %d", len(bugs))
	}
	if bugs[0].filename != "bug_001.md" {
		t.Errorf("expected filename bug_001.md, got %s", bugs[0].filename)
	}

	// Should be marked unapproved in approval store
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a["bug_001"] {
		t.Error("expected bug_001 to be unapproved")
	}
}

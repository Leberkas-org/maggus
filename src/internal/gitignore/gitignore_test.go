package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureEntries_CreatesGitignoreWithAllEntries(t *testing.T) {
	dir := t.TempDir()

	added, err := EnsureEntries(dir)
	if err != nil {
		t.Fatalf("EnsureEntries failed: %v", err)
	}

	if len(added) != len(requiredEntries) {
		t.Errorf("expected %d entries added, got %d", len(requiredEntries), len(added))
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	for _, entry := range requiredEntries {
		if !strings.Contains(string(content), entry) {
			t.Errorf("expected .gitignore to contain %q", entry)
		}
	}
}

func TestEnsureEntries_IncludesMaggusWork(t *testing.T) {
	dir := t.TempDir()

	added, err := EnsureEntries(dir)
	if err != nil {
		t.Fatalf("EnsureEntries failed: %v", err)
	}

	found := false
	for _, entry := range added {
		if entry == ".maggus-work/" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .maggus-work/ to be in added entries")
	}
}

func TestEnsureEntries_DoesNotDuplicateExisting(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate with some entries
	initial := ".maggus/runs\n.maggus-work/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial .gitignore: %v", err)
	}

	added, err := EnsureEntries(dir)
	if err != nil {
		t.Fatalf("EnsureEntries failed: %v", err)
	}

	// Should not re-add .maggus/runs or .maggus-work/
	for _, entry := range added {
		if entry == ".maggus/runs" || entry == ".maggus-work/" {
			t.Errorf("should not have re-added %q", entry)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	// Verify all required entries are present
	for _, entry := range requiredEntries {
		if !strings.Contains(string(content), entry) {
			t.Errorf("expected .gitignore to contain %q", entry)
		}
	}
}

func TestEnsureEntries_NoChangesWhenAllPresent(t *testing.T) {
	dir := t.TempDir()

	all := strings.Join(requiredEntries, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(all), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	added, err := EnsureEntries(dir)
	if err != nil {
		t.Fatalf("EnsureEntries failed: %v", err)
	}

	if added != nil {
		t.Errorf("expected no entries added, got %v", added)
	}
}

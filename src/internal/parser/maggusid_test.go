package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureMaggusID_ExistingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_001.md")
	existingID := "12345678-1234-4234-8234-123456789abc"
	content := "<!-- maggus-id: " + existingID + " -->\n# Feature 001: Test\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := EnsureMaggusID(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != existingID {
		t.Errorf("got %q, want %q", id, existingID)
	}

	// File must not be modified
	data, _ := os.ReadFile(path)
	if string(data) != content {
		t.Error("file was modified despite existing maggus-id")
	}
}

func TestEnsureMaggusID_NoID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_002.md")
	original := "# Feature 002: No ID\n\nSome content.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := EnsureMaggusID(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty UUID")
	}
	if len(id) != 36 {
		t.Errorf("UUID has unexpected length %d: %q", len(id), id)
	}

	// File must now start with the maggus-id comment
	data, _ := os.ReadFile(path)
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	expected := "<!-- maggus-id: " + id + " -->"
	if firstLine != expected {
		t.Errorf("first line = %q, want %q", firstLine, expected)
	}

	// Original content must follow
	if !strings.Contains(string(data), original) {
		t.Error("original content not preserved after prepending maggus-id")
	}

	// ParseMaggusID must now return the same UUID
	if got := ParseMaggusID(path); got != id {
		t.Errorf("ParseMaggusID returned %q, want %q", got, id)
	}
}

func TestEnsureMaggusID_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature_003.md")
	if err := os.WriteFile(path, []byte("# Feature 003: Idempotent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	id1, err := EnsureMaggusID(path)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := EnsureMaggusID(path)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("second call returned different UUID: %q vs %q", id1, id2)
	}

	// Only one maggus-id line should be in the file
	data, _ := os.ReadFile(path)
	count := strings.Count(string(data), "maggus-id:")
	if count != 1 {
		t.Errorf("expected 1 maggus-id line, found %d", count)
	}
}

func TestEnsureMaggusID_MissingFile(t *testing.T) {
	_, err := EnsureMaggusID("/nonexistent/path/file.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

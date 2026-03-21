package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProjectHash(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "windows path with drive letter",
			path: `C:\c\maggus`,
			want: "C--c-maggus",
		},
		{
			name: "windows path with users",
			path: `C:\Users\Dirnei`,
			want: "C--Users-Dirnei",
		},
		{
			name: "unix absolute path",
			path: "/home/user/project",
			want: "-home-user-project",
		},
		{
			name: "simple unix path",
			path: "/c",
			want: "-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectHash(tt.path)
			if got != tt.want {
				t.Errorf("ProjectHash(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSnapshotDir(t *testing.T) {
	dir := t.TempDir()

	// Create some .jsonl files and a directory
	for _, name := range []string{"aaa.jsonl", "bbb.jsonl", "ccc.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	snap, err := SnapshotDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(snap) != 2 {
		t.Fatalf("expected 2 jsonl files, got %d: %v", len(snap), snap)
	}
	if !snap["aaa.jsonl"] || !snap["bbb.jsonl"] {
		t.Errorf("expected aaa.jsonl and bbb.jsonl in snapshot, got %v", snap)
	}
	if snap["ccc.txt"] {
		t.Error("non-jsonl file should not be in snapshot")
	}
}

func TestSnapshotDir_NonExistent(t *testing.T) {
	snap, err := SnapshotDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot for non-existent dir, got %v", snap)
	}
}

func TestDetectNewSessions(t *testing.T) {
	dir := t.TempDir()

	// Create initial files
	os.WriteFile(filepath.Join(dir, "existing.jsonl"), []byte("{}"), 0644)

	// Take snapshot
	before, err := SnapshotDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate Claude creating a new session file
	newFile := "new-session-id.jsonl"
	os.WriteFile(filepath.Join(dir, newFile), []byte("{}"), 0644)

	// Detect new files
	newFiles, err := DetectNewSessions(dir, before)
	if err != nil {
		t.Fatal(err)
	}

	if len(newFiles) != 1 {
		t.Fatalf("expected 1 new file, got %d: %v", len(newFiles), newFiles)
	}

	expected := filepath.Join(dir, newFile)
	if newFiles[0] != expected {
		t.Errorf("expected %q, got %q", expected, newFiles[0])
	}
}

func TestDetectNewSessions_NoNewFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "existing.jsonl"), []byte("{}"), 0644)

	before, err := SnapshotDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// No new files created
	newFiles, err := DetectNewSessions(dir, before)
	if err != nil {
		t.Fatal(err)
	}

	if len(newFiles) != 0 {
		t.Errorf("expected 0 new files, got %d: %v", len(newFiles), newFiles)
	}
}

func TestDetectNewSessions_MultipleNewFiles(t *testing.T) {
	dir := t.TempDir()

	before, err := SnapshotDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple new session files
	os.WriteFile(filepath.Join(dir, "session-1.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "session-2.jsonl"), []byte("{}"), 0644)

	newFiles, err := DetectNewSessions(dir, before)
	if err != nil {
		t.Fatal(err)
	}

	if len(newFiles) != 2 {
		t.Fatalf("expected 2 new files, got %d: %v", len(newFiles), newFiles)
	}
}

func TestSessionDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	var workDir string
	var expectedHash string
	if runtime.GOOS == "windows" {
		workDir = `C:\c\maggus`
		expectedHash = "C--c-maggus"
	} else {
		workDir = "/home/user/maggus"
		expectedHash = "-home-user-maggus"
	}

	dir, err := SessionDir(workDir)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(home, ".claude", "projects", expectedHash)
	if dir != expected {
		t.Errorf("SessionDir(%q) = %q, want %q", workDir, dir, expected)
	}
}

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInitCmd executes the init command's RunE in the given temp directory.
func runInitCmd(t *testing.T, dir string) (string, error) {
	t.Helper()
	t.Chdir(dir)

	var buf bytes.Buffer
	cmd := *initCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Ensure caps.HasClaude is false so the plugin step is skipped.
	oldCaps := caps
	caps.HasClaude = false
	t.Cleanup(func() { caps = oldCaps })

	err := cmd.RunE(&cmd, nil)
	return buf.String(), err
}

func TestInitCreatesMaggusDir(t *testing.T) {
	dir := t.TempDir()
	_, err := runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".maggus"))
	if err != nil {
		t.Fatal(".maggus/ directory was not created")
	}
	if !info.IsDir() {
		t.Fatal(".maggus should be a directory")
	}
}

func TestInitCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	configPath := filepath.Join(dir, ".maggus", "config.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal("config.yml was not created")
	}

	if string(content) != defaultConfig {
		t.Errorf("config.yml content mismatch\ngot:\n%s\nwant:\n%s", string(content), defaultConfig)
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()

	// Run init the first time
	_, err := runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Write custom content into config.yml to verify it's not overwritten
	configPath := filepath.Join(dir, ".maggus", "config.yml")
	customContent := "# custom config\nmodel: opus\n"
	if err := os.WriteFile(configPath, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Record gitignore state before second run
	gitignoreBefore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	// Run init a second time — should not error
	_, err = runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("second init failed: %v", err)
	}

	// Config should not have been overwritten
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != customContent {
		t.Error("config.yml was overwritten on second init")
	}

	// .maggus/ directory should still exist
	if _, err := os.Stat(filepath.Join(dir, ".maggus")); err != nil {
		t.Error(".maggus/ directory should still exist after second init")
	}

	// .gitignore should be unchanged (entries already present)
	gitignoreAfter, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gitignoreAfter) != string(gitignoreBefore) {
		t.Error(".gitignore was modified on second init when entries already present")
	}
}

func TestInitAddsGitignoreEntries(t *testing.T) {
	dir := t.TempDir()
	_, err := runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	gitignorePath := filepath.Join(dir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(".gitignore was not created")
	}

	requiredEntries := []string{
		".maggus/runs",
		".maggus/MEMORY.md",
		".maggus/RELEASE_NOTES.md",
		".maggus/usage.csv",
		".maggus/usage_work.jsonl",
		".maggus/usage_prompt.jsonl",
		".maggus/locks/",
		".maggus-work/",
		"COMMIT.md",
	}

	for _, entry := range requiredEntries {
		if !strings.Contains(string(content), entry) {
			t.Errorf(".gitignore missing entry %q", entry)
		}
	}
}

func TestInitExistingGitignorePreservesContent(t *testing.T) {
	dir := t.TempDir()

	// Create a .gitignore with existing content
	existing := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runInitCmd(t, dir)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	// Original entries should still be present
	if !strings.Contains(string(content), "node_modules/") {
		t.Error("existing .gitignore content was lost")
	}
	if !strings.Contains(string(content), "*.log") {
		t.Error("existing .gitignore content was lost")
	}

	// Maggus entries should also be present
	if !strings.Contains(string(content), ".maggus/runs") {
		t.Error(".maggus/runs missing from .gitignore")
	}
}

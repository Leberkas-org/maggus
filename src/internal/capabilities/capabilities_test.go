package capabilities

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setTestDir overrides the env var that userDir() reads so configFile()
// points into a temp directory controlled by the test.
func setTestDir(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("HOME", dir)
	}
}

func TestConfigFile_ReturnsNonEmptyPath(t *testing.T) {
	tmp := t.TempDir()
	setTestDir(t, tmp)

	path := configFile()
	if path == "" {
		t.Fatal("configFile() returned empty string")
	}
	if filepath.Base(path) != "capabilities.json" {
		t.Errorf("expected filename capabilities.json, got %s", filepath.Base(path))
	}
}

func TestLoad_ReturnsZeroValueWhenFileDoesNotExist(t *testing.T) {
	tmp := t.TempDir()
	setTestDir(t, tmp)
	// No file written — cache does not exist.

	caps := Load()
	if caps.HasClaude || caps.HasOpenCode {
		t.Errorf("expected zero-value Capabilities, got %+v", caps)
	}
}

func TestLoad_ReturnsZeroValueWhenFileContainsInvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	setTestDir(t, tmp)

	path := configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	caps := Load()
	if caps.HasClaude || caps.HasOpenCode {
		t.Errorf("expected zero-value Capabilities for invalid JSON, got %+v", caps)
	}
}

func TestLoad_DeserializesValidJSON(t *testing.T) {
	tmp := t.TempDir()
	setTestDir(t, tmp)

	path := configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	want := Capabilities{HasClaude: true, HasOpenCode: false}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got := Load()
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_DeserializesBothTrue(t *testing.T) {
	tmp := t.TempDir()
	setTestDir(t, tmp)

	path := configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	want := Capabilities{HasClaude: true, HasOpenCode: true}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got := Load()
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestWrite_CreatesDirectoryAndWritesValidJSON(t *testing.T) {
	tmp := t.TempDir()
	// Use a nested path that doesn't exist yet.
	path := filepath.Join(tmp, "nested", "dir", "capabilities.json")

	caps := Capabilities{HasClaude: true, HasOpenCode: false}
	write(path, caps)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("write() did not create file: %v", err)
	}

	var got Capabilities
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("write() produced invalid JSON: %v", err)
	}
	if got != caps {
		t.Errorf("written caps = %+v, want %+v", got, caps)
	}
}

// Package capabilities detects available CLI tools and caches the result
// to a global config file so it persists across runs.
package capabilities

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Capabilities holds detected tool availability.
type Capabilities struct {
	HasClaude   bool `json:"has_claude"`
	HasOpenCode bool `json:"has_opencode"`
}

// configFile returns the path to the capabilities cache file.
func configFile() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "capabilities.json")
}

// Detect checks which CLI tools are available on PATH and caches the result.
func Detect() Capabilities {
	caps := Capabilities{
		HasClaude:   isOnPath("claude"),
		HasOpenCode: isOnPath("opencode"),
	}
	if path := configFile(); path != "" {
		write(path, caps)
	}
	return caps
}

// Load reads the cached capabilities from disk.
// Returns zero-value Capabilities if the file doesn't exist or is unreadable.
func Load() Capabilities {
	path := configFile()
	if path == "" {
		return Capabilities{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Capabilities{}
	}
	var caps Capabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		return Capabilities{}
	}
	return caps
}

func isOnPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func write(path string, caps Capabilities) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(caps, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func userDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "maggus")
		}
		return ""
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".maggus")
		}
		return ""
	}
}

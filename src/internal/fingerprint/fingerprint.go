package fingerprint

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Get returns a stable UUID for this machine. On first call it generates a
// UUID v4 and persists it; subsequent calls return the stored value. It tries
// a system-level path first and falls back to a user-level path.
func Get() (string, error) {
	return get(systemDir(), userDir())
}

func get(systemPath, userPath string) (string, error) {
	// Try reading from system path first, then user path.
	for _, dir := range []string{systemPath, userPath} {
		if dir == "" {
			continue
		}
		fp := filepath.Join(dir, "fingerprint")
		if id, err := readFingerprint(fp); err == nil {
			return id, nil
		}
	}

	// No existing fingerprint found — generate one.
	id, err := generateUUID()
	if err != nil {
		return "", fmt.Errorf("generating fingerprint: %w", err)
	}

	// Try writing to system path first, fall back to user path.
	for _, dir := range []string{systemPath, userPath} {
		if dir == "" {
			continue
		}
		fp := filepath.Join(dir, "fingerprint")
		if writeErr := writeFingerprint(fp, id); writeErr == nil {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not write fingerprint to any path")
}

func readFingerprint(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if !isValidUUID(id) {
		return "", fmt.Errorf("invalid UUID in %s", path)
	}
	return id, nil
}

func writeFingerprint(path string, id string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(id+"\n"), 0o644)
}

// generateUUID produces a UUID v4 using crypto/rand.
func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version 4 (bits 12-15 of time_hi_and_version).
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant (bits 6-7 of clock_seq_hi_and_reserved).
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// isValidUUID checks for the 8-4-4-4-12 hex format.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func systemDir() string {
	switch runtime.GOOS {
	case "windows":
		return `C:\Program Files\maggus`
	case "darwin":
		return "/Library/Application Support/maggus"
	default:
		return "/usr/local/share/maggus"
	}
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

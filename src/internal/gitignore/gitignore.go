package gitignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var requiredEntries = []string{
	".maggus/runs",
	".maggus/MEMORY.md",
	".maggus/locks/",
	".maggus-work/",
	"COMMIT.md",
}

// EnsureEntries checks the .gitignore in dir and appends any missing required entries.
// Returns a list of entries that were added (empty if none were needed).
func EnsureEntries(dir string) ([]string, error) {
	path := filepath.Join(dir, ".gitignore")

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read .gitignore: %w", err)
	}

	existing := string(content)
	lines := strings.Split(existing, "\n")

	// Build set of existing entries (trimmed)
	have := make(map[string]bool, len(lines))
	for _, line := range lines {
		have[strings.TrimSpace(line)] = true
	}

	var missing []string
	for _, entry := range requiredEntries {
		if !have[entry] {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil, nil
	}

	// Ensure existing content ends with a newline before appending
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}

	existing += strings.Join(missing, "\n") + "\n"

	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		return nil, fmt.Errorf("write .gitignore: %w", err)
	}

	return missing, nil
}

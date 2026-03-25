package cmd

import (
	"os"
	"path/filepath"
)

// isInitialized returns true if the .maggus/ directory exists in the current working directory.
func isInitialized() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, ".maggus"))
	return err == nil && info.IsDir()
}

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestCheckCriterionInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature.md")
	content := "### TASK-001: Test\n- [ ] First criterion\n- [ ] Second criterion\n- [x] Third done\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := checkCriterionInFile(path, "First criterion"); err != nil {
		t.Fatalf("checkCriterionInFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "- [x] First criterion") {
		t.Error("expected criterion to be checked")
	}
	if !strings.Contains(got, "- [ ] Second criterion") {
		t.Error("expected second criterion to remain unchecked")
	}
}

func taskIDs(tasks []parser.Task) []string {
	var result []string
	for _, t := range tasks {
		result = append(result, t.ID)
	}
	return result
}

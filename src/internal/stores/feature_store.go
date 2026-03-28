package stores

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
)

// planIDFromPath returns the base filename with .md and _completed suffixes stripped.
func planIDFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".md")
	base = strings.TrimSuffix(base, "_completed")
	return base
}

// FileFeatureStore is a file-backed implementation of FeatureStore that delegates
// to parser functions operating on .maggus/features/ files.
type FileFeatureStore struct {
	dir string
}

// NewFileFeatureStore returns a FileFeatureStore rooted at dir (the project root).
func NewFileFeatureStore(dir string) *FileFeatureStore {
	return &FileFeatureStore{dir: dir}
}

// Compile-time assertion that FileFeatureStore satisfies FeatureStore.
var _ FeatureStore = &FileFeatureStore{}

// LoadAll returns all feature plans. When includeCompleted is false, _completed.md files are excluded.
func (s *FileFeatureStore) LoadAll(includeCompleted bool) ([]parser.Plan, error) {
	files, err := parser.GlobFeatureFiles(s.dir, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("glob feature files: %w", err)
	}

	plans := make([]parser.Plan, 0, len(files))
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, parser.Plan{
			ID:        planIDFromPath(f),
			MaggusID:  parser.ParseMaggusID(f),
			File:      f,
			Tasks:     tasks,
			IsBug:     false,
			Completed: strings.HasSuffix(f, "_completed.md"),
		})
	}
	return plans, nil
}

// MarkCompleted renames (or deletes) fully-completed feature files.
func (s *FileFeatureStore) MarkCompleted(action string) ([]string, error) {
	return parser.MarkCompletedFeatures(s.dir, action)
}

// GlobFiles returns the raw file paths for all feature files.
func (s *FileFeatureStore) GlobFiles(includeCompleted bool) ([]string, error) {
	return parser.GlobFeatureFiles(s.dir, includeCompleted)
}

// DeleteTask removes a task section from the given file.
func (s *FileFeatureStore) DeleteTask(filePath, taskID string) error {
	return parser.DeleteTask(filePath, taskID)
}

// UnblockCriterion removes the BLOCKED: prefix from a criterion line.
func (s *FileFeatureStore) UnblockCriterion(filePath string, c parser.Criterion) error {
	return parser.UnblockCriterion(filePath, c)
}

// ResolveCriterion removes the BLOCKED: prefix and marks a criterion as checked.
func (s *FileFeatureStore) ResolveCriterion(filePath string, c parser.Criterion) error {
	return parser.ResolveCriterion(filePath, c)
}

// DeleteCriterion removes a criterion line from the given file.
func (s *FileFeatureStore) DeleteCriterion(filePath string, c parser.Criterion) error {
	return parser.DeleteCriterion(filePath, c)
}

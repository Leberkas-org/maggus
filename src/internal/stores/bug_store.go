package stores

import (
	"fmt"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
)

// FileBugStore is a file-backed implementation of BugStore that delegates
// to parser functions operating on .maggus/bugs/ files.
type FileBugStore struct {
	dir string
}

// NewFileBugStore returns a FileBugStore rooted at dir (the project root).
func NewFileBugStore(dir string) *FileBugStore {
	return &FileBugStore{dir: dir}
}

// Compile-time assertion that FileBugStore satisfies BugStore.
var _ BugStore = &FileBugStore{}

// LoadAll returns all bug plans. When includeCompleted is false, _completed.md files are excluded.
// Legacy TASK-NNN headings in bug files are migrated to BUG-NNN-XXX format before parsing.
func (s *FileBugStore) LoadAll(includeCompleted bool) ([]parser.Plan, error) {
	files, err := parser.GlobBugFiles(s.dir, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("glob bug files: %w", err)
	}

	plans := make([]parser.Plan, 0, len(files))
	for _, f := range files {
		if _, err := parser.MigrateLegacyBugIDs(f); err != nil {
			return nil, fmt.Errorf("migrate bug IDs in %s: %w", f, err)
		}
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, parser.Plan{
			ID:        planIDFromPath(f),
			MaggusID:  parser.ParseMaggusID(f),
			File:      f,
			Tasks:     tasks,
			IsBug:     true,
			Completed: strings.HasSuffix(f, "_completed.md"),
		})
	}
	return plans, nil
}

// MarkCompleted renames (or deletes) fully-completed bug files.
func (s *FileBugStore) MarkCompleted(action string) ([]string, error) {
	return parser.MarkCompletedBugs(s.dir, action)
}

// GlobFiles returns the raw file paths for all bug files.
func (s *FileBugStore) GlobFiles(includeCompleted bool) ([]string, error) {
	return parser.GlobBugFiles(s.dir, includeCompleted)
}

// DeleteTask removes a task section from the given file.
func (s *FileBugStore) DeleteTask(filePath, taskID string) error {
	return parser.DeleteTask(filePath, taskID)
}

// UnblockCriterion removes the BLOCKED: prefix from a criterion line.
func (s *FileBugStore) UnblockCriterion(filePath string, c parser.Criterion) error {
	return parser.UnblockCriterion(filePath, c)
}

// ResolveCriterion removes the BLOCKED: prefix and marks a criterion as checked.
func (s *FileBugStore) ResolveCriterion(filePath string, c parser.Criterion) error {
	return parser.ResolveCriterion(filePath, c)
}

// DeleteCriterion removes a criterion line from the given file.
func (s *FileBugStore) DeleteCriterion(filePath string, c parser.Criterion) error {
	return parser.DeleteCriterion(filePath, c)
}

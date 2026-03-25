package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Plan represents a single feature or bug file with its parsed tasks and metadata.
// Approval status is NOT stored here — callers look it up separately via approval.Load().
type Plan struct {
	ID        string  // base filename without extension or _completed suffix (e.g. "feature_001", "bug_002")
	MaggusID  string  // UUID from <!-- maggus-id: ... --> comment; empty if absent
	File      string  // full path to the source file
	Tasks     []Task  // all tasks from this file (may include complete/blocked)
	IsBug     bool    // true for bug files (from .maggus/bugs/)
	Completed bool    // true if the filename contains _completed suffix
}

// ApprovalKey returns the MaggusID if set, otherwise falls back to the filename-based ID.
// This matches the approval key logic used in status_plans.go and work_loop.go.
func (p *Plan) ApprovalKey() string {
	if p.MaggusID != "" {
		return p.MaggusID
	}
	return p.ID
}

// DoneCount returns the count of completed tasks.
func (p *Plan) DoneCount() int {
	n := 0
	for _, t := range p.Tasks {
		if t.IsComplete() {
			n++
		}
	}
	return n
}

// BlockedCount returns the count of incomplete tasks that are blocked.
func (p *Plan) BlockedCount() int {
	n := 0
	for _, t := range p.Tasks {
		if !t.IsComplete() && t.IsBlocked() {
			n++
		}
	}
	return n
}

// planIDFromPath extracts the Plan ID from a file path: the base filename with
// both the ".md" extension and the "_completed" suffix stripped.
// For example: ".maggus/features/feature_003_completed.md" → "feature_003"
func planIDFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".md")
	base = strings.TrimSuffix(base, "_completed")
	return base
}

// LoadPlans loads all feature and bug plan files from the given directory.
// Bugs are returned before features (matching the work loop priority order).
// If includeCompleted is false, files ending in _completed.md are excluded.
// For bug files, MigrateLegacyBugIDs is called before parsing.
func LoadPlans(dir string, includeCompleted bool) ([]Plan, error) {
	bugFiles, err := GlobBugFiles(dir, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("glob bugs: %w", err)
	}

	featureFiles, err := GlobFeatureFiles(dir, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("glob features: %w", err)
	}

	plans := make([]Plan, 0, len(bugFiles)+len(featureFiles))

	// Bugs first.
	for _, f := range bugFiles {
		if _, err := MigrateLegacyBugIDs(f); err != nil {
			return nil, fmt.Errorf("migrate bug IDs in %s: %w", f, err)
		}
		tasks, err := ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, Plan{
			ID:        planIDFromPath(f),
			MaggusID:  ParseMaggusID(f),
			File:      f,
			Tasks:     tasks,
			IsBug:     true,
			Completed: strings.HasSuffix(f, "_completed.md"),
		})
	}

	// Features next.
	for _, f := range featureFiles {
		tasks, err := ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		plans = append(plans, Plan{
			ID:        planIDFromPath(f),
			MaggusID:  ParseMaggusID(f),
			File:      f,
			Tasks:     tasks,
			IsBug:     false,
			Completed: strings.HasSuffix(f, "_completed.md"),
		})
	}

	return plans, nil
}

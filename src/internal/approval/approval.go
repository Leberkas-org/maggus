// Package approval manages feature approval state for the maggus work loop.
// Approval state is stored in .maggus/feature_approvals.yml as a map of feature
// ID to approved boolean. The behaviour of IsApproved depends on the configured
// approval mode: opt-in requires explicit approval; opt-out approves all features
// by default and only excludes those explicitly unapproved.
package approval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Approvals maps a feature ID (e.g. "feature_001") to its approved state.
type Approvals map[string]bool

// Load reads .maggus/feature_approvals.yml from dir.
// If the file does not exist, an empty Approvals map is returned with no error.
func Load(dir string) (Approvals, error) {
	path := filepath.Join(dir, ".maggus", "feature_approvals.yml")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Approvals{}, nil
		}
		return nil, fmt.Errorf("read feature_approvals.yml: %w", err)
	}

	var a Approvals
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parse feature_approvals.yml: %w", err)
	}
	if a == nil {
		a = Approvals{}
	}
	return a, nil
}

// Save writes the approval state to .maggus/feature_approvals.yml in dir.
func Save(dir string, a Approvals) error {
	path := filepath.Join(dir, ".maggus", "feature_approvals.yml")

	data, err := yaml.Marshal(map[string]bool(a))
	if err != nil {
		return fmt.Errorf("marshal feature_approvals.yml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write feature_approvals.yml: %w", err)
	}
	return nil
}

// Approve marks the given feature ID as approved and persists the change.
func Approve(dir, featureID string) error {
	a, err := Load(dir)
	if err != nil {
		return err
	}
	a[featureID] = true
	return Save(dir, a)
}

// Unapprove marks the given feature ID as unapproved and persists the change.
// This is meaningful in opt-in mode (removes approval) and opt-out mode (blocks execution).
func Unapprove(dir, featureID string) error {
	a, err := Load(dir)
	if err != nil {
		return err
	}
	a[featureID] = false
	return Save(dir, a)
}

// Remove deletes the entry for featureID from feature_approvals.yml in dir.
// If the entry does not exist, the function is a no-op and the file is not rewritten.
func Remove(dir, featureID string) error {
	a, err := Load(dir)
	if err != nil {
		return err
	}
	if _, ok := a[featureID]; !ok {
		return nil
	}
	delete(a, featureID)
	return Save(dir, a)
}

// Prune removes entries from feature_approvals.yml whose key is not in knownIDs.
// If knownIDs is empty, the function is a no-op to prevent accidentally wiping the file.
// If no entries are removed, the file is not rewritten.
func Prune(dir string, knownIDs []string) error {
	if len(knownIDs) == 0 {
		return nil
	}

	a, err := Load(dir)
	if err != nil {
		return err
	}

	known := make(map[string]struct{}, len(knownIDs))
	for _, id := range knownIDs {
		known[id] = struct{}{}
	}

	removed := 0
	for key := range a {
		if _, ok := known[key]; !ok {
			delete(a, key)
			removed++
		}
	}

	if removed == 0 {
		return nil
	}

	return Save(dir, a)
}

// IsApproved reports whether the given feature is approved for execution.
//
// When approvalRequired is true (opt-in mode), a feature must have been explicitly
// approved (a[featureID] == true) to return true.
//
// When approvalRequired is false (opt-out mode), all features are approved by
// default unless explicitly unapproved (a[featureID] == false).
func IsApproved(a Approvals, featureID string, approvalRequired bool) bool {
	if approvalRequired {
		// opt-in: must be explicitly approved
		return a[featureID]
	}
	// opt-out: approved unless explicitly set to false
	if approved, ok := a[featureID]; ok {
		return approved
	}
	return true
}

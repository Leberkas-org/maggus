package parser

import (
	"fmt"
	"os"
	"path/filepath"
)

// IsFeatureComplete reports whether the feature identified by featureNum (a
// zero-padded three-digit string such as "006") is complete. The check uses
// featureDir as the directory that holds feature_*.md files.
//
// Rules (checked in order):
//  1. If feature_NNN_completed.md exists → complete.
//  2. If feature_NNN.md exists → complete only when every task in the file has
//     all acceptance criteria checked (no unchecked criteria remain).
//  3. If neither file exists → treat as complete so dependent tasks are not
//     permanently blocked by a feature that was never created.
func IsFeatureComplete(featureDir, featureNum string) bool {
	completedPath := filepath.Join(featureDir, "feature_"+featureNum+"_completed.md")
	if _, err := os.Stat(completedPath); err == nil {
		return true
	}

	activePath := filepath.Join(featureDir, "feature_"+featureNum+".md")
	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		// Neither file exists — treat as complete.
		return true
	}

	tasks, err := ParseFile(activePath)
	if err != nil {
		// If parsing fails we cannot determine completion; treat as incomplete
		// to avoid accidentally unblocking a dependent task.
		return false
	}

	for i := range tasks {
		if !tasks[i].IsComplete() {
			return false
		}
	}
	return true
}

// CrossFeaturePredecessorsSatisfied reports whether every cross-feature
// predecessor for the task is satisfied. It calls IsFeatureComplete for each
// feature number in every CrossFeatureRef. Returns true when
// CrossFeaturePredecessors is nil or empty.
func (t *Task) CrossFeaturePredecessorsSatisfied(featureDir string) bool {
	for _, ref := range t.CrossFeaturePredecessors {
		for _, num := range ref.FeatureNums {
			if !IsFeatureComplete(featureDir, fmt.Sprintf("%03d", num)) {
				return false
			}
		}
	}
	return true
}

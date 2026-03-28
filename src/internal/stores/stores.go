// Package stores defines the FeatureStore and BugStore interfaces for plan file I/O.
package stores

import "github.com/leberkas-org/maggus/internal/parser"

// FeatureStore manages reading and mutating feature plan files.
type FeatureStore interface {
	LoadAll(includeCompleted bool) ([]parser.Plan, error)
	MarkCompleted(action string) ([]string, error)
	GlobFiles(includeCompleted bool) ([]string, error)
	DeleteTask(filePath, taskID string) error
	UnblockCriterion(filePath string, c parser.Criterion) error
	ResolveCriterion(filePath string, c parser.Criterion) error
	DeleteCriterion(filePath string, c parser.Criterion) error
}

// BugStore manages reading and mutating bug plan files.
type BugStore interface {
	LoadAll(includeCompleted bool) ([]parser.Plan, error)
	MarkCompleted(action string) ([]string, error)
	GlobFiles(includeCompleted bool) ([]string, error)
	DeleteTask(filePath, taskID string) error
	UnblockCriterion(filePath string, c parser.Criterion) error
	ResolveCriterion(filePath string, c parser.Criterion) error
	DeleteCriterion(filePath string, c parser.Criterion) error
}

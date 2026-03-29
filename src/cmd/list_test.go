package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/stores"
)

func TestRunList_TabSeparatedOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	featurePlans := []parser.Plan{
		{
			ID:    "feature_001",
			File:  filepath.Join(dir, ".maggus", "features", "feature_001.md"),
			Title: "My Feature",
		},
	}
	bugPlans := []parser.Plan{
		{
			ID:    "bug_001",
			File:  filepath.Join(dir, ".maggus", "bugs", "bug_001.md"),
			Title: "A Bug",
		},
	}

	featureStore := stores.NewMemFeatureStore(featurePlans)
	bugStore := stores.NewMemBugStore(bugPlans)

	// Approve the feature plan.
	if err := approval.Approve(dir, "feature_001"); err != nil {
		t.Fatal(err)
	}

	cmd, stdout, _ := newTestCmd(t)
	if err := runList(cmd, dir, featureStore, bugStore); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Bugs appear first (matching loadAllPlans ordering).
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), output)
	}

	// First line: bug
	bugFields := strings.Split(lines[0], "\t")
	if len(bugFields) != 4 {
		t.Fatalf("expected 4 tab-separated fields in bug line, got %d: %q", len(bugFields), lines[0])
	}
	if bugFields[0] != "bug_001.md" {
		t.Errorf("filename: want %q, got %q", "bug_001.md", bugFields[0])
	}
	if bugFields[1] != "bug_001" {
		t.Errorf("id: want %q, got %q", "bug_001", bugFields[1])
	}
	if bugFields[2] != "A Bug" {
		t.Errorf("title: want %q, got %q", "A Bug", bugFields[2])
	}
	if bugFields[3] != "unapproved" {
		t.Errorf("approved: want %q, got %q", "unapproved", bugFields[3])
	}

	// Second line: feature (approved)
	featureFields := strings.Split(lines[1], "\t")
	if len(featureFields) != 4 {
		t.Fatalf("expected 4 tab-separated fields in feature line, got %d: %q", len(featureFields), lines[1])
	}
	if featureFields[0] != "feature_001.md" {
		t.Errorf("filename: want %q, got %q", "feature_001.md", featureFields[0])
	}
	if featureFields[1] != "feature_001" {
		t.Errorf("id: want %q, got %q", "feature_001", featureFields[1])
	}
	if featureFields[2] != "My Feature" {
		t.Errorf("title: want %q, got %q", "My Feature", featureFields[2])
	}
	if featureFields[3] != "approved" {
		t.Errorf("approved: want %q, got %q", "approved", featureFields[3])
	}
}

func TestRunList_NoPlans(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	featureStore := stores.NewMemFeatureStore(nil)
	bugStore := stores.NewMemBugStore(nil)

	cmd, stdout, _ := newTestCmd(t)
	if err := runList(cmd, dir, featureStore, bugStore); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("expected empty output for no plans, got: %q", stdout.String())
	}
}

func TestRunList_EmptyTitle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	featurePlans := []parser.Plan{
		{
			ID:    "feature_002",
			File:  filepath.Join(dir, ".maggus", "features", "feature_002.md"),
			Title: "", // no title
		},
	}

	featureStore := stores.NewMemFeatureStore(featurePlans)
	bugStore := stores.NewMemBugStore(nil)

	cmd, stdout, _ := newTestCmd(t)
	if err := runList(cmd, dir, featureStore, bugStore); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	line := strings.TrimRight(stdout.String(), "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 4 {
		t.Fatalf("expected 4 fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "" {
		t.Errorf("title: want empty string, got %q", fields[2])
	}
}

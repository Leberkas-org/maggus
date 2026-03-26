package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func newTestTask() *parser.Task {
	return &parser.Task{
		ID:          "TASK-042",
		Title:       "Implement the thing",
		Description: "As a dev, I want the thing so it works.",
		SourceFile:  ".maggus/features/feature_002.md",
		Criteria: []parser.Criterion{
			{Text: "First criterion", Checked: false},
			{Text: "Second criterion", Checked: true},
		},
	}
}

func newTestOpts() Options {
	return Options{
		RunID:     "20260309-120000",
		Iteration: 3,
	}
}

func TestBuild_ContainsAllSections(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	result := Build(task, opts)

	checks := []string{
		// Bootstrap
		"# Bootstrap",
		"CLAUDE.md",
		"AGENTS.md",
		"PROJECT_CONTEXT.md",
		"TOOLING.md",

		// Run metadata
		"# Run Metadata",
		"RUN_ID:** 20260309-120000",
		"ITERATION:** 3",

		// Task
		"TASK-042",
		"Implement the thing",
		"As a dev, I want the thing so it works.",
		"- [ ] First criterion",
		"- [x] Second criterion",

		// Instructions
		"Work ONLY on TASK-042: Implement the thing",
		"Do NOT scan feature files to find a different task",
		"verify that every acceptance criterion",
		"Stage all changed files",
		"do NOT commit",
		"COMMIT.md",
		"Update the feature file",
		".maggus/features/feature_002.md",
	}

	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("prompt missing %q\n\nGot:\n%s", want, result)
		}
	}
}

func TestBuild_NoBootstrap_OmitsBootstrapSection(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	opts.NoBootstrap = true
	result := Build(task, opts)

	// Should NOT contain bootstrap section
	bootstrapMarkers := []string{
		"# Bootstrap",
		"- CLAUDE.md",
		"- AGENTS.md",
		"- PROJECT_CONTEXT.md",
		"- TOOLING.md",
	}

	for _, marker := range bootstrapMarkers {
		if strings.Contains(result, marker) {
			t.Errorf("--no-bootstrap prompt should not contain %q\n\nGot:\n%s", marker, result)
		}
	}

	// Should still contain everything else
	requiredSections := []string{
		"# Run Metadata",
		"TASK-042",
		"Work ONLY on TASK-042",
		"COMMIT.md",
	}

	for _, want := range requiredSections {
		if !strings.Contains(result, want) {
			t.Errorf("--no-bootstrap prompt missing required section %q\n\nGot:\n%s", want, result)
		}
	}
}

func TestBuild_EmptyIncludes_StandardBootstrapOnly(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	// opts.Include is nil (empty)
	result := Build(task, opts)

	// Standard bootstrap files should be present
	for _, f := range []string{"CLAUDE.md", "AGENTS.md", "PROJECT_CONTEXT.md", "TOOLING.md"} {
		if !strings.Contains(result, f) {
			t.Errorf("expected standard bootstrap file %q in prompt", f)
		}
	}

	// No "Read the file" instructions should appear
	if strings.Contains(result, "Read the file") {
		t.Errorf("empty includes should not produce 'Read the file' instructions\n\nGot:\n%s", result)
	}
}

func TestBuild_Includes_AddsReadInstructions(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	opts.Include = []string{"ARCHITECTURE.md", "docs/PATTERNS.md"}
	result := Build(task, opts)

	// Standard bootstrap files should still be present
	for _, f := range []string{"CLAUDE.md", "AGENTS.md", "PROJECT_CONTEXT.md", "TOOLING.md"} {
		if !strings.Contains(result, f) {
			t.Errorf("expected standard bootstrap file %q in prompt", f)
		}
	}

	// Custom includes should appear as "Read the file" instructions
	for _, inc := range opts.Include {
		want := fmt.Sprintf("Read the file `%s` if it exists in the working directory.", inc)
		if !strings.Contains(result, want) {
			t.Errorf("expected include instruction %q in prompt\n\nGot:\n%s", want, result)
		}
	}

	// Custom includes should appear after standard bootstrap section
	bootstrapIdx := strings.Index(result, "TOOLING.md")
	archIdx := strings.Index(result, "ARCHITECTURE.md")
	if archIdx <= bootstrapIdx {
		t.Errorf("custom includes should appear after standard bootstrap files")
	}
}

func TestBuild_ContainsReleaseNotesInstruction(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	result := Build(task, opts)

	releaseNotesChecks := []string{
		"`.maggus/RELEASE_NOTES.md`",
		"## TASK-NNN: Title",
		"1-3 bullet points",
		"user's perspective",
		"Do NOT commit this file",
	}

	for _, want := range releaseNotesChecks {
		if !strings.Contains(result, want) {
			t.Errorf("prompt missing release notes instruction %q\n\nGot:\n%s", want, result)
		}
	}

	// Verify it appears as step 5 (after step 4 about MEMORY.md)
	memoryIdx := strings.Index(result, "4. Create or update `.maggus/MEMORY.md`")
	releaseIdx := strings.Index(result, "5. Append a short release note entry")
	if memoryIdx == -1 {
		t.Fatal("step 4 (MEMORY.md) not found")
	}
	if releaseIdx == -1 {
		t.Fatal("step 5 (RELEASE_NOTES.md) not found")
	}
	if releaseIdx <= memoryIdx {
		t.Error("step 5 (release notes) should appear after step 4 (MEMORY.md)")
	}
}

func TestBuild_BugSourceFile_ReferencedCorrectly(t *testing.T) {
	task := &parser.Task{
		ID:          "BUG-001-001",
		Title:       "Fix the crash",
		Description: "The app crashes on startup.",
		SourceFile:  ".maggus/bugs/bug_1.md",
		Criteria: []parser.Criterion{
			{Text: "Fix the crash", Checked: false},
		},
	}
	opts := newTestOpts()
	result := Build(task, opts)

	// The prompt should reference the bug source file, not a feature file.
	if !strings.Contains(result, ".maggus/bugs/bug_1.md") {
		t.Errorf("prompt should reference bug source file .maggus/bugs/bug_1.md\n\nGot:\n%s", result)
	}
	if !strings.Contains(result, "Update the feature file (`.maggus/bugs/bug_1.md`)") {
		t.Errorf("prompt should instruct updating the bug source file\n\nGot:\n%s", result)
	}
}

func TestBuild_SectionOrder(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	result := Build(task, opts)

	// Verify sections appear in correct order
	sections := []string{
		"# Bootstrap",
		"# Run Metadata",
		"# Task",
		"# Instructions",
	}

	lastIdx := -1
	for _, section := range sections {
		idx := strings.Index(result, section)
		if idx == -1 {
			t.Errorf("section %q not found", section)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("section %q appears before previous section (idx %d <= %d)", section, idx, lastIdx)
		}
		lastIdx = idx
	}
}

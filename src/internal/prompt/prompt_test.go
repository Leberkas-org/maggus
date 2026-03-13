package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dirnei/maggus/internal/parser"
)

func newTestTask() *parser.Task {
	return &parser.Task{
		ID:          "TASK-042",
		Title:       "Implement the thing",
		Description: "As a dev, I want the thing so it works.",
		SourceFile:  ".maggus/plan_2.md",
		Criteria: []parser.Criterion{
			{Text: "First criterion", Checked: false},
			{Text: "Second criterion", Checked: true},
		},
	}
}

func newTestOpts() Options {
	return Options{
		RunID:     "20260309-120000",
		RunDir:    ".maggus/runs/20260309-120000",
		Iteration: 3,
		IterLog:   ".maggus/runs/20260309-120000/iteration-03.md",
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
		"RUN_DIR:** .maggus/runs/20260309-120000",
		"ITERATION:** 3",
		"ITER_LOG:** .maggus/runs/20260309-120000/iteration-03.md",

		// Task
		"TASK-042",
		"Implement the thing",
		"As a dev, I want the thing so it works.",
		"- [ ] First criterion",
		"- [x] Second criterion",

		// Instructions
		"Work ONLY on TASK-042: Implement the thing",
		"Do NOT scan plan files to find a different task",
		"verify that every acceptance criterion",
		"Stage all changed files",
		"do NOT commit",
		"COMMIT.md",
		"Update the plan file",
		".maggus/plan_2.md",
		"Write an iteration log",
		"iteration-03.md",
		"Task selected",
		"Commands run",
		"deviations or skips",
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
		"CLAUDE.md",
		"AGENTS.md",
		"PROJECT_CONTEXT.md",
		"TOOLING.md",
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

func TestBuild_Worktree_AddsMetadataAndInstructions(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	opts.Worktree = true
	opts.WorktreeDir = ".maggus-work/20260309-120000"
	result := Build(task, opts)

	worktreeChecks := []string{
		"**WORKTREE:** true",
		"**WORKTREE_DIR:** .maggus-work/20260309-120000",
		"WORKTREE MODE:",
		"Other Maggus sessions may be running concurrently",
		"Do not make assumptions about branch state outside your own branch",
		"Do not modify or switch branches",
	}

	for _, want := range worktreeChecks {
		if !strings.Contains(result, want) {
			t.Errorf("worktree prompt missing %q\n\nGot:\n%s", want, result)
		}
	}

	// Worktree metadata should appear in the Run Metadata section
	metaIdx := strings.Index(result, "# Run Metadata")
	taskIdx := strings.Index(result, "# Task")
	wtIdx := strings.Index(result, "**WORKTREE:** true")
	if wtIdx <= metaIdx || wtIdx >= taskIdx {
		t.Errorf("WORKTREE metadata should appear between Run Metadata and Task sections")
	}
}

func TestBuild_NoWorktree_OmitsWorktreeFields(t *testing.T) {
	task := newTestTask()
	opts := newTestOpts()
	// Worktree is false by default
	result := Build(task, opts)

	worktreeMarkers := []string{
		"WORKTREE:",
		"WORKTREE_DIR:",
		"WORKTREE MODE:",
		"Other Maggus sessions may be running concurrently",
	}

	for _, marker := range worktreeMarkers {
		if strings.Contains(result, marker) {
			t.Errorf("non-worktree prompt should not contain %q\n\nGot:\n%s", marker, result)
		}
	}

	// All standard sections should still be present
	for _, want := range []string{"# Bootstrap", "# Run Metadata", "# Task", "# Instructions"} {
		if !strings.Contains(result, want) {
			t.Errorf("non-worktree prompt missing %q", want)
		}
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

	// Verify it appears as step 6 (after step 5 about MEMORY.md)
	memoryIdx := strings.Index(result, "5. Create or update `.maggus/MEMORY.md`")
	releaseIdx := strings.Index(result, "6. Append a short release note entry")
	if memoryIdx == -1 {
		t.Fatal("step 5 (MEMORY.md) not found")
	}
	if releaseIdx == -1 {
		t.Fatal("step 6 (RELEASE_NOTES.md) not found")
	}
	if releaseIdx <= memoryIdx {
		t.Error("step 6 (release notes) should appear after step 5 (MEMORY.md)")
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

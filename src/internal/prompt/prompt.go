package prompt

import (
	"fmt"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
)

// Options controls what sections are included in the prompt.
type Options struct {
	// NoBootstrap disables reading CLAUDE.md/AGENTS.md/PROJECT_CONTEXT.md/TOOLING.md.
	NoBootstrap bool

	// Include lists additional markdown files to read as part of the bootstrap context.
	// Paths are relative to the project root (e.g. "ARCHITECTURE.md", "docs/PATTERNS.md").
	Include []string

	// Run metadata
	RunID     string
	Iteration int

	// Worktree indicates this session is running inside a git worktree.
	Worktree bool

	// WorktreeDir is the worktree directory relative to the repo root (e.g. ".maggus-work/<run-id>").
	WorktreeDir string
}

// Build creates a focused prompt for Claude Code to work on a single task.
func Build(task *parser.Task, opts Options) string {
	var b strings.Builder

	// --- Bootstrap section ---
	if !opts.NoBootstrap {
		writeBootstrap(&b, opts.Include)
	}

	// --- Run metadata ---
	writeRunMetadata(&b, opts)

	// --- Task section ---
	writeTask(&b, task)

	// --- Instructions ---
	writeInstructions(&b, task, opts)

	return b.String()
}

func writeBootstrap(b *strings.Builder, includes []string) {
	b.WriteString("# Bootstrap\n\n")
	b.WriteString("Before starting work, read the following files if they exist in the working directory:\n")
	b.WriteString("- CLAUDE.md\n")
	b.WriteString("- AGENTS.md\n")
	b.WriteString("- PROJECT_CONTEXT.md\n")
	b.WriteString("- TOOLING.md\n")
	b.WriteString("- .maggus/MEMORY.md\n")
	b.WriteString("\nThese files contain project conventions, architecture context, and tooling instructions. Follow them.\n\n")

	for _, path := range includes {
		fmt.Fprintf(b, "Read the file `%s` if it exists in the working directory.\n", path)
	}
	if len(includes) > 0 {
		b.WriteString("\n")
	}
}

func writeRunMetadata(b *strings.Builder, opts Options) {
	b.WriteString("# Run Metadata\n\n")
	fmt.Fprintf(b, "- **RUN_ID:** %s\n", opts.RunID)
	fmt.Fprintf(b, "- **ITERATION:** %d\n", opts.Iteration)
	if opts.Worktree {
		b.WriteString("- **WORKTREE:** true\n")
		fmt.Fprintf(b, "- **WORKTREE_DIR:** %s\n", opts.WorktreeDir)
	}
	b.WriteString("\n")
}

func writeTask(b *strings.Builder, task *parser.Task) {
	fmt.Fprintf(b, "# Task\n\n")
	fmt.Fprintf(b, "## %s: %s\n\n", task.ID, task.Title)
	fmt.Fprintf(b, "**Description:** %s\n\n", task.Description)
	fmt.Fprintf(b, "**Acceptance Criteria:**\n")
	for _, c := range task.Criteria {
		if c.Checked {
			fmt.Fprintf(b, "- [x] %s\n", c.Text)
		} else {
			fmt.Fprintf(b, "- [ ] %s\n", c.Text)
		}
	}
	b.WriteString("\n")
}

func writeInstructions(b *strings.Builder, task *parser.Task, opts Options) {
	b.WriteString("# Instructions\n\n")
	if opts.Worktree {
		b.WriteString("**WORKTREE MODE:** You are running inside a git worktree. Other Maggus sessions may be running concurrently in separate worktrees. ")
		b.WriteString("Do not make assumptions about branch state outside your own branch. ")
		b.WriteString("Do not modify or switch branches — stay on your current branch.\n\n")
	}
	fmt.Fprintf(b, "IMPORTANT: The task has already been selected for you. Work ONLY on %s: %s.\n", task.ID, task.Title)
	b.WriteString("Do NOT scan feature files to find a different task. Do NOT work on any other task.\n\n")
	b.WriteString("Before finishing, verify that every acceptance criterion above is met. Do not work on anything outside this task.\n\n")
	b.WriteString("If a criterion is TRULY impossible to complete — a required external tool or API is unavailable, ")
	b.WriteString("it requires a human decision you cannot make, or it would require a complete architectural redesign — mark it as:\n")
	b.WriteString("  `- [~] ⚠️ BLOCKED: <original criterion text> — <reason>`\n")
	b.WriteString("Do NOT block a criterion just because it is difficult or you hit an error. ")
	b.WriteString("Try to fix problems yourself first. Only block as an absolute last resort.\n\n")

	// Stage files but do NOT commit
	b.WriteString("When you are done:\n")

	// Update feature checkboxes
	fmt.Fprintf(b, "1. Update the feature file (`%s`) checkboxes: mark completed acceptance criteria as `[x]`.\n", task.SourceFile)

	b.WriteString("2. Stage all changed files with `git add *` but do NOT commit.\n")
	b.WriteString("3. Write a commit message to `COMMIT.md` in the repository root. Include the task ID in the message.\n")

	// Update project memory
	b.WriteString("4. Create or update `.maggus/MEMORY.md` only if something non-obvious was learned during this task. ")
	b.WriteString("This file is a reference for future sessions — keep it architectural, not historical. ")
	b.WriteString("ONLY add entries for: non-obvious platform quirks, gotchas discovered during implementation, ")
	b.WriteString("important constraints or invariants that are not evident from reading the code, ")
	b.WriteString("and cross-cutting architectural decisions with a non-obvious rationale. ")
	b.WriteString("Do NOT record: completed task summaries, what files were changed, what tests were added, ")
	b.WriteString("implementation details that are visible in the code, or anything already in CLAUDE.md. ")
	b.WriteString("If nothing non-obvious was learned, skip this step entirely. Do NOT commit this file.\n")

	// Append release notes
	b.WriteString("5. Append a short release note entry to `.maggus/RELEASE_NOTES.md` describing user-visible changes made in this task. ")
	b.WriteString("Use the format: `## TASK-NNN: Title` followed by 1-3 bullet points. ")
	b.WriteString("Focus on what changed from the user's perspective, not implementation details. ")
	b.WriteString("If the task has no user-visible changes, skip this step. Do NOT commit this file.\n")
}

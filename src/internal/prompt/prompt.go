package prompt

import (
	"fmt"
	"strings"

	"github.com/dirnei/maggus/internal/parser"
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
	RunDir    string
	Iteration int
	IterLog   string

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
	fmt.Fprintf(b, "- **RUN_DIR:** %s\n", opts.RunDir)
	fmt.Fprintf(b, "- **ITERATION:** %d\n", opts.Iteration)
	fmt.Fprintf(b, "- **ITER_LOG:** %s\n", opts.IterLog)
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
	b.WriteString("Do NOT scan plan files to find a different task. Do NOT work on any other task.\n\n")
	b.WriteString("Before finishing, verify that every acceptance criterion above is met. Do not work on anything outside this task.\n\n")
	b.WriteString("If a criterion cannot be completed (missing dependency, needs human input, external blocker), mark it as:\n")
	b.WriteString("  `- [x] ⚠️ BLOCKED: <original criterion text> — <reason>`\n")
	b.WriteString("This tells Maggus to skip this task in future runs.\n\n")

	// Stage files but do NOT commit
	b.WriteString("When you are done:\n")

	// Update plan checkboxes
	fmt.Fprintf(b, "1. Update the plan file (`%s`) checkboxes: mark completed acceptance criteria as `[x]`.\n", task.SourceFile)

	b.WriteString("2. Stage all changed files with `git add *` but do NOT commit.\n")
	b.WriteString("3. Write a commit message to `COMMIT.md` in the repository root. Include the task ID in the message.\n")

	// Write iteration log
	fmt.Fprintf(b, "4. Write an iteration log to `%s` before finishing. The log must include:\n", opts.IterLog)
	b.WriteString("   - Task selected (ID and title)\n")
	b.WriteString("   - Commands run and their outcomes\n")
	b.WriteString("   - Any deviations or skips from the acceptance criteria\n")

	// Update project memory
	b.WriteString("5. Create or update `.maggus/MEMORY.md` with any project knowledge gained during this task. ")
	b.WriteString("This file serves as a portable project memory for consistency across machines. ")
	b.WriteString("Include: project structure changes, build/tooling changes, new conventions, ")
	b.WriteString("architectural decisions, and important file paths. ")
	b.WriteString("Keep it concise and organized by topic. Do NOT commit this file.\n")
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/dirnei/maggus/internal/config"
	"github.com/dirnei/maggus/internal/release"
	"github.com/dirnei/maggus/internal/runner"
	"github.com/spf13/cobra"
)

var releaseModelFlag string

// runClaudeOnce is a package-level variable so tests can replace it.
var runClaudeOnce = runner.RunOnce

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Generate RELEASE.md with changelog and AI summary",
	Long: `Generates a RELEASE.md file combining a structured conventional changelog
with an AI-generated summary of all changes since the last version tag.

If .maggus/RELEASE_NOTES.md exists (rough notes accumulated during work iterations),
it is included as additional context for the AI summary.

Examples:
  maggus release              # generate release notes using default model
  maggus release --model opus # use a specific model`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runRelease(cmd, dir)
	},
}

func runRelease(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	// Resolve model
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	modelInput := cfg.Model
	if releaseModelFlag != "" {
		modelInput = releaseModelFlag
	}
	resolvedModel := config.ResolveModel(modelInput)

	// Find last tag and commits
	tag, err := release.FindLastTag(dir)
	if err != nil {
		return fmt.Errorf("find last tag: %w", err)
	}

	commits, err := release.CommitsSinceTag(dir, tag)
	if err != nil {
		return fmt.Errorf("get commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Fprintln(out, "No changes since last tag.")
		return nil
	}

	// Generate conventional changelog
	groups := release.GroupByType(commits)
	changelog := release.FormatChangelog(groups, tag)

	// Read rough release notes if they exist
	releaseNotes := ""
	notesPath := dir + "/.maggus/RELEASE_NOTES.md"
	if data, err := os.ReadFile(notesPath); err == nil {
		releaseNotes = strings.TrimSpace(string(data))
	}

	// Get diff stat
	diffStat := getDiffStat(dir, tag)

	// Build prompt for Claude
	prompt := buildReleasePrompt(changelog, releaseNotes, diffStat)

	// Invoke Claude for AI summary
	fmt.Fprintln(out, "Generating AI summary...")

	ctx, cancel := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer cancel()

	summary, err := runClaudeOnce(ctx, prompt, resolvedModel)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	// Build final RELEASE.md
	releaseMd := buildReleaseMd(summary, changelog)

	// Write RELEASE.md
	releasePath := dir + "/RELEASE.md"
	if err := os.WriteFile(releasePath, []byte(releaseMd), 0o644); err != nil {
		return fmt.Errorf("write RELEASE.md: %w", err)
	}

	fmt.Fprintf(out, "Wrote %s\n\n", releasePath)
	fmt.Fprintln(out, "## Summary")
	fmt.Fprintln(out, summary)

	// Clear release notes for next cycle
	if err := os.Remove(notesPath); err == nil {
		fmt.Fprintln(out, "\nCleared .maggus/RELEASE_NOTES.md for next release cycle.")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("clear release notes: %w", err)
	}

	return nil
}

func getDiffStat(dir, tag string) string {
	var args []string
	if tag != "" {
		args = []string{"diff", tag + "..HEAD", "--stat"}
	} else {
		args = []string{"diff", "--stat", "HEAD"}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func buildReleasePrompt(changelog, releaseNotes, diffStat string) string {
	var sb strings.Builder

	sb.WriteString("You are generating release notes for a software project. ")
	sb.WriteString("Based on the information below, produce TWO sections:\n\n")
	sb.WriteString("1. **Summary** — A short highlights/summary section (3-8 bullet points) written for end users. ")
	sb.WriteString("Focus on what changed from the user's perspective, not implementation details. ")
	sb.WriteString("Use clear, concise language.\n\n")
	sb.WriteString("2. **Breaking Changes** — If there are any breaking changes or migration notes, list them. ")
	sb.WriteString("If there are none, omit this section entirely.\n\n")
	sb.WriteString("Output ONLY the summary text (and breaking changes if any). Do NOT include markdown headers — I will add those myself. ")
	sb.WriteString("Do NOT include the changelog — I already have it.\n\n")
	sb.WriteString("---\n\n")

	sb.WriteString("## Conventional Changelog\n\n")
	sb.WriteString(changelog)
	sb.WriteString("\n")

	if releaseNotes != "" {
		sb.WriteString("---\n\n")
		sb.WriteString("## Rough Release Notes (accumulated during development)\n\n")
		sb.WriteString(releaseNotes)
		sb.WriteString("\n")
	}

	if diffStat != "" {
		sb.WriteString("---\n\n")
		sb.WriteString("## Diff Summary (git diff --stat)\n\n")
		sb.WriteString("```\n")
		sb.WriteString(diffStat)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

func buildReleaseMd(summary, changelog string) string {
	var sb strings.Builder

	sb.WriteString("# Release Notes\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(summary)
	sb.WriteString("\n\n")
	sb.WriteString("## Changelog\n\n")
	sb.WriteString(changelog)
	sb.WriteString("\n")

	return sb.String()
}

func init() {
	releaseCmd.Flags().StringVar(&releaseModelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
	rootCmd.AddCommand(releaseCmd)
}

package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/spf13/cobra"
)

var promptModelFlag string

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Launch an interactive Claude Code session with usage tracking",
	Long: `Opens a TUI picker to select a prompt mode (plain or skill), then
launches Claude Code interactively. Usage data is extracted after the
session ends.`,
	RunE: runPrompt,
}

func init() {
	promptCmd.Flags().StringVar(&promptModelFlag, "model", "", "model to use (e.g. opus, sonnet, haiku, or a full model ID)")
}

// skillMapping maps picker labels to their Claude skill command and usage file.
type skillMapping struct {
	skill     string // e.g. "/maggus-plan"; empty for Plain
	usageFile string // e.g. "usage_plan.jsonl"
}

var skillMappings = map[string]skillMapping{
	"open console":          {skill: "", usageFile: "usage_prompt.jsonl"},
	"/maggus-plan":          {skill: "/maggus-plan", usageFile: "usage_plan.jsonl"},
	"/maggus-vision":        {skill: "/maggus-vision", usageFile: "usage_vision.jsonl"},
	"/maggus-architecture":  {skill: "/maggus-architecture", usageFile: "usage_architecture.jsonl"},
	"/maggus-bugreport":     {skill: "/maggus-bugreport", usageFile: "usage_bugreport.jsonl"},
	"/bryan-plan":           {skill: "/bryan-plan", usageFile: "usage_bryan_plan.jsonl"},
	"/bryan-bugreport":      {skill: "/bryan-bugreport", usageFile: "usage_bryan_bugreport.jsonl"},
}

func runPrompt(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Load config for default model.
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Resolve model: CLI flag overrides config file.
	modelInput := cfg.Model
	if promptModelFlag != "" {
		modelInput = promptModelFlag
	}
	resolvedModel := config.ResolveModel(modelInput)

	// Show the prompt picker TUI.
	picker := newPromptPickerModel()
	p := tea.NewProgram(picker, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("prompt picker: %w", err)
	}

	result := finalModel.(promptPickerModel).result
	if result.Cancelled {
		return nil
	}

	mapping, ok := skillMappings[result.Skill]
	if !ok {
		return fmt.Errorf("unknown skill: %s", result.Skill)
	}

	agentName := cfg.Agent
	if agentName == "" {
		agentName = "claude"
	}

	// Ensure the maggus plugin is installed for non-plain skills.
	if mapping.skill != "" && agentName == "claude" {
		if err := ensureMaggusPlugin(); err != nil {
			return err
		}
	}

	// Build the prompt string: for skills it's "/skill-name description".
	var prompt string
	if mapping.skill != "" {
		prompt = mapping.skill
		if result.Description != "" {
			prompt += " " + result.Description
		}
	}

	info, err := launchInteractive(agentName, prompt, dir, result.SkipPermissions, resolvedModel)

	// Extract usage.
	if mapping.usageFile != "" && info != nil {
		extractSkillUsage(dir, resolvedModel, agentName, mapping.usageFile, info)
	}

	return err
}

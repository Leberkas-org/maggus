package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/discord"
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

// skillVerbMapping maps picker labels to Discord presence verbs.
var skillVerbMapping = map[string]string{
	"open console":         "Consulting",
	"/maggus-plan":         "Planning",
	"/maggus-vision":       "Visioning",
	"/maggus-architecture": "Architecting",
	"/maggus-bugreport":    "Reporting Bug",
	"/bryan-plan":          "Planning",
	"/bryan-bugreport":     "Reporting Bug",
}

var skillMappings = map[string]skillMapping{
	"open console":         {skill: "", usageFile: "usage_prompt.jsonl"},
	"/maggus-plan":         {skill: "/maggus-plan", usageFile: "usage_plan.jsonl"},
	"/maggus-vision":       {skill: "/maggus-vision", usageFile: "usage_vision.jsonl"},
	"/maggus-architecture": {skill: "/maggus-architecture", usageFile: "usage_architecture.jsonl"},
	"/maggus-bugreport":    {skill: "/maggus-bugreport", usageFile: "usage_bugreport.jsonl"},
	"/bryan-plan":          {skill: "/bryan-plan", usageFile: "usage_bryan_plan.jsonl"},
	"/bryan-bugreport":     {skill: "/bryan-bugreport", usageFile: "usage_bryan_bugreport.jsonl"},
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

	// Use shared presence from root menu if available; otherwise create our own.
	presence := sharedPresence
	ownPresence := false
	if presence == nil && cfg.DiscordPresence {
		presence = &discord.Presence{}
		_ = presence.Connect()
		ownPresence = true
	}
	defer func() {
		if ownPresence && presence != nil {
			_ = presence.Close()
		}
	}()

	// Update presence with the selected skill's verb.
	if presence != nil {
		verb := skillVerbMapping[result.Skill]
		details := result.Skill
		if result.Skill == "open console" {
			details = "Open Console"
		}
		_ = presence.Update(discord.PresenceState{
			FeatureTitle: details,
			Verb:         verb,
			StartTime:    time.Now(),
		})
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

	// Build the prompt string: just the skill name.
	var prompt string
	if mapping.skill != "" {
		prompt = mapping.skill
	}

	info, err := launchInteractive(agentName, prompt, dir, result.SkipPermissions, resolvedModel)

	// Extract usage.
	if mapping.usageFile != "" && info != nil {
		extractSkillUsage(dir, resolvedModel, agentName, mapping.usageFile, info)
	}

	return err
}

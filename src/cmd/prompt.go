package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/usage"
)

// skillMapping maps picker labels to their Claude skill command and usage kind.
type skillMapping struct {
	skill  string // e.g. "/maggus-plan"; empty for Plain
	kind   string // usage kind: "prompt", "plan", "vision", etc.
	title  string
	detail string
}

var skillMappings = map[string]skillMapping{
	"open console":         {skill: "", kind: "prompt", title: "Consulting AI", detail: "Manual Prompting"},
	"/maggus-plan":         {skill: "/maggus-plan", kind: "plan", title: "Planning", detail: "Manual Prompting"},
	"/maggus-vision":       {skill: "/maggus-vision", kind: "vision", title: "Defining a vision", detail: "Manual Prompting"},
	"/maggus-architecture": {skill: "/maggus-architecture", kind: "architecture", title: "Architecture", detail: "Manual Prompting"},
	"/maggus-bugreport":    {skill: "/maggus-bugreport", kind: "bugreport", title: "Creating bug ticket", detail: "Manual Prompting"},
	"/bryan-plan":          {skill: "/bryan-plan", kind: "bryan_plan", title: "Planning with bryan", detail: "Manual Prompting"},
	"/bryan-bugreport":     {skill: "/bryan-bugreport", kind: "bryan_bugreport", title: "Reporting bug to bryan", detail: "Manual Prompting"},
}

func runPrompt() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Migrate any legacy per-project usage data to the global store (once, at startup).
	_ = usage.MigrateProject(dir)

	// Load config for default model.
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	resolvedModel := config.ResolveModel(cfg.Model)

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
	gs, _ := globalconfig.LoadSettings()
	if presence == nil && gs.DiscordPresence {
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
		verb := skillMappings[result.Skill]

		_ = presence.Update(discord.PresenceState{
			FeatureTitle: verb.title,
			Verb:         verb.detail,
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
	if mapping.kind != "" && info != nil {
		extractSkillUsage(dir, resolvedModel, agentName, mapping.kind, info)
	}

	return err
}

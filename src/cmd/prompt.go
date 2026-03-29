package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/config"
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
	agentName := cfg.Agent
	if agentName == "" {
		agentName = "claude"
	}

	pm := newPromptPickerModel(dir, resolvedModel, agentName)
	app := appModel{
		active: screenPrompt,
		prompt: &pm,
	}
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

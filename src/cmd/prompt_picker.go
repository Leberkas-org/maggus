package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// promptPickerFocus tracks which UI element has focus.
type promptPickerFocus int

const (
	focusSkillList promptPickerFocus = iota
	focusToggle
)

// skillOption represents a selectable skill in the picker.
type skillOption struct {
	label     string
	separator bool // true for non-selectable group separator rows
}

var defaultSkills = []skillOption{
	{label: "open console"},
	{separator: true, label: "maggus"},
	{label: "/maggus-plan"},
	{label: "/maggus-vision"},
	{label: "/maggus-architecture"},
	{label: "/maggus-bugreport"},
	{separator: true, label: "bryan"},
	{label: "/bryan-plan"},
	{label: "/bryan-bugreport"},
}

type promptPickerModel struct {
	skills          []skillOption
	skillCursor     int
	focus           promptPickerFocus
	skipPermissions bool
	width           int
	height          int

	// Config needed to build the interactive command on selection.
	dir           string
	resolvedModel string
	agentName     string
}

func newPromptPickerModel(dir, resolvedModel, agentName string) promptPickerModel {
	return promptPickerModel{
		skills:          defaultSkills,
		skipPermissions: true,
		dir:             dir,
		resolvedModel:   resolvedModel,
		agentName:       agentName,
	}
}

func (m promptPickerModel) Init() tea.Cmd {
	return nil
}

func (m promptPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "q", "esc":
			return m, func() tea.Msg { return navigateBackMsg{} }

		case "up", "k":
			if m.focus == focusSkillList {
				if m.skillCursor > 0 {
					m.skillCursor--
					m.skipSeparatorUp()
				}
			} else if m.focus == focusToggle {
				m.focus = focusSkillList
			}

		case "down", "j":
			if m.focus == focusSkillList {
				if m.skillCursor < len(m.skills)-1 {
					m.skillCursor++
					m.skipSeparatorDown()
				} else {
					m.focus = focusToggle
				}
			}

		case "tab":
			switch m.focus {
			case focusSkillList:
				m.focus = focusToggle
			case focusToggle:
				m.focus = focusSkillList
			}

		case "shift+tab":
			switch m.focus {
			case focusSkillList:
				m.focus = focusToggle
			case focusToggle:
				m.focus = focusSkillList
			}

		case "left", "right":
			if m.focus == focusToggle {
				m.skipPermissions = !m.skipPermissions
			}

		case "enter":
			if m.focus == focusSkillList {
				label := m.skills[m.skillCursor].label
				dir := m.dir
				resolvedModel := m.resolvedModel
				agentName := m.agentName
				skipPermissions := m.skipPermissions
				return m, m.buildLaunchCmd(label, dir, resolvedModel, agentName, skipPermissions)
			} else if m.focus == focusToggle {
				m.skipPermissions = !m.skipPermissions
			}

		case " ":
			if m.focus == focusToggle {
				m.skipPermissions = !m.skipPermissions
			}
		}
	}

	return m, nil
}

// buildLaunchCmd returns a tea.Cmd that prepares and emits an execProcessMsg for
// the selected skill. All pre-process work (presence update, plugin ensure, session
// snapshot) is done inside the cmd function so it runs off the main goroutine.
func (m promptPickerModel) buildLaunchCmd(label, dir, resolvedModel, agentName string, skipPermissions bool) tea.Cmd {
	return func() tea.Msg {
		mapping, ok := skillMappings[label]
		if !ok {
			return navigateBackMsg{}
		}

		// Use shared presence from root menu if available; otherwise create our own.
		presence := sharedPresence
		ownPresence := false
		if presence == nil {
			gs, _ := globalconfig.LoadSettings()
			if gs.DiscordPresence {
				p := &discord.Presence{}
				_ = p.Connect()
				presence = p
				ownPresence = true
			}
		}

		// Update presence with the selected skill's verb.
		if presence != nil {
			_ = presence.Update(discord.PresenceState{
				FeatureTitle: mapping.title,
				Verb:         mapping.detail,
				StartTime:    time.Now(),
			})
		}

		// Ensure the maggus plugin is installed for non-plain skills.
		if mapping.skill != "" && agentName == "claude" {
			if err := ensureMaggusPlugin(); err != nil {
				if ownPresence && presence != nil {
					_ = presence.Close()
				}
				return navigateBackMsg{}
			}
		}

		var prompt string
		if mapping.skill != "" {
			prompt = mapping.skill
		}

		cmd, info, err := buildInteractiveCmd(agentName, prompt, dir, skipPermissions, resolvedModel)
		if err != nil {
			if ownPresence && presence != nil {
				_ = presence.Close()
			}
			return navigateBackMsg{}
		}

		return execProcessMsg{
			cmd: cmd,
			onDone: func(err error) tea.Msg {
				info.EndTime = time.Now()
				if mapping.kind != "" {
					extractSkillUsage(dir, resolvedModel, agentName, mapping.kind, info)
				}
				if ownPresence && presence != nil {
					_ = presence.Close()
				}
				return navigateBackMsg{}
			},
		}
	}
}

// skipSeparatorUp moves the cursor up, skipping any separator rows.
func (m *promptPickerModel) skipSeparatorUp() {
	for m.skillCursor > 0 && m.skills[m.skillCursor].separator {
		m.skillCursor--
	}
}

// skipSeparatorDown moves the cursor down, skipping any separator rows.
func (m *promptPickerModel) skipSeparatorDown() {
	for m.skillCursor < len(m.skills)-1 && m.skills[m.skillCursor].separator {
		m.skillCursor++
	}
}

func (m promptPickerModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	activeOffStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	focusLabelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Prompt Mode"))
	sb.WriteString("\n\n")

	// Skill list.
	separatorStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	for i, skill := range m.skills {
		if skill.separator {
			sep := fmt.Sprintf("  ─── %s ───", skill.label)
			fmt.Fprintf(&sb, "  %s\n", separatorStyle.Render(sep))
			continue
		}
		if m.focus == focusSkillList && i == m.skillCursor {
			fmt.Fprintf(&sb, "  %s %s\n",
				cursorStyle.Render("->"),
				selectedStyle.Render(skill.label),
			)
		} else if i == m.skillCursor {
			fmt.Fprintf(&sb, "     %s\n", selectedStyle.Render(skill.label))
		} else {
			fmt.Fprintf(&sb, "     %s\n", mutedStyle.Render(skill.label))
		}
	}

	sb.WriteString("\n")

	// Skip permissions toggle.
	toggleLabel := "Skip permissions"
	if m.focus == focusToggle {
		toggleLabel = focusLabelStyle.Render(toggleLabel)
	}

	onStyle := mutedStyle
	offStyle := mutedStyle
	if m.skipPermissions {
		onStyle = activeValueStyle
	} else {
		offStyle = activeOffStyle
	}

	toggleValue := onStyle.Render("on") + mutedStyle.Render(" / ") + offStyle.Render("off")

	if m.focus == focusToggle {
		fmt.Fprintf(&sb, "  %s %s  %s\n",
			cursorStyle.Render("->"),
			toggleLabel,
			toggleValue,
		)
	} else {
		fmt.Fprintf(&sb, "     %s  %s\n", toggleLabel, toggleValue)
	}

	content := sb.String()
	footer := styles.StatusBar.Render("up/down: navigate | tab: switch focus | enter: confirm | left/right: toggle | q/esc: cancel")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, styles.Primary)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
}

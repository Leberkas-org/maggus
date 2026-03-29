package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// promptPickerResult holds the user's selection from the prompt picker TUI.
type promptPickerResult struct {
	Skill           string
	SkipPermissions bool
	Cancelled       bool
}

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
	result          promptPickerResult
	width           int
	height          int
}

func newPromptPickerModel() promptPickerModel {
	return promptPickerModel{
		skills:          defaultSkills,
		skipPermissions: true,
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
			m.result = promptPickerResult{Cancelled: true}
			return m, tea.Quit

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
				m.result = m.buildResult()
				return m, tea.Quit
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

func (m promptPickerModel) buildResult() promptPickerResult {
	return promptPickerResult{
		Skill:           m.skills[m.skillCursor].label,
		SkipPermissions: m.skipPermissions,
		Cancelled:       false,
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

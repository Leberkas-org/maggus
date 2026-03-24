package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// promptPickerResult holds the user's selection from the prompt picker TUI.
type promptPickerResult struct {
	Skill           string
	Description     string
	SkipPermissions bool
	Cancelled       bool
}

// promptPickerFocus tracks which UI element has focus.
type promptPickerFocus int

const (
	focusSkillList promptPickerFocus = iota
	focusDescription
	focusToggle
)

// skillOption represents a selectable skill in the picker.
type skillOption struct {
	label string
}

var defaultSkills = []skillOption{
	{label: "open console"},
	{label: "/maggus-plan"},
	{label: "/maggus-vision"},
	{label: "/maggus-architecture"},
	{label: "/maggus-bugreport"},
	{label: "/bryan-plan"},
	{label: "/bryan-bugreport"},
}

type promptPickerModel struct {
	skills          []skillOption
	skillCursor     int
	focus           promptPickerFocus
	descInput       textinput.Model
	skipPermissions bool
	result          promptPickerResult
	width           int
	height          int
}

func newPromptPickerModel() promptPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Enter description..."
	ti.CharLimit = 500
	ti.Width = 60

	return promptPickerModel{
		skills:          defaultSkills,
		skipPermissions: true,
		descInput:       ti,
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

		// Global quit keys (but not when typing in the description field).
		if m.focus == focusDescription {
			switch key {
			case "esc":
				m.result = promptPickerResult{Cancelled: true}
				return m, tea.Quit
			case "ctrl+c":
				m.result = promptPickerResult{Cancelled: true}
				return m, tea.Quit
			case "tab", "shift+tab":
				// Tab out of description input.
				m.descInput.Blur()
				if key == "shift+tab" {
					m.focus = focusSkillList
				} else {
					m.focus = focusToggle
				}
				return m, nil
			case "enter":
				// Confirm selection from description input.
				m.result = m.buildResult()
				return m, tea.Quit
			default:
				// Forward all other keys to the text input.
				var cmd tea.Cmd
				m.descInput, cmd = m.descInput.Update(msg)
				return m, cmd
			}
		}

		switch key {
		case "q", "esc", "ctrl+c":
			m.result = promptPickerResult{Cancelled: true}
			return m, tea.Quit

		case "up", "k":
			if m.focus == focusSkillList {
				if m.skillCursor > 0 {
					m.skillCursor--
				}
			} else if m.focus == focusDescription {
				// handled above
			} else if m.focus == focusToggle {
				if m.isPlainSelected() {
					m.focus = focusSkillList
				} else {
					m.focus = focusDescription
					m.descInput.Focus()
				}
			}

		case "down", "j":
			if m.focus == focusSkillList {
				if m.skillCursor < len(m.skills)-1 {
					m.skillCursor++
				} else {
					// Down past the list moves focus.
					if m.isPlainSelected() {
						m.focus = focusToggle
					} else {
						m.focus = focusDescription
						m.descInput.Focus()
					}
				}
			} else if m.focus == focusToggle {
				// Already at bottom, do nothing.
			}

		case "tab":
			switch m.focus {
			case focusSkillList:
				if m.isPlainSelected() {
					m.focus = focusToggle
				} else {
					m.focus = focusDescription
					m.descInput.Focus()
				}
			case focusDescription:
				m.descInput.Blur()
				m.focus = focusToggle
			case focusToggle:
				m.focus = focusSkillList
			}

		case "shift+tab":
			switch m.focus {
			case focusSkillList:
				m.focus = focusToggle
			case focusDescription:
				m.descInput.Blur()
				m.focus = focusSkillList
			case focusToggle:
				if m.isPlainSelected() {
					m.focus = focusSkillList
				} else {
					m.focus = focusDescription
					m.descInput.Focus()
				}
			}

		case "left", "right":
			if m.focus == focusToggle {
				m.skipPermissions = !m.skipPermissions
			}

		case "enter":
			if m.focus == focusSkillList {
				// Move to next focusable element.
				if m.isPlainSelected() {
					m.focus = focusToggle
				} else {
					m.focus = focusDescription
					m.descInput.Focus()
				}
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
		Description:     m.descInput.Value(),
		SkipPermissions: m.skipPermissions,
		Cancelled:       false,
	}
}

func (m promptPickerModel) isPlainSelected() bool {
	return m.skillCursor == 0
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
	for i, skill := range m.skills {
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

	// Description input.
	if m.isPlainSelected() {
		descLabel := mutedStyle.Render("Description")
		descField := mutedStyle.Render("(not available for Plain)")
		fmt.Fprintf(&sb, "  %s  %s\n", descLabel, descField)
	} else {
		label := "Description"
		if m.focus == focusDescription {
			label = focusLabelStyle.Render(label)
		}
		fmt.Fprintf(&sb, "  %s\n", label)
		fmt.Fprintf(&sb, "  %s\n", m.descInput.View())
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
	footer := styles.StatusBar.Render("up/down: navigate | tab: switch focus | enter: confirm | left/right: toggle | q/esc: exit")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, styles.Primary)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
}

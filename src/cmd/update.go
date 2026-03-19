package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/leberkas-org/maggus/internal/updater"
	"github.com/spf13/cobra"
)

// checkLatestVersion is a package-level var so tests can replace it.
var checkLatestVersion = func(currentVersion string) updater.UpdateInfo {
	return updater.CheckLatestVersion(currentVersion)
}

// applyUpdate is a package-level var so tests can replace it.
var applyUpdate = func(downloadURL string) error {
	return updater.Apply(downloadURL)
}

// updatePhase represents the current phase of the update flow.
type updatePhase int

const (
	phaseChecking    updatePhase = iota // checking for updates
	phaseUpToDate                       // already on latest version
	phaseConfirm                        // update available, waiting for confirmation
	phaseDownloading                    // downloading and applying
	phaseSuccess                        // update applied successfully
	phaseError                          // an error occurred
)

// updateCheckMsg is sent when the version check completes.
type updateCheckMsg struct {
	info updater.UpdateInfo
}

// updateApplyMsg is sent when the apply completes.
type updateApplyMsg struct {
	err error
}

// updateTickMsg drives the spinner.
type updateTickMsg time.Time

// updateModel is the bubbletea model for the update TUI.
type updateModel struct {
	phase          updatePhase
	currentVersion string
	info           updater.UpdateInfo
	errorMsg       string
	frame          int
	width          int
	height         int
	menuChoice     int // 0 = Install, 1 = Cancel (confirm phase)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newUpdateModel(currentVersion string) updateModel {
	return updateModel{
		phase:          phaseChecking,
		currentVersion: currentVersion,
	}
}

func (m updateModel) Init() tea.Cmd {
	return tea.Batch(
		m.checkVersion,
		updateTickCmd(),
	)
}

func updateTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return updateTickMsg(t)
	})
}

func (m updateModel) checkVersion() tea.Msg {
	info := checkLatestVersion(m.currentVersion)
	return updateCheckMsg{info: info}
}

func (m updateModel) applyVersion() tea.Msg {
	err := applyUpdate(m.info.DownloadURL)
	return updateApplyMsg{err: err}
}

func (m updateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case updateTickMsg:
		m.frame = (m.frame + 1) % len(spinnerFrames)
		return m, updateTickCmd()

	case updateCheckMsg:
		m.info = msg.info
		if !msg.info.IsNewer {
			m.phase = phaseUpToDate
			return m, nil
		}
		if msg.info.DownloadURL == "" {
			m.phase = phaseError
			m.errorMsg = "No download available for your platform"
			return m, nil
		}
		m.phase = phaseConfirm
		return m, nil

	case updateApplyMsg:
		if msg.err != nil {
			m.phase = phaseError
			m.errorMsg = fmt.Sprintf("Update failed: %v", msg.err)
			return m, nil
		}
		m.phase = phaseSuccess
		return m, nil

	case tea.KeyMsg:
		switch m.phase {
		case phaseChecking, phaseDownloading:
			// Only allow quit during async phases
			if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEscape {
				return m, tea.Quit
			}

		case phaseUpToDate, phaseSuccess, phaseError:
			// Any key exits
			return m, tea.Quit

		case phaseConfirm:
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEscape:
				return m, tea.Quit
			case tea.KeyUp, tea.KeyShiftTab:
				if m.menuChoice > 0 {
					m.menuChoice--
				}
			case tea.KeyDown, tea.KeyTab:
				if m.menuChoice < 1 {
					m.menuChoice++
				}
			case tea.KeyEnter:
				if m.menuChoice == 0 {
					// Install
					m.phase = phaseDownloading
					return m, m.applyVersion
				}
				// Cancel
				return m, tea.Quit
			default:
				if len(msg.Runes) == 1 {
					switch msg.Runes[0] {
					case 'y', 'Y':
						m.phase = phaseDownloading
						return m, m.applyVersion
					case 'n', 'N', 'q', 'Q':
						return m, tea.Quit
					case 'j':
						if m.menuChoice < 1 {
							m.menuChoice++
						}
					case 'k':
						if m.menuChoice > 0 {
							m.menuChoice--
						}
					}
				}
			}
		}
	}

	return m, nil
}

func (m updateModel) View() string {
	var content strings.Builder
	var footer string

	titleStyle := styles.Title
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)
	cyanSt := lipgloss.NewStyle().Foreground(styles.Primary)

	// Header
	content.WriteString(titleStyle.Render("Maggus Update") + "\n")
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	content.WriteString(styles.Separator(innerW) + "\n\n")

	// Current version
	currentDisplay := m.currentVersion
	if currentDisplay != "dev" {
		currentDisplay = "v" + strings.TrimPrefix(currentDisplay, "v")
	}
	content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Current:"), valStyle.Render(currentDisplay)))

	switch m.phase {
	case phaseChecking:
		spinner := cyanSt.Render(spinnerFrames[m.frame])
		content.WriteString(fmt.Sprintf("\n%s Checking for updates...\n", spinner))
		footer = styles.StatusBar.Render("esc: cancel")

	case phaseUpToDate:
		content.WriteString(fmt.Sprintf("\n%s\n", successStyle.Render("Already up to date!")))
		footer = styles.StatusBar.Render("press any key to exit")

	case phaseConfirm:
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Latest:"), successStyle.Render(m.info.TagName)))
		content.WriteString(fmt.Sprintf("\n%s\n", successStyle.Render(fmt.Sprintf("Update available: %s → %s", currentDisplay, m.info.TagName))))

		if m.info.Body != "" {
			content.WriteString(fmt.Sprintf("\n%s\n", styles.Subtitle.Render("Changelog")))
			content.WriteString(styles.Separator(innerW) + "\n")
			content.WriteString(m.info.Body + "\n")
		}

		content.WriteString("\n")

		// Menu
		selectedStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		normalStyle := lipgloss.NewStyle().Foreground(styles.Muted)

		if m.menuChoice == 0 {
			content.WriteString(fmt.Sprintf("  %s %s\n", selectedStyle.Render("▸"), selectedStyle.Render("Install update")))
		} else {
			content.WriteString(fmt.Sprintf("  %s %s\n", normalStyle.Render(" "), normalStyle.Render("Install update")))
		}
		if m.menuChoice == 1 {
			content.WriteString(fmt.Sprintf("  %s %s\n", selectedStyle.Render("▸"), selectedStyle.Render("Cancel")))
		} else {
			content.WriteString(fmt.Sprintf("  %s %s\n", normalStyle.Render(" "), normalStyle.Render("Cancel")))
		}

		footer = styles.StatusBar.Render("↑/↓: select · enter: confirm · y: install · q/esc: cancel")

	case phaseDownloading:
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Latest:"), successStyle.Render(m.info.TagName)))
		spinner := cyanSt.Render(spinnerFrames[m.frame])
		content.WriteString(fmt.Sprintf("\n%s Downloading and installing %s...\n", spinner, m.info.TagName))
		footer = styles.StatusBar.Render("please wait...")

	case phaseSuccess:
		content.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Updated:"), successStyle.Render(m.info.TagName)))
		content.WriteString(fmt.Sprintf("\n%s\n", successStyle.Render(fmt.Sprintf("Successfully updated to %s!", m.info.TagName))))
		content.WriteString(fmt.Sprintf("\n%s\n", valStyle.Render("Please restart maggus to use the new version.")))
		footer = styles.StatusBar.Render("press any key to exit")

	case phaseError:
		content.WriteString(fmt.Sprintf("\n%s\n", errStyle.Render(m.errorMsg)))
		footer = styles.StatusBar.Render("press any key to exit")
	}

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(content.String(), footer, m.width, m.height)
	}
	return styles.Box.Render(content.String()) + "\n"
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long: `Checks GitHub Releases for a newer version of maggus and offers to install it.

Examples:
  maggus update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newUpdateModel(Version)
		prog := tea.NewProgram(m, tea.WithAltScreen())
		_, err := prog.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/globalconfig"
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

// autoUpdateModes lists the cycle order for the auto-update setting.
var autoUpdateModes = []globalconfig.AutoUpdateMode{
	globalconfig.AutoUpdateOff,
	globalconfig.AutoUpdateNotify,
	globalconfig.AutoUpdateAuto,
}

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
	scrollOffset   int // vertical scroll offset for content

	// Auto-update setting
	autoUpdateIdx     int  // index into autoUpdateModes
	autoUpdateOrigIdx int  // original index (to detect changes)
	autoUpdateDirty   bool // true if user changed the setting

	is2x       bool // true when Claude is in 2x mode (border turns yellow)
	standalone bool // true when run via the cobra command (uses tea.Quit); false when embedded in the app router (uses navigateBackMsg)
}

// loadGlobalSettings is injectable for testing.
var loadGlobalSettings = func() (globalconfig.Settings, error) {
	return globalconfig.LoadSettings()
}

// saveGlobalSettings is injectable for testing.
var saveGlobalSettings = func(s globalconfig.Settings) error {
	return globalconfig.SaveSettings(s)
}

func newUpdateModel(currentVersion string) updateModel {
	// Load current auto-update setting
	idx := 1 // default to "notify"
	settings, err := loadGlobalSettings()
	if err == nil {
		for i, mode := range autoUpdateModes {
			if mode == settings.AutoUpdate {
				idx = i
				break
			}
		}
	}

	return updateModel{
		phase:             phaseChecking,
		currentVersion:    currentVersion,
		autoUpdateIdx:     idx,
		autoUpdateOrigIdx: idx,
	}
}

func (m updateModel) Init() tea.Cmd {
	return tea.Batch(
		m.checkVersion,
		updateTickCmd(),
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
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
		clampUpdateScroll(&m)
		return m, nil

	case updateTickMsg:
		m.frame = (m.frame + 1) % len(styles.SpinnerFrames)
		return m, updateTickCmd()

	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, _, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		return m, tickCmd

	case updateCheckMsg:
		m.info = msg.info
		m.scrollOffset = 0
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
		m.scrollOffset = 0
		if msg.err != nil {
			m.phase = phaseError
			m.errorMsg = fmt.Sprintf("Update failed: %v", msg.err)
			return m, nil
		}
		m.phase = phaseSuccess
		return m, nil

	case tea.KeyMsg:
		// 'a' cycles auto-update setting in all non-async phases
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'a' || msg.Runes[0] == 'A') {
			if m.phase != phaseChecking && m.phase != phaseDownloading {
				m.autoUpdateIdx = (m.autoUpdateIdx + 1) % len(autoUpdateModes)
				m.autoUpdateDirty = m.autoUpdateIdx != m.autoUpdateOrigIdx
				return m, nil
			}
		}

		switch m.phase {
		case phaseUpToDate, phaseSuccess, phaseError:
			// Any key exits (except 'a' which is handled above)
			saveAutoUpdateIfDirty(&m)
			return m, m.doneCmd()

		case phaseConfirm:
			switch msg.Type {
			case tea.KeyUp:
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			case tea.KeyDown:
				m.scrollOffset++
				clampUpdateScroll(&m)
			case tea.KeyLeft, tea.KeyShiftTab:
				if m.menuChoice > 0 {
					m.menuChoice--
				}
			case tea.KeyRight, tea.KeyTab:
				if m.menuChoice < 1 {
					m.menuChoice++
				}
			case tea.KeyHome:
				m.scrollOffset = 0
			case tea.KeyEnd:
				m.scrollOffset = m.totalContentLines()
				clampUpdateScroll(&m)
			case tea.KeyPgUp:
				m.scrollOffset -= m.viewportHeight()
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			case tea.KeyPgDown:
				m.scrollOffset += m.viewportHeight()
				clampUpdateScroll(&m)
			case tea.KeyEnter:
				saveAutoUpdateIfDirty(&m)
				if m.menuChoice == 0 {
					// Install
					m.phase = phaseDownloading
					m.scrollOffset = 0
					return m, m.applyVersion
				}
				// Cancel
				return m, m.doneCmd()
			default:
				if len(msg.Runes) == 1 {
					switch msg.Runes[0] {
					case 'y', 'Y':
						m.phase = phaseDownloading
						m.scrollOffset = 0
						return m, m.applyVersion
					case 'n', 'N', 'q', 'Q':
						saveAutoUpdateIfDirty(&m)
						return m, m.doneCmd()
					case 'j':
						m.scrollOffset++
						clampUpdateScroll(&m)
					case 'k':
						if m.scrollOffset > 0 {
							m.scrollOffset--
						}
					case 'h':
						if m.menuChoice > 0 {
							m.menuChoice--
						}
					case 'l':
						if m.menuChoice < 1 {
							m.menuChoice++
						}
					}
				}
			}
		}
	}

	return m, nil
}

// doneCmd returns the appropriate tea.Cmd to exit the update screen.
// In standalone mode it quits the program; when embedded it navigates back to the menu.
func (m updateModel) doneCmd() tea.Cmd {
	if m.standalone {
		return tea.Quit
	}
	return func() tea.Msg { return navigateBackMsg{} }
}

// saveAutoUpdateIfDirty persists the auto-update setting if it was changed.
func saveAutoUpdateIfDirty(m *updateModel) {
	if !m.autoUpdateDirty {
		return
	}
	settings, err := loadGlobalSettings()
	if err != nil {
		settings = globalconfig.DefaultSettings()
	}
	settings.AutoUpdate = autoUpdateModes[m.autoUpdateIdx]
	_ = saveGlobalSettings(settings)
	m.autoUpdateDirty = false
	m.autoUpdateOrigIdx = m.autoUpdateIdx
}

// viewportHeight returns the number of content lines visible in the viewport.
// It subtracts the footer height plus a gap line so content never collides
// with the pinned footer rendered by FullScreenLeft.
func (m updateModel) viewportHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	footerLines := 1
	if m.phase == phaseConfirm {
		footerLines = 2 // menu line + hints line
	}
	gap := 1 // always keep at least one blank line before footer
	vp := innerH - footerLines - gap
	if vp < 1 {
		return 1
	}
	return vp
}

// totalContentLines returns the total number of lines in the current content.
func (m updateModel) totalContentLines() int {
	content := m.renderContent()
	return strings.Count(content, "\n") + 1
}

// clampUpdateScroll ensures scrollOffset stays within valid bounds.
func clampUpdateScroll(m *updateModel) {
	total := m.totalContentLines()
	vp := m.viewportHeight()
	maxOffset := total - vp
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// renderContent builds the full (unscrolled) content string for the current phase.
func (m updateModel) renderContent() string {
	var content strings.Builder

	titleStyle := styles.Title
	labelStyle := styles.Label.Width(10).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	successStyle := lipgloss.NewStyle().Foreground(styles.Success)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)
	cyanSt := lipgloss.NewStyle().Foreground(styles.Primary)

	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)

	// Header
	content.WriteString(titleStyle.Render("Maggus Update") + "\n")
	content.WriteString(styles.Separator(innerW) + "\n\n")

	// Current version
	currentDisplay := m.currentVersion
	if !strings.HasPrefix(currentDisplay, "dev") {
		currentDisplay = "v" + strings.TrimPrefix(currentDisplay, "v")
	}
	fmt.Fprintf(&content, "%s  %s\n", labelStyle.Render("Current:"), valStyle.Render(currentDisplay))

	// Auto-update setting (toggleable with 'a')
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	activeOffStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	var modeParts []string
	for i, mode := range autoUpdateModes {
		label := string(mode)
		if i == m.autoUpdateIdx {
			if mode == globalconfig.AutoUpdateOff {
				modeParts = append(modeParts, activeOffStyle.Render(label))
			} else {
				modeParts = append(modeParts, activeStyle.Render(label))
			}
		} else {
			modeParts = append(modeParts, valStyle.Render(label))
		}
	}
	autoLine := strings.Join(modeParts, valStyle.Render(" / "))
	if m.autoUpdateDirty {
		autoLine += " " + lipgloss.NewStyle().Foreground(styles.Warning).Render("(modified)")
	}
	fmt.Fprintf(&content, "%s  %s\n", labelStyle.Render("Auto:"), autoLine)

	switch m.phase {
	case phaseChecking:
		spinner := cyanSt.Render(styles.SpinnerFrames[m.frame])
		fmt.Fprintf(&content, "\n%s Checking for updates...\n", spinner)

	case phaseUpToDate:
		fmt.Fprintf(&content, "\n%s\n", successStyle.Render("Already up to date!"))

	case phaseConfirm:
		fmt.Fprintf(&content, "%s  %s\n", labelStyle.Render("Latest:"), successStyle.Render(m.info.TagName))
		fmt.Fprintf(&content, "\n%s\n", successStyle.Render(fmt.Sprintf("Update available: %s → %s", currentDisplay, m.info.TagName)))

		if m.info.Body != "" {
			fmt.Fprintf(&content, "\n%s\n", styles.Subtitle.Render("Changelog"))
			content.WriteString(styles.Separator(innerW) + "\n")
			content.WriteString(m.info.Body + "\n")
		}

		content.WriteString("\n")

	case phaseDownloading:
		fmt.Fprintf(&content, "%s  %s\n", labelStyle.Render("Latest:"), successStyle.Render(m.info.TagName))
		spinner := cyanSt.Render(styles.SpinnerFrames[m.frame])
		fmt.Fprintf(&content, "\n%s Downloading and installing %s...\n", spinner, m.info.TagName)

	case phaseSuccess:
		fmt.Fprintf(&content, "%s  %s\n", labelStyle.Render("Updated:"), successStyle.Render(m.info.TagName))
		fmt.Fprintf(&content, "\n%s\n", successStyle.Render(fmt.Sprintf("Successfully updated to %s!", m.info.TagName)))
		fmt.Fprintf(&content, "\n%s\n", valStyle.Render("Please restart maggus to use the new version."))

	case phaseError:
		fmt.Fprintf(&content, "\n%s\n", errStyle.Render(m.errorMsg))
	}

	return content.String()
}

func (m updateModel) View() string {
	fullContent := m.renderContent()

	// Build footer based on phase
	var footer string
	switch m.phase {
	case phaseChecking:
		footer = styles.StatusBar.Render("esc: cancel")
	case phaseUpToDate:
		footer = styles.StatusBar.Render("a: auto-update mode · any key: exit")
	case phaseConfirm:
		// Menu rendered as part of the pinned footer
		selectedStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		normalStyle := lipgloss.NewStyle().Foreground(styles.Muted)

		var installLabel, cancelLabel string
		if m.menuChoice == 0 {
			installLabel = selectedStyle.Render("▸ Install update")
		} else {
			installLabel = normalStyle.Render("  Install update")
		}
		if m.menuChoice == 1 {
			cancelLabel = selectedStyle.Render("▸ Cancel")
		} else {
			cancelLabel = normalStyle.Render("  Cancel")
		}
		menu := fmt.Sprintf("%s    %s", installLabel, cancelLabel)

		total := strings.Count(fullContent, "\n") + 1
		vp := m.viewportHeight()
		var hints string
		if total > vp {
			hints = styles.StatusBar.Render("↑/↓: scroll · ←/→: select · a: auto-update · enter: confirm · q: cancel")
		} else {
			hints = styles.StatusBar.Render("←/→: select · a: auto-update · enter: confirm · q: cancel")
		}
		footer = menu + "\n" + hints
	case phaseDownloading:
		footer = styles.StatusBar.Render("please wait...")
	case phaseSuccess:
		footer = styles.StatusBar.Render("a: auto-update mode · any key: exit")
	case phaseError:
		footer = styles.StatusBar.Render("a: auto-update mode · any key: exit")
	}

	// Apply viewport scrolling — always produce exactly vp lines so
	// FullScreenLeft sees a consistent content height and keeps the
	// footer pinned to the bottom.
	lines := strings.Split(fullContent, "\n")
	vp := m.viewportHeight()
	offset := m.scrollOffset
	if offset > len(lines) {
		offset = len(lines)
	}

	var visibleLines []string
	if len(lines) > vp {
		end := offset + vp
		if end > len(lines) {
			end = len(lines)
		}
		visibleLines = lines[offset:end]
	} else {
		visibleLines = lines
	}

	// Pad to exactly vp lines so the box height stays constant.
	for len(visibleLines) < vp {
		visibleLines = append(visibleLines, "")
	}

	visible := strings.Join(visibleLines, "\n")

	borderColor := styles.ThemeColor(m.is2x)
	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeftColor(visible, footer, m.width, m.height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(visible) + "\n"
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long: `Checks GitHub Releases for a newer version of maggus and offers to install it.

Examples:
  maggus update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := newUpdateModel(Version)
		m.standalone = true
		prog := tea.NewProgram(m, tea.WithAltScreen())
		_, err := prog.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

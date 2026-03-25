package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Edit project settings interactively",
	Long:  `Opens an interactive editor for .maggus/config.yml settings.`,
	Args:  cobra.NoArgs,
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	m := newConfigModel(cfg, dir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// configAction represents the user's chosen action on exit.
type configAction int

const (
	configActionNone        configAction = iota
	configActionSaveProject              // save .maggus/config.yml
	configActionSaveGlobal               // save ~/.maggus/config.yml
	configActionEditProject              // open .maggus/config.yml in editor
	configActionEditGlobal               // open ~/.maggus/config.yml in editor
)

// configRow is a single navigable row in the config TUI.
// It is either a setting (values != nil) or an action button.
type configRow struct {
	label   string
	values  []string     // nil for action buttons
	current int          // selected value index (settings only)
	action  configAction // non-zero for action buttons
	section string       // non-empty triggers a section header before this row
	isSave  bool         // render with save style
	display string       // non-empty = read-only display row (no cycling)
}

func (r configRow) isOption() bool  { return r.values != nil }
func (r configRow) isDisplay() bool { return r.display != "" }

// configResultMsg is sent after an async save/edit action completes.
type configResultMsg struct {
	text string
	err  error
}

type configModel struct {
	projectRows           []configRow
	globalRows            []configRow
	activeTab             int // 0 = Project, 1 = Global
	tabFocused            bool
	cursor                int
	action                configAction
	width                 int
	height                int
	dir                   string
	globalAutoUpdateIdx   int
	origInclude           []string
	origProtectedBranches []string
	statusText            string
	is2x                  bool
}

// activeRows returns a pointer to the currently active tab's row slice.
func (m *configModel) activeRows() *[]configRow {
	if m.activeTab == 1 {
		return &m.globalRows
	}
	return &m.projectRows
}

func newConfigModel(cfg config.Config, dir string) configModel {
	agentValues := []string{"claude", "opencode"}
	agentIdx := indexOf(agentValues, cfg.Agent)

	modelValues := []string{"(default)", "sonnet", "opus", "haiku"}
	modelIdx := indexOf(modelValues, cfg.Model)

	worktreeValues := []string{"on", "off"}
	worktreeIdx := 1
	if cfg.Worktree {
		worktreeIdx = 0
	}

	autoApproveValues := []string{"disabled", "enabled"}
	autoApproveIdx := 0
	if cfg.ApprovalMode == config.ApprovalModeOptOut {
		autoApproveIdx = 1
	}

	autoBranchValues := []string{"on", "off"}
	autoBranchIdx := 0
	if !cfg.Git.IsAutoBranchEnabled() {
		autoBranchIdx = 1
	}

	checkSyncValues := []string{"on", "off"}
	checkSyncIdx := 0
	if !cfg.Git.IsCheckSyncEnabled() {
		checkSyncIdx = 1
	}

	protectedDisplay := strings.Join(cfg.Git.ProtectedBranchList(), ", ")
	if len(protectedDisplay) > 40 {
		protectedDisplay = protectedDisplay[:37] + "..."
	}

	soundValues := []string{"on", "off"}
	soundIdx := 1
	if cfg.Notifications.Sound {
		soundIdx = 0
	}

	taskCompleteValues := []string{"on", "off"}
	taskCompleteIdx := 0
	if cfg.Notifications.OnTaskComplete != nil && !*cfg.Notifications.OnTaskComplete {
		taskCompleteIdx = 1
	}

	runCompleteValues := []string{"on", "off"}
	runCompleteIdx := 0
	if cfg.Notifications.OnRunComplete != nil && !*cfg.Notifications.OnRunComplete {
		runCompleteIdx = 1
	}

	errorValues := []string{"on", "off"}
	errorIdx := 0
	if cfg.Notifications.OnError != nil && !*cfg.Notifications.OnError {
		errorIdx = 1
	}

	onCompleteValues := []string{"rename", "delete"}
	featureActionIdx := indexOf(onCompleteValues, cfg.OnComplete.FeatureAction())
	bugActionIdx := indexOf(onCompleteValues, cfg.OnComplete.BugAction())

	// Load global settings
	autoUpdateValues := []string{"off", "notify", "auto"}
	autoUpdateIdx := 1 // default: notify
	discordPresenceValues := []string{"on", "off"}
	discordPresenceIdx := 1
	globalSettings, err := loadGlobalSettings()
	if err == nil {
		autoUpdateIdx = indexOf(autoUpdateValues, string(globalSettings.AutoUpdate))
		if globalSettings.DiscordPresence {
			discordPresenceIdx = 0
		}
	}

	projectRows := []configRow{
		{label: "Agent", values: agentValues, current: agentIdx},
		{label: "Model", values: modelValues, current: modelIdx},
		{label: "Worktree", values: worktreeValues, current: worktreeIdx},
		{label: "Auto-approve", values: autoApproveValues, current: autoApproveIdx},
		{label: "Auto-branch", values: autoBranchValues, current: autoBranchIdx},
		{label: "Check sync", values: checkSyncValues, current: checkSyncIdx},
		{label: "Protected branches", display: protectedDisplay},
		{label: "Sound", values: soundValues, current: soundIdx},
		{label: "  On task complete", values: taskCompleteValues, current: taskCompleteIdx},
		{label: "  On run complete", values: runCompleteValues, current: runCompleteIdx},
		{label: "  On error", values: errorValues, current: errorIdx},
		{label: "  Feature", values: onCompleteValues, current: featureActionIdx, section: "On complete behaviour"},
		{label: "  Bug", values: onCompleteValues, current: bugActionIdx},
		{label: "Save project config", action: configActionSaveProject, isSave: true},
		{label: "Edit project file in editor", action: configActionEditProject},
	}

	globalRows := []configRow{
		{label: "Discord presence", values: discordPresenceValues, current: discordPresenceIdx},
		{label: "Auto-update", values: autoUpdateValues, current: autoUpdateIdx},
		{label: "Save global config", action: configActionSaveGlobal, isSave: true},
		{label: "Edit global file in editor", action: configActionEditGlobal},
	}

	// Find the auto-update row index within globalRows for saveGlobalConfig
	globalAutoUpdateIdx := 0
	for i, r := range globalRows {
		if r.label == "Auto-update" && r.isOption() {
			globalAutoUpdateIdx = i
			break
		}
	}

	return configModel{
		projectRows:           projectRows,
		globalRows:            globalRows,
		globalAutoUpdateIdx:   globalAutoUpdateIdx,
		dir:                   dir,
		origInclude:           cfg.Include,
		origProtectedBranches: cfg.Git.ProtectedBranchList(),
	}
}

// optionByLabel finds the first option row with the given label,
// searching both projectRows and globalRows so buildConfig works from either tab.
func (m configModel) optionByLabel(label string) configRow {
	for _, r := range m.projectRows {
		if r.label == label && r.isOption() {
			return r
		}
	}
	for _, r := range m.globalRows {
		if r.label == label && r.isOption() {
			return r
		}
	}
	return configRow{}
}

func (m configModel) buildConfig() config.Config {
	cfg := config.Config{
		Agent:    m.optionByLabel("Agent").values[m.optionByLabel("Agent").current],
		Worktree: m.optionByLabel("Worktree").values[m.optionByLabel("Worktree").current] == "on",
	}

	modelRow := m.optionByLabel("Model")
	model := modelRow.values[modelRow.current]
	if model != "(default)" {
		cfg.Model = model
	}

	cfg.Notifications.Sound = m.optionByLabel("Sound").values[m.optionByLabel("Sound").current] == "on"

	if m.optionByLabel("  On task complete").values[m.optionByLabel("  On task complete").current] == "off" {
		f := false
		cfg.Notifications.OnTaskComplete = &f
	}
	if m.optionByLabel("  On run complete").values[m.optionByLabel("  On run complete").current] == "off" {
		f := false
		cfg.Notifications.OnRunComplete = &f
	}
	if m.optionByLabel("  On error").values[m.optionByLabel("  On error").current] == "off" {
		f := false
		cfg.Notifications.OnError = &f
	}

	if m.optionByLabel("Auto-branch").values[m.optionByLabel("Auto-branch").current] == "off" {
		f := false
		cfg.Git.AutoBranch = &f
	}
	if m.optionByLabel("Check sync").values[m.optionByLabel("Check sync").current] == "off" {
		f := false
		cfg.Git.CheckSync = &f
	}
	cfg.Git.ProtectedBranches = m.origProtectedBranches

	featureAction := m.optionByLabel("  Feature").values[m.optionByLabel("  Feature").current]
	if featureAction == "delete" {
		cfg.OnComplete.Feature = "delete"
	}
	bugAction := m.optionByLabel("  Bug").values[m.optionByLabel("  Bug").current]
	if bugAction == "delete" {
		cfg.OnComplete.Bug = "delete"
	}

	autoApproveRow := m.optionByLabel("Auto-approve")
	if autoApproveRow.values[autoApproveRow.current] == "enabled" {
		cfg.ApprovalMode = config.ApprovalModeOptOut
	} else {
		cfg.ApprovalMode = config.ApprovalModeOptIn
	}

	return cfg
}

func (m configModel) saveGlobalConfig() error {
	row := m.globalRows[m.globalAutoUpdateIdx]
	mode := globalconfig.AutoUpdateMode(row.values[row.current])
	settings, err := loadGlobalSettings()
	if err != nil {
		settings = globalconfig.DefaultSettings()
	}
	settings.AutoUpdate = mode
	settings.DiscordPresence = m.optionByLabel("Discord presence").values[m.optionByLabel("Discord presence").current] == "on"
	return saveGlobalSettings(settings)
}

func (m configModel) Init() tea.Cmd {
	return func() tea.Msg {
		return claude2xResultMsg{status: claude2x.FetchStatus()}
	}
}

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

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

	case configResultMsg:
		if msg.err != nil {
			m.statusText = "Error: " + msg.err.Error()
		} else {
			m.statusText = msg.text
		}
		return m, nil

	case tea.KeyMsg:
		// Clear status on any keypress
		m.statusText = ""

		rows := m.activeRows()
		itemCount := len(*rows)

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "1":
			m.activeTab = 0
			m.cursor = 0
			m.tabFocused = false
		case "2":
			m.activeTab = 1
			m.cursor = 0
			m.tabFocused = false
		case "up", "k":
			if m.tabFocused {
				// Already at tab bar, do nothing
			} else if m.cursor > 0 {
				m.cursor--
			} else {
				// At row 0, move focus to tab bar
				m.tabFocused = true
			}
		case "down", "j":
			if m.tabFocused {
				m.tabFocused = false
				m.cursor = 0
			} else if m.cursor < itemCount-1 {
				m.cursor++
			}
		case "left", "h":
			if m.tabFocused {
				// Switch to the other tab
				m.activeTab = 1 - m.activeTab
				m.cursor = 0
			} else if row := &(*rows)[m.cursor]; row.isOption() {
				if row.current > 0 {
					row.current--
				} else {
					row.current = len(row.values) - 1
				}
			}
		case "right", "l":
			if m.tabFocused {
				// Switch to the other tab
				m.activeTab = 1 - m.activeTab
				m.cursor = 0
			} else if row := &(*rows)[m.cursor]; row.isOption() {
				if row.current < len(row.values)-1 {
					row.current++
				} else {
					row.current = 0
				}
			}
		case "enter":
			if m.tabFocused {
				m.tabFocused = false
				m.cursor = 0
			} else {
				row := &(*rows)[m.cursor]
				if row.isOption() {
					if row.current < len(row.values)-1 {
						row.current++
					} else {
						row.current = 0
					}
				} else {
					return m, m.executeAction(row.action)
				}
			}
		}
	}
	return m, nil
}

// executeAction returns a tea.Cmd that performs the given config action.
func (m configModel) executeAction(action configAction) tea.Cmd {
	switch action {
	case configActionSaveProject:
		return func() tea.Msg {
			newCfg := m.buildConfig()
			newCfg.Include = m.origInclude
			if err := saveConfig(m.dir, newCfg); err != nil {
				return configResultMsg{err: err}
			}
			return configResultMsg{text: "Saved project config"}
		}
	case configActionSaveGlobal:
		return func() tea.Msg {
			if err := m.saveGlobalConfig(); err != nil {
				return configResultMsg{err: err}
			}
			return configResultMsg{text: "Saved global config"}
		}
	case configActionEditProject:
		return func() tea.Msg {
			path := filepath.Join(m.dir, ".maggus", "config.yml")
			if err := openInEditor(path); err != nil {
				return configResultMsg{err: err}
			}
			return configResultMsg{text: "Opened project config in editor"}
		}
	case configActionEditGlobal:
		return func() tea.Msg {
			path, err := globalconfig.SettingsFilePath()
			if err != nil {
				return configResultMsg{err: err}
			}
			if err := openInEditor(path); err != nil {
				return configResultMsg{err: err}
			}
			return configResultMsg{text: "Opened global config in editor"}
		}
	}
	return nil
}

func (m configModel) renderTabBar() string {
	tabNames := []string{"Project", "Global"}

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(0, 1)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Padding(0, 1)

	focusedActiveStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(styles.Primary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(0, 1)

	var tabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			if m.tabFocused {
				tabs = append(tabs, focusedActiveStyle.Render(name))
			} else {
				tabs = append(tabs, activeStyle.Render(name))
			}
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
}

func (m configModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	activeOffStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	saveStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)

	// Sections without a path display just the title
	sectionTitleOnly := map[string]bool{
		"On complete behaviour": true,
	}

	var sb strings.Builder

	// Render tab bar
	sb.WriteString(m.renderTabBar())
	sb.WriteString("\n\n")

	rows := *m.activeRows()
	for i, row := range rows {
		// Section header
		if row.section != "" {
			if i > 0 {
				sb.WriteString("\n")
			}
			if sectionTitleOnly[row.section] {
				sb.WriteString(titleStyle.Render(row.section) + "\n")
			} else {
				sb.WriteString(titleStyle.Render(row.section) + "\n")
			}
		}

		if row.isOption() {
			label := fmt.Sprintf("%-22s", row.label)

			var valueParts []string
			for vi, v := range row.values {
				if vi == row.current {
					if v == "off" {
						valueParts = append(valueParts, activeOffStyle.Render(v))
					} else {
						valueParts = append(valueParts, activeValueStyle.Render(v))
					}
				} else {
					valueParts = append(valueParts, mutedStyle.Render(v))
				}
			}
			valueStr := strings.Join(valueParts, mutedStyle.Render(" / "))

			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s  %s\n",
					cursorStyle.Render("->"),
					normalStyle.Render(label),
					valueStr,
				)
			} else {
				fmt.Fprintf(&sb, "     %s  %s\n",
					normalStyle.Render(label),
					valueStr,
				)
			}
		} else if row.isDisplay() {
			label := fmt.Sprintf("%-22s", row.label)
			hint := mutedStyle.Render("(edit in config.yml)")
			displayVal := mutedStyle.Render(row.display)
			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s  %s  %s\n",
					cursorStyle.Render("->"),
					normalStyle.Render(label),
					displayVal,
					hint,
				)
			} else {
				fmt.Fprintf(&sb, "     %s  %s  %s\n",
					normalStyle.Render(label),
					displayVal,
					hint,
				)
			}
		} else {
			// Action button
			btnStyle := normalStyle
			if row.isSave {
				btnStyle = saveStyle
			}
			if i == m.cursor {
				fmt.Fprintf(&sb, "  %s %s\n", cursorStyle.Render("->"), btnStyle.Render(row.label))
			} else {
				fmt.Fprintf(&sb, "     %s\n", mutedStyle.Render(row.label))
			}
		}
	}

	// Status feedback
	if m.statusText != "" {
		sb.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(styles.Success)
		if strings.HasPrefix(m.statusText, "Error:") {
			statusStyle = lipgloss.NewStyle().Foreground(styles.Error)
		}
		sb.WriteString("  " + statusStyle.Render(m.statusText) + "\n")
	}

	content := sb.String()
	footer := styles.StatusBar.Render("1/2: switch tab | up/down: navigate | left/right: change value | enter: select | q/esc: exit")

	borderColor := styles.ThemeColor(m.is2x)
	if m.width > 0 && m.height > 0 {
		return styles.FullScreenColor(content, footer, m.width, m.height, borderColor)
	}
	return styles.Box.BorderForeground(borderColor).Render(content+"\n\n"+footer) + "\n"
}

func saveConfig(dir string, cfg config.Config) error {
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		return fmt.Errorf("create .maggus/: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(maggusDir, "config.yml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Saved %s\n", path)
	return nil
}

// openInEditor opens the given file in the user's preferred editor as a
// detached process so it doesn't block maggus or take over the terminal.
func openInEditor(path string) error {
	// Ensure the file exists before opening.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("create directory: %w", mkErr)
		}
		if writeErr := os.WriteFile(path, []byte(defaultConfig), 0o644); writeErr != nil {
			return fmt.Errorf("create config file: %w", writeErr)
		}
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "xdg-open"
		}
	}

	cmd := exec.Command(editor, path)
	// Detach from terminal so the editor runs in the background.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func indexOf(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

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

	m := newConfigModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	final := result.(configModel)
	switch final.action {
	case configActionSaveProject:
		newCfg := final.buildConfig()
		newCfg.Include = cfg.Include
		return saveConfig(dir, newCfg)
	case configActionSaveGlobal:
		return final.saveGlobalConfig()
	case configActionEditProject:
		return openInEditor(filepath.Join(dir, ".maggus", "config.yml"))
	case configActionEditGlobal:
		path, err := globalconfig.SettingsFilePath()
		if err != nil {
			return err
		}
		return openInEditor(path)
	}
	return nil
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
}

func (r configRow) isOption() bool { return r.values != nil }

type configModel struct {
	rows   []configRow
	cursor int
	action configAction
	width  int
	height int
	// indices for buildConfig / saveGlobalConfig
	globalAutoUpdateIdx int
}

func newConfigModel(cfg config.Config) configModel {
	agentValues := []string{"claude", "opencode"}
	agentIdx := indexOf(agentValues, cfg.Agent)

	modelValues := []string{"(default)", "sonnet", "opus", "haiku"}
	modelIdx := indexOf(modelValues, cfg.Model)

	worktreeValues := []string{"on", "off"}
	worktreeIdx := 1
	if cfg.Worktree {
		worktreeIdx = 0
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

	// Load global auto-update setting
	autoUpdateValues := []string{"off", "notify", "auto"}
	autoUpdateIdx := 1 // default: notify
	globalSettings, err := loadGlobalSettings()
	if err == nil {
		autoUpdateIdx = indexOf(autoUpdateValues, string(globalSettings.AutoUpdate))
	}

	rows := []configRow{
		// Project section
		{label: "Agent", values: agentValues, current: agentIdx, section: "Project"},
		{label: "Model", values: modelValues, current: modelIdx},
		{label: "Worktree", values: worktreeValues, current: worktreeIdx},
		{label: "Sound", values: soundValues, current: soundIdx},
		{label: "  On task complete", values: taskCompleteValues, current: taskCompleteIdx},
		{label: "  On run complete", values: runCompleteValues, current: runCompleteIdx},
		{label: "  On error", values: errorValues, current: errorIdx},
		// Project actions
		{label: "Save project config", action: configActionSaveProject, isSave: true},
		{label: "Edit project file in editor", action: configActionEditProject},
		// Global section
		{label: "Auto-update", values: autoUpdateValues, current: autoUpdateIdx, section: "Global"},
		// Global actions
		{label: "Save global config", action: configActionSaveGlobal, isSave: true},
		{label: "Edit global file in editor", action: configActionEditGlobal},
	}

	// Find the auto-update row index for saveGlobalConfig
	globalAutoUpdateIdx := 0
	for i, r := range rows {
		if r.label == "Auto-update" && r.isOption() {
			globalAutoUpdateIdx = i
			break
		}
	}

	return configModel{
		rows:                rows,
		globalAutoUpdateIdx: globalAutoUpdateIdx,
	}
}

// optionByLabel finds the first option row with the given label.
func (m configModel) optionByLabel(label string) configRow {
	for _, r := range m.rows {
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

	return cfg
}

func (m configModel) saveGlobalConfig() error {
	row := m.rows[m.globalAutoUpdateIdx]
	mode := globalconfig.AutoUpdateMode(row.values[row.current])
	settings, err := loadGlobalSettings()
	if err != nil {
		settings = globalconfig.DefaultSettings()
	}
	settings.AutoUpdate = mode
	return saveGlobalSettings(settings)
}

func (m configModel) Init() tea.Cmd { return nil }

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		itemCount := len(m.rows)

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = itemCount - 1
			}
		case "down", "j":
			if m.cursor < itemCount-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "left", "h":
			if row := &m.rows[m.cursor]; row.isOption() {
				if row.current > 0 {
					row.current--
				} else {
					row.current = len(row.values) - 1
				}
			}
		case "right", "l":
			if row := &m.rows[m.cursor]; row.isOption() {
				if row.current < len(row.values)-1 {
					row.current++
				} else {
					row.current = 0
				}
			}
		case "enter":
			row := &m.rows[m.cursor]
			if row.isOption() {
				// Cycle value on enter
				if row.current < len(row.values)-1 {
					row.current++
				} else {
					row.current = 0
				}
			} else {
				m.action = row.action
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m configModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	activeValueStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	activeOffStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	saveStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)

	sectionPaths := map[string]string{
		"Project": ".maggus/config.yml",
		"Global":  "~/.maggus/config.yml",
	}

	var sb strings.Builder

	for i, row := range m.rows {
		// Section header
		if row.section != "" {
			if i > 0 {
				sb.WriteString("\n")
			}
			path := sectionPaths[row.section]
			sb.WriteString(titleStyle.Render(row.section) + "  " + mutedStyle.Render(path) + "\n")
			sb.WriteString(styles.Separator(50) + "\n")
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

	content := sb.String()
	footer := styles.StatusBar.Render("up/down: navigate | left/right: change value | enter: select | q/esc: cancel")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreen(content, footer, m.width, m.height)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
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

// openInEditor opens the given file in the user's preferred editor.
// It checks $VISUAL, $EDITOR, and falls back to platform defaults.
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
			editor = "vi"
		}
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func indexOf(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

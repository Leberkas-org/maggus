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
	if final.saved {
		newCfg := final.buildConfig()
		// Preserve fields not editable in the TUI.
		newCfg.Include = cfg.Include
		return saveConfig(dir, newCfg)
	}
	if final.openEditor {
		return openInEditor(filepath.Join(dir, ".maggus", "config.yml"))
	}
	return nil
}

// configOption represents a single editable setting.
type configOption struct {
	label   string
	key     string // config key for display
	values  []string
	current int
}

type configModel struct {
	options    []configOption
	cursor     int
	saved      bool
	openEditor bool
	width      int
	height     int
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

	return configModel{
		options: []configOption{
			{label: "Agent", key: "agent", values: agentValues, current: agentIdx},
			{label: "Model", key: "model", values: modelValues, current: modelIdx},
			{label: "Worktree", key: "worktree", values: worktreeValues, current: worktreeIdx},
			{label: "Sound", key: "notifications.sound", values: soundValues, current: soundIdx},
			{label: "  On task complete", key: "notifications.on_task_complete", values: taskCompleteValues, current: taskCompleteIdx},
			{label: "  On run complete", key: "notifications.on_run_complete", values: runCompleteValues, current: runCompleteIdx},
			{label: "  On error", key: "notifications.on_error", values: errorValues, current: errorIdx},
		},
	}
}

func (m configModel) buildConfig() config.Config {
	cfg := config.Config{
		Agent:    m.options[0].values[m.options[0].current],
		Worktree: m.options[2].values[m.options[2].current] == "on",
	}

	model := m.options[1].values[m.options[1].current]
	if model != "(default)" {
		cfg.Model = model
	}

	sound := m.options[3].values[m.options[3].current] == "on"
	cfg.Notifications.Sound = sound

	if m.options[4].values[m.options[4].current] == "off" {
		f := false
		cfg.Notifications.OnTaskComplete = &f
	}
	if m.options[5].values[m.options[5].current] == "off" {
		f := false
		cfg.Notifications.OnRunComplete = &f
	}
	if m.options[6].values[m.options[6].current] == "off" {
		f := false
		cfg.Notifications.OnError = &f
	}

	return cfg
}

func (m configModel) Init() tea.Cmd { return nil }

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		// Total items = options + Save + Edit file + Cancel
		itemCount := len(m.options) + 3

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
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
			if m.cursor < len(m.options) {
				opt := &m.options[m.cursor]
				if opt.current > 0 {
					opt.current--
				} else {
					opt.current = len(opt.values) - 1
				}
			}
		case "right", "l":
			if m.cursor < len(m.options) {
				opt := &m.options[m.cursor]
				if opt.current < len(opt.values)-1 {
					opt.current++
				} else {
					opt.current = 0
				}
			}
		case "enter":
			if m.cursor < len(m.options) {
				// Cycle value on enter
				opt := &m.options[m.cursor]
				if opt.current < len(opt.values)-1 {
					opt.current++
				} else {
					opt.current = 0
				}
			} else if m.cursor == len(m.options) {
				// Save
				m.saved = true
				return m, tea.Quit
			} else if m.cursor == len(m.options)+1 {
				// Edit file
				m.openEditor = true
				return m, tea.Quit
			} else {
				// Cancel
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

	header := titleStyle.Render("Configuration") + "  " + mutedStyle.Render(".maggus/config.yml")

	var sb strings.Builder
	sb.WriteString(header + "\n")
	sb.WriteString(styles.Separator(50) + "\n")

	for i, opt := range m.options {
		label := fmt.Sprintf("%-22s", opt.label)

		var valueParts []string
		for vi, v := range opt.values {
			if vi == opt.current {
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
	}

	sb.WriteString("\n")

	// Save button
	saveIdx := len(m.options)
	if m.cursor == saveIdx {
		fmt.Fprintf(&sb, "  %s %s\n", cursorStyle.Render("->"), saveStyle.Render("Save"))
	} else {
		fmt.Fprintf(&sb, "     %s\n", normalStyle.Render("Save"))
	}

	// Edit file button
	editIdx := saveIdx + 1
	if m.cursor == editIdx {
		fmt.Fprintf(&sb, "  %s %s\n", cursorStyle.Render("->"), normalStyle.Render("Edit file in editor"))
	} else {
		fmt.Fprintf(&sb, "     %s\n", mutedStyle.Render("Edit file in editor"))
	}

	// Cancel button
	cancelIdx := editIdx + 1
	if m.cursor == cancelIdx {
		fmt.Fprintf(&sb, "  %s %s\n", cursorStyle.Render("->"), normalStyle.Render("Cancel"))
	} else {
		fmt.Fprintf(&sb, "     %s\n", normalStyle.Render("Cancel"))
	}

	content := sb.String()
	footer := styles.StatusBar.Render("up/down: navigate | left/right: change value | enter: select | esc: cancel")

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

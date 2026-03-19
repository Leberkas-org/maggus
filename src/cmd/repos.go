package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/tui/filebrowser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/spf13/cobra"
)

// reposState represents the current state of the repos TUI.
type reposState int

const (
	reposStateList        reposState = iota // showing repo list
	reposStateBrowsing                      // file browser active
	reposStateConfirmInit                   // asking whether to initialize .maggus
)

// reposModel is the bubbletea model for the repository management screen.
type reposModel struct {
	repos      []globalconfig.Repository
	lastOpened string
	cwd        string // current working directory
	cursor     int
	state      reposState
	width      int
	height     int
	quitting   bool
	switched   bool // true if user switched repos (triggers menu reload)
	statusMsg  string

	// File browser sub-model (only active in browsing state)
	browser    filebrowser.Model
	pendingDir string // directory selected by browser, pending init confirmation

	// Injected for testing
	loadConfig func() (globalconfig.GlobalConfig, error)
	saveConfig func(globalconfig.GlobalConfig) error
	isGitRepo  func(string) bool
	chdir      func(string) error
}

func newReposModel() reposModel {
	cfg, _ := globalconfig.Load()
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd)

	return reposModel{
		repos:      cfg.Repositories,
		lastOpened: cfg.LastOpened,
		cwd:        cwd,
		loadConfig: globalconfig.Load,
		saveConfig: globalconfig.Save,
		isGitRepo:  isGitRepoCheck,
		chdir:      os.Chdir,
	}
}

func (m reposModel) Init() tea.Cmd {
	return nil
}

func (m reposModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == reposStateBrowsing {
			updated, cmd := m.browser.Update(msg)
			m.browser = updated.(filebrowser.Model)
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case reposStateList:
			return m.updateList(msg)
		case reposStateBrowsing:
			return m.updateBrowsing(msg)
		case reposStateConfirmInit:
			return m.updateConfirmInit(msg)
		}
	}
	return m, nil
}

func (m reposModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if len(m.repos) > 0 {
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.repos) - 1
			}
		}
	case "down", "j":
		if len(m.repos) > 0 {
			if m.cursor < len(m.repos)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		}
	case "home":
		m.cursor = 0
	case "end":
		if len(m.repos) > 0 {
			m.cursor = len(m.repos) - 1
		}
	case "enter":
		// Switch to selected repo
		if len(m.repos) > 0 {
			return m.switchToRepo(m.repos[m.cursor].Path)
		}
	case "a":
		// Add repo via file browser
		m.state = reposStateBrowsing
		m.browser = filebrowser.New(m.cwd)
		// Forward current size to browser
		updated, _ := m.browser.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		m.browser = updated.(filebrowser.Model)
		m.statusMsg = ""
		return m, nil
	case "d", "delete", "backspace":
		// Remove selected repo
		if len(m.repos) > 0 {
			return m.removeRepo()
		}
	}
	return m, nil
}

func (m reposModel) updateBrowsing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, cmd := m.browser.Update(msg)
	m.browser = updated.(filebrowser.Model)

	if m.browser.Cancelled() {
		m.state = reposStateList
		m.statusMsg = ""
		return m, nil
	}

	if sel := m.browser.Selected(); sel != "" {
		// Validate it's a git repo
		if !m.isGitRepo(sel) {
			m.state = reposStateList
			m.statusMsg = "Not a git repository"
			return m, nil
		}

		// Check if .maggus/ exists
		maggusDir := filepath.Join(sel, ".maggus")
		if info, err := os.Stat(maggusDir); err != nil || !info.IsDir() {
			// Ask whether to initialize
			m.pendingDir = sel
			m.state = reposStateConfirmInit
			return m, nil
		}

		// It's a git repo with .maggus — add it
		return m.addRepo(sel)
	}

	return m, cmd
}

func (m reposModel) updateConfirmInit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		// Initialize .maggus directory
		maggusDir := filepath.Join(m.pendingDir, ".maggus")
		if err := os.MkdirAll(maggusDir, 0o755); err != nil {
			m.state = reposStateList
			m.statusMsg = fmt.Sprintf("Failed to create .maggus: %v", err)
			return m, nil
		}
		return m.addRepo(m.pendingDir)
	case "n", "esc":
		// Add without initializing
		return m.addRepo(m.pendingDir)
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m reposModel) switchToRepo(path string) (tea.Model, tea.Cmd) {
	if err := m.chdir(path); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to switch: %v", err)
		return m, nil
	}

	// Update global config
	cfg, err := m.loadConfig()
	if err == nil {
		cfg.SetLastOpened(path)
		_ = m.saveConfig(cfg)
	}

	m.cwd = path
	m.switched = true
	m.quitting = true
	return m, tea.Quit
}

func (m reposModel) addRepo(path string) (tea.Model, tea.Cmd) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	cfg, err := m.loadConfig()
	if err != nil {
		m.state = reposStateList
		m.statusMsg = fmt.Sprintf("Failed to load config: %v", err)
		return m, nil
	}

	if !cfg.AddRepository(absPath) {
		m.state = reposStateList
		m.statusMsg = "Repository already configured"
		return m, nil
	}

	if err := m.saveConfig(cfg); err != nil {
		m.state = reposStateList
		m.statusMsg = fmt.Sprintf("Failed to save: %v", err)
		return m, nil
	}

	// Refresh the repo list
	m.repos = cfg.Repositories
	m.lastOpened = cfg.LastOpened
	m.cursor = len(m.repos) - 1 // move cursor to newly added
	m.state = reposStateList
	m.statusMsg = fmt.Sprintf("Added %s", filepath.Base(absPath))
	return m, nil
}

func (m reposModel) removeRepo() (tea.Model, tea.Cmd) {
	path := m.repos[m.cursor].Path

	cfg, err := m.loadConfig()
	if err != nil {
		m.statusMsg = fmt.Sprintf("Failed to load config: %v", err)
		return m, nil
	}

	cfg.RemoveRepository(path)
	if err := m.saveConfig(cfg); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to save: %v", err)
		return m, nil
	}

	m.repos = cfg.Repositories
	m.lastOpened = cfg.LastOpened
	if m.cursor >= len(m.repos) && m.cursor > 0 {
		m.cursor = len(m.repos) - 1
	}
	m.statusMsg = fmt.Sprintf("Removed %s", filepath.Base(path))
	return m, nil
}

func (m reposModel) View() string {
	switch m.state {
	case reposStateBrowsing:
		return m.browser.View()
	case reposStateConfirmInit:
		return m.viewConfirmInit()
	default:
		return m.viewList()
	}
}

func (m reposModel) viewList() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	activeStyle := lipgloss.NewStyle().Foreground(styles.Success)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()

	var b strings.Builder

	b.WriteString(titleStyle.Render("Repositories"))
	b.WriteString("\n")

	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 20 {
		innerW = 60
	}
	b.WriteString(styles.Separator(innerW))
	b.WriteString("\n")

	if len(m.repos) == 0 {
		b.WriteString(mutedStyle.Render("No repositories configured. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, repo := range m.repos {
			isActive := repo.Path == m.cwd
			prefix := "  "
			if i == m.cursor {
				prefix = "→ "
			}

			displayPath := repo.Path
			label := filepath.Base(repo.Path)

			var line string
			if i == m.cursor {
				if isActive {
					line = selectedStyle.Render(prefix+label) + activeStyle.Render(" ●") + mutedStyle.Render("  "+displayPath)
				} else {
					line = selectedStyle.Render(prefix+label) + mutedStyle.Render("  "+displayPath)
				}
			} else {
				if isActive {
					line = normalStyle.Render(prefix+label) + activeStyle.Render(" ●") + mutedStyle.Render("  "+displayPath)
				} else {
					line = normalStyle.Render(prefix+label) + mutedStyle.Render("  "+displayPath)
				}
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	content := b.String()
	footer := styles.StatusBar.Render("↑/↓ navigate · enter switch · a add · d remove · esc back")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(content, footer, m.width, m.height)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
}

func (m reposModel) viewConfirmInit() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	warningStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Initialize Repository"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "The directory %s is a git repo but has no .maggus/ directory.\n\n", warningStyle.Render(m.pendingDir))
	b.WriteString("Initialize it? ")
	b.WriteString(mutedStyle.Render("(y/n)"))
	b.WriteString("\n")

	content := b.String()
	footer := styles.StatusBar.Render("y: initialize · n: add without initializing · esc: cancel")

	if m.width > 0 && m.height > 0 {
		return styles.FullScreenLeft(content, footer, m.width, m.height)
	}
	return styles.Box.Render(content+"\n\n"+footer) + "\n"
}

// isGitRepoCheck checks if a directory is inside a git work tree.
func isGitRepoCheck(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// runRepos is the cobra RunE for the repos command. It is a package-level var
// so it can be replaced in tests.
var runRepos = func(_ *cobra.Command, _ []string) error {
	m := newReposModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}
	_ = result.(reposModel)
	return nil
}

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage configured repositories",
	RunE:  func(cmd *cobra.Command, args []string) error { return runRepos(cmd, args) },
}

func init() {
	rootCmd.AddCommand(reposCmd)
}

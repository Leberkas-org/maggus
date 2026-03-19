// Package filebrowser provides a bubbletea model for browsing and selecting
// directories in the filesystem.
package filebrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// dirEntry represents a single directory in the listing.
type dirEntry struct {
	name   string
	hidden bool // starts with "."
}

// Model is the bubbletea model for the directory browser.
type Model struct {
	currentDir   string
	entries      []dirEntry
	cursor       int
	scrollOffset int
	selected     string // confirmed directory path
	cancelled    bool
	errMsg       string // inline error message
	width        int
	height       int

	// readDir is injectable for testing.
	readDir func(string) ([]os.DirEntry, error)
}

// New creates a new file browser model starting at the given directory.
func New(startDir string) Model {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		abs = startDir
	}
	m := Model{
		currentDir: abs,
		readDir:    os.ReadDir,
	}
	m.loadEntries()
	return m
}

// Selected returns the confirmed directory path, or empty if none was selected.
func (m Model) Selected() string {
	return m.selected
}

// Cancelled returns true if the user pressed esc to cancel.
func (m Model) Cancelled() bool {
	return m.cancelled
}

// loadEntries reads the current directory and populates the entries list.
func (m *Model) loadEntries() {
	m.errMsg = ""
	m.entries = nil

	// Add parent directory entry if not at root.
	if parent := filepath.Dir(m.currentDir); parent != m.currentDir {
		m.entries = append(m.entries, dirEntry{name: ".."})
	}

	items, err := m.readDir(m.currentDir)
	if err != nil {
		m.errMsg = fmt.Sprintf("Error: %s", err.Error())
		return
	}

	var dirs []dirEntry
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		dirs = append(dirs, dirEntry{
			name:   item.Name(),
			hidden: strings.HasPrefix(item.Name(), "."),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})

	m.entries = append(m.entries, dirs...)
	m.cursor = 0
	m.scrollOffset = 0
}

// navigate changes into the selected directory.
func (m *Model) navigate(name string) {
	if name == ".." {
		m.currentDir = filepath.Dir(m.currentDir)
	} else {
		m.currentDir = filepath.Join(m.currentDir, name)
	}
	m.loadEntries()
}

// visibleLines returns the number of directory lines that fit in the viewport.
// Reserves lines for: path header, separator, footer, error (if any).
func (m Model) visibleLines() int {
	reserved := 4 // path + separator + blank + footer
	if m.errMsg != "" {
		reserved++
	}
	v := m.height - reserved
	if v < 1 {
		v = 1
	}
	return v
}

// ensureCursorVisible adjusts scrollOffset so the cursor stays visible.
func (m *Model) ensureCursorVisible() {
	vis := m.visibleLines()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+vis {
		m.scrollOffset = m.cursor - vis + 1
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}

		case tea.KeyDown:
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}

		case tea.KeyEnter:
			if len(m.entries) > 0 {
				m.navigate(m.entries[m.cursor].name)
			}

		case tea.KeyBackspace:
			if parent := filepath.Dir(m.currentDir); parent != m.currentDir {
				m.navigate("..")
			}

		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "s":
				m.selected = m.currentDir
				return m, tea.Quit
			case "q":
				m.cancelled = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	if innerW < 10 {
		innerW = 40
	}

	var b strings.Builder

	// Path header.
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	b.WriteString(pathStyle.Render(styles.Truncate(m.currentDir, innerW)))
	b.WriteString("\n")
	b.WriteString(styles.Separator(innerW))
	b.WriteString("\n")

	// Error message if any.
	if m.errMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(styles.Error)
		b.WriteString(errStyle.Render(m.errMsg))
		b.WriteString("\n")
	}

	// Directory listing with scrolling.
	vis := m.visibleLines()
	end := m.scrollOffset + vis
	if end > len(m.entries) {
		end = len(m.entries)
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	folderIcon := "📁 "

	for i := m.scrollOffset; i < end; i++ {
		entry := m.entries[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "→ "
		}

		name := entry.name
		if entry.name == ".." {
			name = ".."
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(prefix + name))
			} else {
				b.WriteString(mutedStyle.Render(prefix + name))
			}
		} else if entry.hidden {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(prefix) + mutedStyle.Render(folderIcon+name))
			} else {
				b.WriteString(mutedStyle.Render(prefix + folderIcon + name))
			}
		} else {
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(prefix+folderIcon+name))
			} else {
				b.WriteString(normalStyle.Render(prefix + folderIcon + name))
			}
		}
		b.WriteString("\n")
	}

	// Scroll indicator.
	if len(m.entries) > vis {
		indicator := fmt.Sprintf("[%d-%d of %d]", m.scrollOffset+1, end, len(m.entries))
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Muted).Render(indicator))
		b.WriteString("\n")
	}

	content := b.String()

	// Footer.
	footerStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	footer := footerStyle.Render("↑/↓ navigate · enter open · backspace up · s select · esc cancel")

	return styles.FullScreenLeft(content, footer, m.width, m.height)
}

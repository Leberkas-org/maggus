package component

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type DirEntry struct {
	Name  string
	Path  string
	IsDir bool
	IsGit bool
}

type FileBrowser struct {
	Dir           string
	RootDir       string // if set, can't navigate above this directory
	Entries       []DirEntry
	Cursor        int
	Offset        int
	Width, Height int
	FileFilter    string // e.g. ".md" — show files with this extension (empty = dirs only)
	Title         string
	err           string
}

func NewFileBrowser() *FileBrowser {
	fb := &FileBrowser{Title: "Select Repository"}
	start, err := os.UserHomeDir()
	if err != nil {
		start = "/"
		if runtime.GOOS == "windows" {
			start = "C:\\"
		}
	}
	fb.Navigate(start)
	return fb
}

func NewFileBrowserAt(dir, title, fileFilter string) *FileBrowser {
	fb := &FileBrowser{Title: title, FileFilter: fileFilter}
	fb.Navigate(dir)
	return fb
}

func NewFileBrowserScoped(rootDir, startDir, title, fileFilter string) *FileBrowser {
	abs, _ := filepath.Abs(rootDir)
	fb := &FileBrowser{Title: title, FileFilter: fileFilter, RootDir: abs}
	fb.Navigate(startDir)
	return fb
}

func (fb *FileBrowser) Navigate(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fb.err = err.Error()
		return
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		fb.err = err.Error()
		return
	}

	fb.err = ""
	fb.Dir = abs
	fb.Cursor = 0
	fb.Offset = 0
	fb.Entries = nil

	parent := filepath.Dir(abs)
	atRoot := parent == abs || (fb.RootDir != "" && abs == fb.RootDir)
	if !atRoot {
		fb.Entries = append(fb.Entries, DirEntry{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		})
	}

	var dirs []DirEntry
	var files []DirEntry
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if strings.HasPrefix(name, ".") && name != ".git" {
				continue
			}
			full := filepath.Join(abs, name)
			dirs = append(dirs, DirEntry{
				Name:  name,
				Path:  full,
				IsDir: true,
				IsGit: isGitRepo(full),
			})
		} else if fb.FileFilter != "" && strings.HasSuffix(strings.ToLower(name), fb.FileFilter) {
			files = append(files, DirEntry{
				Name:  name,
				Path:  filepath.Join(abs, name),
				IsDir: false,
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsGit != dirs[j].IsGit {
			return dirs[i].IsGit
		}
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	fb.Entries = append(fb.Entries, dirs...)
	fb.Entries = append(fb.Entries, files...)
}

func (fb *FileBrowser) MoveUp() {
	if fb.Cursor > 0 {
		fb.Cursor--
		fb.ensureVisible()
	}
}

func (fb *FileBrowser) MoveDown() {
	if fb.Cursor < len(fb.Entries)-1 {
		fb.Cursor++
		fb.ensureVisible()
	}
}

func (fb *FileBrowser) Enter() {
	if fb.Cursor >= len(fb.Entries) {
		return
	}
	entry := fb.Entries[fb.Cursor]
	if entry.IsDir {
		fb.Navigate(entry.Path)
	}
}

func (fb *FileBrowser) GoUp() {
	if fb.RootDir != "" && fb.Dir == fb.RootDir {
		return
	}
	parent := filepath.Dir(fb.Dir)
	if parent != fb.Dir {
		fb.Navigate(parent)
	}
}

func (fb *FileBrowser) Selected() *DirEntry {
	if fb.Cursor >= len(fb.Entries) {
		return nil
	}
	return &fb.Entries[fb.Cursor]
}

func (fb *FileBrowser) SelectedIsGit() bool {
	sel := fb.Selected()
	return sel != nil && sel.IsGit
}

func (fb *FileBrowser) SelectedIsFile() bool {
	sel := fb.Selected()
	return sel != nil && !sel.IsDir
}

func (fb *FileBrowser) CurrentDirIsGit() bool {
	return isGitRepo(fb.Dir)
}

// boxWidth returns the outer box width (capped at 80, min 30).
func (fb *FileBrowser) boxWidth() int {
	return max(min(fb.Width-4, 80), 30)
}

// innerWidth returns usable content width inside the box
// (box width minus border(2) minus padding(2)).
func (fb *FileBrowser) innerWidth() int {
	return max(fb.boxWidth()-4, 10)
}

// innerHeight returns usable content height inside the box
// (terminal height minus border(2) minus padding(2) minus centering slack).
func (fb *FileBrowser) innerHeight() int {
	return max(fb.Height-6, 6)
}

// listHeight returns how many directory rows fit between header and footer.
// header = path(1) + separator(1), footer = separator(1) + hints(1) → 4 fixed lines.
func (fb *FileBrowser) listHeight() int {
	return max(fb.innerHeight()-4, 1)
}

func (fb *FileBrowser) ensureVisible() {
	listH := fb.listHeight()
	if fb.Cursor < fb.Offset {
		fb.Offset = fb.Cursor
	}
	if listH > 0 && fb.Cursor >= fb.Offset+listH {
		fb.Offset = fb.Cursor - listH + 1
	}
}

func (fb *FileBrowser) View() string {
	iw := fb.innerWidth()

	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	gitBadge := lipgloss.NewStyle().Bold(true).Foreground(styles.Success)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)
	normalStyle := lipgloss.NewStyle()
	selectedBg := lipgloss.NewStyle().
		Background(styles.Primary).
		Foreground(lipgloss.Color("0"))

	var lines []string

	pathLine := pathStyle.Render(styles.Truncate(fb.Dir, iw-6))
	if fb.CurrentDirIsGit() {
		pathLine += " " + gitBadge.Render("[git]")
	}
	lines = append(lines, pathLine)
	lines = append(lines, styles.Separator(iw))

	if fb.err != "" {
		lines = append(lines, errStyle.Render(fb.err))
		return fb.wrapInBox(strings.Join(lines, "\n"))
	}

	listH := fb.listHeight()
	end := min(fb.Offset+listH, len(fb.Entries))

	fileStyle := lipgloss.NewStyle().Foreground(styles.Accent)

	for i := fb.Offset; i < end; i++ {
		entry := fb.Entries[i]
		icon := "  "
		if entry.Name == ".." {
			icon = "  "
		} else if entry.IsGit {
			icon = "  "
		} else if !entry.IsDir {
			icon = "  "
		}

		label := icon + entry.Name
		if entry.IsGit {
			label += " [git]"
		}
		label = styles.Truncate(label, iw)

		if i == fb.Cursor {
			lines = append(lines, selectedBg.Width(iw).Render(label))
		} else if !entry.IsDir {
			lines = append(lines, fileStyle.Width(iw).Render(label))
		} else if entry.IsGit {
			lines = append(lines, gitBadge.Width(iw).Render(label))
		} else {
			lines = append(lines, normalStyle.Width(iw).Render(label))
		}
	}

	// Pad remaining space
	totalContentLines := 2 + listH + 2 // header(2) + list + footer(2)
	for len(lines) < totalContentLines-2 {
		lines = append(lines, "")
	}

	lines = append(lines, styles.Separator(iw))
	hint := "Enter: open  Backspace: up  "
	if fb.SelectedIsFile() {
		hint = "Enter: select file  Backspace: up  "
	} else if fb.SelectedIsGit() {
		hint += "Enter: select repo  "
	}
	if fb.CurrentDirIsGit() {
		hint += "s: select this dir  "
	}
	hint += "Esc: cancel"
	lines = append(lines, hintStyle.Render(styles.Truncate(hint, iw)))

	return fb.wrapInBox(strings.Join(lines, "\n"))
}

func (fb *FileBrowser) wrapInBox(content string) string {
	bw := fb.boxWidth()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 1).
		Width(bw)

	return box.Render(content)
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

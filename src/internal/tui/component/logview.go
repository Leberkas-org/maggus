package component

import (
	"bufio"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/logging"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type LogView struct {
	Path          string
	Lines         []string
	Width, Height int
	mu            sync.Mutex
	lastSize      int64
}

func NewLogView(path string) *LogView {
	lv := &LogView{Path: path}
	lv.Refresh()
	return lv
}

func (lv *LogView) Refresh() {
	lv.mu.Lock()
	defer lv.mu.Unlock()

	info, err := os.Stat(lv.Path)
	if err != nil {
		return
	}
	if info.Size() == lv.lastSize {
		return
	}
	lv.lastSize = info.Size()

	f, err := os.Open(lv.Path)
	if err != nil {
		return
	}
	defer f.Close()

	// Read all lines, keep last N
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	maxLines := 200
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	lv.Lines = lines
}

func (lv *LogView) View() string {
	lv.mu.Lock()
	defer lv.mu.Unlock()

	if lv.Height <= 0 || lv.Width <= 0 {
		return ""
	}

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Muted)
	dbgStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	infStyle := lipgloss.NewStyle().Foreground(styles.Success)
	wrnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)

	header := labelStyle.Render(" Log")
	sep := styles.Separator(lv.Width)
	viewH := lv.Height - 3 // header + top separator + bottom separator

	// Get last viewH lines
	start := 0
	if len(lv.Lines) > viewH {
		start = len(lv.Lines) - viewH
	}
	visible := lv.Lines[start:]

	var rendered []string
	for _, line := range visible {
		formatted := logging.FormatSlogLine(line)
		truncated := styles.Truncate(formatted, lv.Width)

		var s lipgloss.Style
		if strings.Contains(formatted, "[ERR]") {
			s = errStyle
		} else if strings.Contains(formatted, "[WRN]") {
			s = wrnStyle
		} else if strings.Contains(formatted, "[DBG]") {
			s = dbgStyle
		} else {
			s = infStyle
		}
		rendered = append(rendered, s.Render(truncated))
	}
	for len(rendered) < viewH {
		rendered = append(rendered, "")
	}

	content := lipgloss.NewStyle().
		Width(lv.Width).
		Height(viewH).
		Render(strings.Join(rendered, "\n"))

	return header + "\n" + sep + "\n" + content + "\n" + sep
}

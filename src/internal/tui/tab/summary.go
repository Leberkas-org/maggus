package tab

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type SummaryTab struct {
	data *SummaryData
}

type SummaryData struct {
	Status     string
	Duration   string
	CommitHash string
	TotalCost  float64
	Tasks      int
	Done       int
}

func NewSummaryTab() *SummaryTab {
	return &SummaryTab{}
}

func (t *SummaryTab) Name() string { return "Summary" }

func (t *SummaryTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	return t, nil
}

func (t *SummaryTab) View(width, height int) string {
	if t.data == nil {
		return lipgloss.NewStyle().Foreground(styles.Muted).Render("  No summary available")
	}

	d := t.data
	var lines []string

	statusStyle := styles.Title
	if d.Status == "failed" {
		statusStyle = lipgloss.NewStyle().Bold(true).Foreground(styles.Error)
	}

	lines = append(lines, statusStyle.Render(fmt.Sprintf("  %s", strings.ToUpper(d.Status))))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Tasks:     %d/%d", d.Done, d.Tasks))
	lines = append(lines, fmt.Sprintf("  Duration:  %s", d.Duration))
	if d.CommitHash != "" {
		lines = append(lines, fmt.Sprintf("  Commit:    %s", d.CommitHash))
	}
	lines = append(lines, fmt.Sprintf("  Cost:      $%.4f", d.TotalCost))

	return lipgloss.NewStyle().Width(width).Height(height).
		Render(strings.Join(lines, "\n"))
}

func (t *SummaryTab) SetData(data any) {
	if s, ok := data.(*SummaryData); ok {
		t.data = s
	}
}

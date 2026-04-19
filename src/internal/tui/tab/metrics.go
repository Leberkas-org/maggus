package tab

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type MetricsTab struct {
	data *MetricsData
}

type MetricsData struct {
	Usage     ipc.TokenUsage
	StartedAt time.Time
	Status    string
}

func NewMetricsTab() *MetricsTab {
	return &MetricsTab{}
}

func (t *MetricsTab) Name() string { return "Metrics" }

func (t *MetricsTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	return t, nil
}

func (t *MetricsTab) View(width, height int) string {
	if t.data == nil {
		return lipgloss.NewStyle().Foreground(styles.Muted).Render("  No metrics available")
	}

	var lines []string
	d := t.data

	lines = append(lines, styles.Title.Render("  Token Usage"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Input:          %d", d.Usage.InputTokens))
	lines = append(lines, fmt.Sprintf("  Output:         %d", d.Usage.OutputTokens))
	lines = append(lines, fmt.Sprintf("  Cache Read:     %d", d.Usage.CacheReadTokens))
	lines = append(lines, fmt.Sprintf("  Cache Create:   %d", d.Usage.CacheCreateTokens))
	lines = append(lines, fmt.Sprintf("  Cost:           $%.4f", d.Usage.CostUSD))
	lines = append(lines, "")

	if !d.StartedAt.IsZero() {
		elapsed := time.Since(d.StartedAt).Round(time.Second)
		lines = append(lines, fmt.Sprintf("  Duration:       %s", elapsed))
	}

	return lipgloss.NewStyle().Width(width).Height(height).
		Render(strings.Join(lines, "\n"))
}

func (t *MetricsTab) SetData(data any) {
	if m, ok := data.(*MetricsData); ok {
		t.data = m
	}
}

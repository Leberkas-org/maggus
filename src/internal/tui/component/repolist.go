package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type RepoList struct {
	Repos         []config.RepoEntry
	Cursor        int
	Offset        int
	Width, Height int
}

func NewRepoList() *RepoList {
	rl := &RepoList{}
	rl.Reload()
	return rl
}

func (rl *RepoList) Reload() {
	repos, _ := config.ListRepos()
	rl.Repos = repos
	if rl.Cursor >= len(rl.Repos) {
		rl.Cursor = max(len(rl.Repos)-1, 0)
	}
}

func (rl *RepoList) MoveUp() {
	if rl.Cursor > 0 {
		rl.Cursor--
		rl.ensureVisible()
	}
}

func (rl *RepoList) MoveDown() {
	if rl.Cursor < len(rl.Repos)-1 {
		rl.Cursor++
		rl.ensureVisible()
	}
}

func (rl *RepoList) Selected() *config.RepoEntry {
	if rl.Cursor < len(rl.Repos) {
		return &rl.Repos[rl.Cursor]
	}
	return nil
}

func (rl *RepoList) listHeight() int {
	ih := max(rl.Height-6, 6)
	return max(ih-4, 1) // title(1) + blank(1) + separator(1) + hints(1)
}

func (rl *RepoList) ensureVisible() {
	lh := rl.listHeight()
	if rl.Cursor < rl.Offset {
		rl.Offset = rl.Cursor
	}
	if lh > 0 && rl.Cursor >= rl.Offset+lh {
		rl.Offset = rl.Cursor - lh + 1
	}
}

func (rl *RepoList) View() string {
	bw := max(min(rl.Width-4, 80), 30)
	iw := max(bw-4, 10)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	normalStyle := lipgloss.NewStyle()
	selectedBg := lipgloss.NewStyle().
		Background(styles.Primary).
		Foreground(lipgloss.Color("0"))

	var lines []string
	lines = append(lines, titleStyle.Render("Repositories"))
	lines = append(lines, styles.Separator(iw))

	if len(rl.Repos) == 0 {
		empty := lipgloss.NewStyle().Foreground(styles.Muted)
		lines = append(lines, "")
		lines = append(lines, empty.Render("  No repositories registered."))
		lines = append(lines, empty.Render("  Press 'a' to add one."))
	} else {
		lh := rl.listHeight()
		end := min(rl.Offset+lh, len(rl.Repos))

		for i := rl.Offset; i < end; i++ {
			repo := rl.Repos[i]
			label := "  " + repo.Path
			if repo.URL != "" {
				label += "  " + hintStyle.Render("("+repo.URL+")")
			}

			if i == rl.Cursor {
				selLabel := "  " + repo.Path
				lines = append(lines, selectedBg.Width(iw).Render(styles.Truncate(selLabel, iw)))
			} else {
				lines = append(lines, normalStyle.Width(iw).Render(styles.Truncate(label, iw)))
			}
		}
	}

	// Pad
	targetLines := rl.listHeight() + 2 // title + separator
	for len(lines) < targetLines {
		lines = append(lines, "")
	}

	lines = append(lines, styles.Separator(iw))
	lines = append(lines, hintStyle.Render("Enter: switch  a: add  d: remove  Esc: back"))

	content := strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 1).
		Width(bw)

	return box.Render(content)
}

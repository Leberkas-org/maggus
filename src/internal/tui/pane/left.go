package pane

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/tui/component"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type LeftPane struct {
	BasePane
	tree *component.Tree
}

func NewLeftPane() *LeftPane {
	return &LeftPane{
		tree: component.NewTree(),
	}
}

func (p *LeftPane) Init() tea.Cmd { return nil }

func (p *LeftPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if !p.Focused {
		return p, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			p.tree.MoveUp()
		case "down", "j":
			p.tree.MoveDown()
		case "enter", "right", "l":
			p.tree.Expand()
		case "left", "h":
			p.tree.Collapse()
		case " ":
			p.tree.Toggle()
		}
	}
	return p, nil
}

func (p *LeftPane) View() string {
	p.tree.Width = p.Width - 2
	p.tree.Height = p.Height - 1

	treeView := p.tree.View()
	sep := styles.Separator(p.Width - 2)

	content := lipgloss.NewStyle().
		Width(p.Width - 2).
		Height(p.Height - 1).
		Render(treeView)

	left := content + "\n" + sep

	divStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	divider := divStyle.Render(strings.Repeat("│\n", p.Height-1) + "┴")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, divider)
}

func (p *LeftPane) Selected() *component.TreeNode {
	return p.tree.Selected()
}

func (p *LeftPane) UpdateState(snap ipc.DaemonSnapshot) {
	var nodes []*component.TreeNode

	repoMap := make(map[string]*component.TreeNode)
	for _, q := range snap.Queue {
		repoNode, ok := repoMap[q.RepoURL]
		if !ok {
			repoNode = &component.TreeNode{
				ID:       q.RepoURL,
				Label:    q.RepoURL,
				Expanded: true,
			}
			repoMap[q.RepoURL] = repoNode
			nodes = append(nodes, repoNode)
		}

		itemIcon := statusIcon(q.Status)
		itemNode := &component.TreeNode{
			ID:       q.ID,
			Label:    q.Title,
			Icon:     itemIcon,
			Expanded: q.Status == "active",
		}
		repoNode.Children = append(repoNode.Children, itemNode)
	}

	for _, w := range snap.Workers {
		if repoNode, ok := repoMap[w.RepoURL]; ok {
			for _, child := range repoNode.Children {
				if child.ID == w.ItemID {
					child.Children = append(child.Children, &component.TreeNode{
						ID:    w.TaskID,
						Label: w.TaskTitle,
						Icon:  statusIcon(w.Status),
					})
				}
			}
		}
	}

	p.tree.SetNodes(nodes)
}

func statusIcon(status string) string {
	switch status {
	case "done", "completed":
		return "✓"
	case "active", "running", "running agent":
		return "→"
	case "ready":
		return "○"
	case "pending":
		return "⏳"
	case "failed":
		return "⚠"
	case "skipped":
		return "⏭"
	default:
		return "○"
	}
}

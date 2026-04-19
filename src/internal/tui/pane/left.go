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
	contentW := p.Width - 1 // 1 column for divider
	p.tree.Width = contentW
	p.tree.Height = p.Height - 1

	treeView := p.tree.View()
	sep := styles.Separator(contentW)

	content := lipgloss.NewStyle().
		Width(contentW).
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

func (p *LeftPane) SelectedItemID() string {
	sel := p.tree.Selected()
	if sel == nil {
		return ""
	}
	// If it's a top-level node (work item), return its ID
	for _, node := range p.tree.Nodes {
		if node.ID == sel.ID {
			return node.ID
		}
		// Check if selected is a child (task) of this item
		for _, child := range node.Children {
			if child.ID == sel.ID {
				return node.ID
			}
		}
	}
	return sel.ID
}

func (p *LeftPane) UpdateState(snap ipc.DaemonSnapshot) {
	// Don't replace existing tree with empty data
	if len(snap.Queue) == 0 && len(p.tree.Nodes) > 0 {
		return
	}

	var nodes []*component.TreeNode

	for _, q := range snap.Queue {
		itemNode := &component.TreeNode{
			ID:       q.ID,
			Label:    q.Title,
			Icon:     statusIcon(q.Status),
			Expanded: true,
		}

		for _, t := range q.TaskList {
			taskIcon := statusIcon(t.Status)
			for _, w := range snap.Workers {
				if w.TaskID == t.ID {
					taskIcon = statusIcon(w.Status)
					break
				}
			}
			itemNode.Children = append(itemNode.Children, &component.TreeNode{
				ID:    t.ID,
				Label: t.Title,
				Icon:  taskIcon,
			})
		}

		nodes = append(nodes, itemNode)
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

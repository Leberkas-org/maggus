package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type TreeNode struct {
	ID       string
	Label    string
	Icon     string
	Children []*TreeNode
	Expanded bool
}

type Tree struct {
	Nodes         []*TreeNode
	Cursor        int
	Offset        int
	Width, Height int
	flat          []*flatNode
}

type flatNode struct {
	node  *TreeNode
	depth int
}

func NewTree() *Tree {
	return &Tree{}
}

func (t *Tree) SetNodes(nodes []*TreeNode) {
	t.Nodes = nodes
	t.flatten()
}

func (t *Tree) flatten() {
	t.flat = nil
	for _, n := range t.Nodes {
		t.flattenNode(n, 0)
	}
}

func (t *Tree) flattenNode(n *TreeNode, depth int) {
	t.flat = append(t.flat, &flatNode{node: n, depth: depth})
	if n.Expanded {
		for _, child := range n.Children {
			t.flattenNode(child, depth+1)
		}
	}
}

func (t *Tree) MoveUp() {
	if t.Cursor > 0 {
		t.Cursor--
		t.ensureVisible()
	}
}

func (t *Tree) MoveDown() {
	if t.Cursor < len(t.flat)-1 {
		t.Cursor++
		t.ensureVisible()
	}
}

func (t *Tree) Toggle() {
	if t.Cursor >= len(t.flat) {
		return
	}
	node := t.flat[t.Cursor].node
	if len(node.Children) > 0 {
		node.Expanded = !node.Expanded
		t.flatten()
	}
}

func (t *Tree) Expand() {
	if t.Cursor >= len(t.flat) {
		return
	}
	node := t.flat[t.Cursor].node
	if len(node.Children) > 0 {
		node.Expanded = true
		t.flatten()
	}
}

func (t *Tree) Collapse() {
	if t.Cursor >= len(t.flat) {
		return
	}
	node := t.flat[t.Cursor].node
	if node.Expanded {
		node.Expanded = false
		t.flatten()
	}
}

func (t *Tree) Selected() *TreeNode {
	if t.Cursor >= len(t.flat) {
		return nil
	}
	return t.flat[t.Cursor].node
}

func (t *Tree) ensureVisible() {
	if t.Cursor < t.Offset {
		t.Offset = t.Cursor
	}
	if t.Height > 0 && t.Cursor >= t.Offset+t.Height {
		t.Offset = t.Cursor - t.Height + 1
	}
}

func (t *Tree) View() string {
	if len(t.flat) == 0 {
		return lipgloss.NewStyle().Foreground(styles.Muted).Render("  No items")
	}

	var lines []string
	end := min(t.Offset+t.Height, len(t.flat))

	for i := t.Offset; i < end; i++ {
		fn := t.flat[i]
		indent := strings.Repeat("  ", fn.depth)
		prefix := "  "
		if len(fn.node.Children) > 0 {
			if fn.node.Expanded {
				prefix = "▼ "
			} else {
				prefix = "▶ "
			}
		}

		icon := ""
		if fn.node.Icon != "" {
			icon = fn.node.Icon + " "
		}

		label := indent + prefix + icon + fn.node.Label
		label = styles.Truncate(label, t.Width)

		if i == t.Cursor {
			line := lipgloss.NewStyle().
				Background(styles.Primary).
				Foreground(lipgloss.Color("0")).
				Width(t.Width).
				Render(label)
			lines = append(lines, line)
		} else {
			lines = append(lines, lipgloss.NewStyle().Width(t.Width).Render(label))
		}
	}

	return strings.Join(lines, "\n")
}

package pane

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/leberkas-org/maggus/internal/tui/tab"
)

type RightPane struct {
	BasePane
	tabs      []tab.Tab
	activeTab int
}

func NewRightPane() *RightPane {
	return &RightPane{
		tabs: []tab.Tab{
			tab.NewPlanTab(),
			tab.NewOutputTab(),
			tab.NewLogTab(),
			tab.NewMetricsTab(),
			tab.NewSummaryTab(),
		},
	}
}

func (p *RightPane) Init() tea.Cmd { return nil }

func (p *RightPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if !p.Focused {
		return p, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "1":
			p.activeTab = 0
		case "2":
			if len(p.tabs) > 1 {
				p.activeTab = 1
			}
		case "3":
			if len(p.tabs) > 2 {
				p.activeTab = 2
			}
		case "4":
			if len(p.tabs) > 3 {
				p.activeTab = 3
			}
		case "5":
			if len(p.tabs) > 4 {
				p.activeTab = 4
			}
		default:
			if p.activeTab < len(p.tabs) {
				updated, cmd := p.tabs[p.activeTab].Update(msg)
				p.tabs[p.activeTab] = updated
				return p, cmd
			}
		}
	}

	return p, nil
}

func (p *RightPane) View() string {
	tabBar := p.renderTabBar()
	contentH := p.Height - 2 // tab bar + separator

	var content string
	if p.activeTab < len(p.tabs) {
		content = p.tabs[p.activeTab].View(p.Width, contentH)
	}

	return tabBar + "\n" + content + "\n" + styles.Separator(p.Width)
}

func (p *RightPane) renderTabBar() string {
	var parts []string
	for i, t := range p.tabs {
		name := t.Name()
		if i == p.activeTab {
			style := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
			parts = append(parts, style.Render("["+name+"]"))
		} else {
			style := lipgloss.NewStyle().Foreground(styles.Muted)
			parts = append(parts, style.Render(" "+name+" "))
		}
	}
	return " " + strings.Join(parts, " ")
}

func (p *RightPane) SetTabs(tabs []tab.Tab) {
	p.tabs = tabs
	if p.activeTab >= len(tabs) {
		p.activeTab = 0
	}
}

func (p *RightPane) ActiveTab() tab.Tab {
	if p.activeTab < len(p.tabs) {
		return p.tabs[p.activeTab]
	}
	return nil
}

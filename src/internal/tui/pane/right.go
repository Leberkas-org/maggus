package pane

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/tui/component"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	"github.com/leberkas-org/maggus/internal/tui/tab"
)

type SelectionContext string

const (
	CtxNone         SelectionContext = ""
	CtxItemPending  SelectionContext = "pending"
	CtxItemActive   SelectionContext = "active"
	CtxItemDone     SelectionContext = "done"
	CtxTaskRunning  SelectionContext = "task_running"
	CtxTaskComplete SelectionContext = "task_complete"
)

const logViewHeight = 9

type RightPane struct {
	BasePane
	tabs      []tab.Tab
	activeTab int
	context   SelectionContext
	logView   *component.LogView
}

func NewRightPane() *RightPane {
	var logPath string
	if globalDir, err := config.GlobalDir(); err == nil {
		logPath = filepath.Join(globalDir, "daemon.log")
	}
	return &RightPane{
		tabs:    tabsForContext(CtxNone),
		logView: component.NewLogView(logPath),
	}
}

func (p *RightPane) SetContext(ctx SelectionContext) {
	if ctx == p.context {
		return
	}
	p.context = ctx
	p.tabs = tabsForContext(ctx)
	p.activeTab = 0
}

func tabsForContext(ctx SelectionContext) []tab.Tab {
	switch ctx {
	case CtxItemPending:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewPlanTab()}
	case CtxItemActive:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewPlanTab(), tab.NewOutputTab(), tab.NewMetricsTab()}
	case CtxItemDone:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewPlanTab(), tab.NewOutputTab(), tab.NewLogTab(), tab.NewMetricsTab()}
	case CtxTaskRunning:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewOutputTab(), tab.NewLogTab(), tab.NewMetricsTab()}
	case CtxTaskComplete:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewOutputTab(), tab.NewLogTab(), tab.NewMetricsTab()}
	default:
		return []tab.Tab{tab.NewSummaryTab(), tab.NewPlanTab(), tab.NewOutputTab(), tab.NewLogTab(), tab.NewMetricsTab()}
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
	sep := styles.Separator(p.Width)

	// Split: top = tabs+content, bottom = log view
	topH := p.Height - logViewHeight
	contentH := topH - 3 // tab bar + top sep + bottom sep

	var content string
	if p.activeTab < len(p.tabs) {
		content = p.tabs[p.activeTab].View(p.Width, contentH)
	}

	// Log view
	p.logView.Width = p.Width
	p.logView.Height = logViewHeight
	p.logView.Refresh()

	return tabBar + "\n" + sep + "\n" + content + "\n" + sep + "\n" + p.logView.View()
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

func (p *RightPane) NextTab() {
	if len(p.tabs) > 0 {
		p.activeTab = (p.activeTab + 1) % len(p.tabs)
	}
}

func (p *RightPane) PrevTab() {
	if len(p.tabs) > 0 {
		p.activeTab = (p.activeTab - 1 + len(p.tabs)) % len(p.tabs)
	}
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

func (p *RightPane) AllTabs() []tab.Tab {
	return p.tabs
}

func (p *RightPane) SetActiveTab(t tab.Tab) {
	if p.activeTab < len(p.tabs) {
		p.tabs[p.activeTab] = t
	}
}

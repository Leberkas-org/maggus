package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
	xterm "golang.org/x/term"
)

const (
	statusHeaderLines = 11 // title + daemon line + blank + tab bar (~2) + separator + blank + progress + blank + tasks header + separator
	progressBarWidth  = 30
)

// Lipgloss styles for the status command.
var (
	statusGreenStyle = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle  = lipgloss.NewStyle().Foreground(styles.Primary)
	statusRedStyle   = lipgloss.NewStyle().Foreground(styles.Error)
	statusDimStyle   = lipgloss.NewStyle().Faint(true)
	statusBoldStyle  = lipgloss.NewStyle().Bold(true)
	statusBlueStyle  = lipgloss.NewStyle().Foreground(styles.Accent)
)

// statusModel is the bubbletea model for the interactive status TUI.
type statusModel struct {
	taskListComponent

	// Split-pane layout fields
	plans       []parser.Plan
	planCursor  int
	leftFocused bool
	activeTab   int // 0–3: Output, Feature Details, Current Task, Metrics

	// Right-pane tab 3 viewport
	currentTaskViewport viewport.Model
	currentTaskDetail   detailState

	// Terminal dimensions
	width  int
	height int

	showAll      bool
	nextTaskID   string
	nextTaskFile string
	agentName    string

	dir       string             // working directory for file operations
	approvals approval.Approvals // cached approvals; reloaded on reloadPlans

	approvalRequired bool // from config; used when reloading plans

	is2x bool // true when Claude is in 2x mode (border turns yellow)

	// Temporary status note (e.g. "feature approved")
	statusNote string

	// Feature-level delete confirmation
	confirmDeleteFeature bool
	deleteFeatureErr     string

	// Daemon stop-mode selection overlay
	daemonStopOverlay bool

	// Exit daemon prompt overlay (shown when daemon is running and auto-start is disabled)
	exitDaemonOverlay bool

	// Live log panel
	showLog       bool
	logLines      []string
	logScroll     int
	logAutoScroll bool
	daemon        daemonStatus

	// Rich live view from state.json
	snapshot     *runlog.StateSnapshot // nil when no snapshot available
	spinnerFrame int

	// Discord Rich Presence (nil when not configured)
	presence *discord.Presence

	// Cached metrics for Tab 4
	cachedFeatureMetrics featureMetrics
	cachedRepoMetrics    repoMetrics
	cachedGlobalMetrics  globalconfig.Metrics

	// File watcher for live feature reload
	watcher   *filewatcher.Watcher
	watcherCh <-chan bool
}

func newStatusModel(features []parser.Plan, showAll bool, nextTaskID, nextTaskFile, agentName, dir string, showLog bool, approvalRequired bool, approvals approval.Approvals) statusModel {
	m := statusModel{
		taskListComponent: taskListComponent{
			HeaderLines: statusHeaderLines,
		},
		plans:            features,
		showAll:          showAll,
		nextTaskID:       nextTaskID,
		nextTaskFile:     nextTaskFile,
		agentName:        agentName,
		dir:              dir,
		showLog:          showLog,
		approvalRequired: approvalRequired,
		approvals:        approvals,
		logAutoScroll:    true,
		leftFocused:      true,
		activeTab:        0,
	}
	// Query actual terminal dimensions before the first render so View() always
	// has a non-zero size and the split-pane is visible on the first frame
	// (same pattern as newMenuModel). Only set width/height here — HandleResize
	// and resizeCurrentTaskViewport are called from the WindowSizeMsg handler
	// where Bubble Tea provides the correct alt-screen dimensions.
	termW, termH, _ := xterm.GetSize(int(os.Stdout.Fd()))
	m.width = termW
	m.height = termH

	visible := m.visiblePlans()
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[0], showAll)
	}
	m.loadMetrics()
	return m
}

// hasCompletedPlans returns true if any plan in m.plans has Completed == true.
func (m statusModel) hasCompletedPlans() bool {
	for _, p := range m.plans {
		if p.Completed {
			return true
		}
	}
	return false
}

// visiblePlans returns the plans that should be shown based on the showAll flag.
func (m statusModel) visiblePlans() []parser.Plan {
	var visible []parser.Plan
	for _, p := range m.plans {
		if p.Completed && !m.showAll {
			continue
		}
		visible = append(visible, p)
	}
	return visible
}

// selectedPlan returns the plan at planCursor from visiblePlans.
// Returns a zero-value Plan if planCursor is out of range.
func (m statusModel) selectedPlan() parser.Plan {
	visible := m.visiblePlans()
	if m.planCursor < 0 || m.planCursor >= len(visible) {
		return parser.Plan{}
	}
	return visible[m.planCursor]
}

// rebuildForSelectedPlan rebuilds the selectable tasks and resets the cursor
// for the currently selected plan.
func (m *statusModel) rebuildForSelectedPlan() {
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		m.planCursor = 0
	}
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(visible[m.planCursor], m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
	m.loadMetrics()
}

// reloadPlans reloads all plans and approvals from disk and rebuilds the current view.
func (m *statusModel) reloadPlans() {
	plans, a, err := loadPlansWithApprovals(m.dir, true)
	if err != nil {
		m.rebuildForSelectedPlan()
		return
	}
	m.approvals = a
	m.plans = plans
	pruneStaleApprovals(m.dir, plans)
	m.nextTaskID, m.nextTaskFile = findNextTask(plans)
	m.rebuildForSelectedPlan()
	m.loadCurrentTaskDetail()
}

// syncDetailSuffix updates the component's DetailSuffix from statusNote.
func (m *statusModel) syncDetailSuffix() {
	if m.statusNote != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
		m.DetailSuffix = "\n" + mutedStyle.Render("  "+m.statusNote)
	} else {
		m.DetailSuffix = ""
	}
}

// featureFilePath returns the full path to a plan's source file.
func (m statusModel) featureFilePath(p parser.Plan) string {
	return p.File
}

// buildProgressBar renders a colored progress bar.
func buildProgressBar(done, total int) string {
	return styles.ProgressBar(done, total, progressBarWidth)
}

// buildProgressBarPlain renders a plain ASCII progress bar.
func buildProgressBarPlain(done, total int) string {
	return styles.ProgressBarPlain(done, total, progressBarWidth)
}

// buildSelectableTasksForFeature returns the flat list of tasks for a single plan.
// When showAll is false, completed tasks are excluded.
func buildSelectableTasksForFeature(plan parser.Plan, showAll bool) []parser.Task {
	var selectable []parser.Task
	for _, t := range plan.Tasks {
		if !showAll && t.IsComplete() {
			continue
		}
		selectable = append(selectable, t)
	}
	return selectable
}

// loadPlansWithApprovals loads all plans and the current approval map.
func loadPlansWithApprovals(dir string, includeCompleted bool) ([]parser.Plan, approval.Approvals, error) {
	plans, err := parser.LoadPlans(dir, includeCompleted)
	if err != nil {
		return nil, nil, err
	}
	a, err := approval.Load(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("load approvals: %w", err)
	}
	return plans, a, nil
}

// isPlanApproved checks whether a plan is approved given the approval map and mode.
func isPlanApproved(p parser.Plan, a approval.Approvals, approvalRequired bool) bool {
	return approval.IsApproved(a, p.ApprovalKey(), approvalRequired)
}

// findNextTask returns the ID and source file of the next workable task across all plans.
// Bugs are prioritized over features.
func findNextTask(plans []parser.Plan) (string, string) {
	// Bugs first, then features
	for _, p := range plans {
		if p.Completed || !p.IsBug {
			continue
		}
		next := parser.FindNextIncomplete(p.Tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	for _, p := range plans {
		if p.Completed || p.IsBug {
			continue
		}
		next := parser.FindNextIncomplete(p.Tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

// pruneStaleApprovals collects all known approval keys from the combined plan
// list and calls approval.Prune to remove stale entries.
func pruneStaleApprovals(dir string, all []parser.Plan) {
	var knownIDs []string
	for i := range all {
		knownIDs = append(knownIDs, all[i].ApprovalKey())
	}
	_ = approval.Prune(dir, knownIDs)
}

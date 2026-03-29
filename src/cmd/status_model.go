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
	"github.com/leberkas-org/maggus/internal/stores"
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
	plans         []parser.Plan
	planCursor    int
	treeCursor       int             // primary navigation index into buildTreeItems(); replaces planCursor
	treeScrollOffset int             // scroll offset for left pane tree view
	expandedPlans    map[string]bool // keyed by plan.ID; starts empty (all collapsed)
	leftFocused   bool
	activeTab     int // 0–3: Output, Feature Details, Current Task, Metrics

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

	dir          string                // working directory for file operations
	approvals    approval.Approvals    // cached approvals; reloaded on reloadPlans
	featureStore stores.FeatureStore   // store for feature plan file operations
	bugStore     stores.BugStore       // store for bug plan file operations

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

	// Live log panel scroll state
	logScroll     int
	logAutoScroll bool
	daemon        daemonStatus

	// Rich live view from state.json
	snapshot          *runlog.StateSnapshot // nil when no snapshot available
	spinnerFrame      int
	spinnerTicking    bool   // true while the 80ms tick loop is live
	frozenRunElapsed  string // frozen run elapsed when snap reaches a terminal state
	frozenTaskElapsed string // frozen task elapsed when snap reaches a terminal state

	// Discord Rich Presence (nil when not configured)
	presence *discord.Presence

	// Cached metrics for Tab 4
	cachedFeatureMetrics featureMetrics
	cachedRepoMetrics    repoMetrics
	cachedGlobalMetrics  globalconfig.Metrics

	// File watcher for live feature reload
	watcher   *filewatcher.Watcher
	watcherCh <-chan bool

	// Daemon state cache subscription channel
	daemonCacheCh chan daemonPIDState

	// fsnotify-based log file watcher (nil when fsnotify unavailable — falls back to polling)
	logWatcher   *LogFileWatcher
	logWatcherCh <-chan logFileUpdateMsg
}

func newStatusModel(features []parser.Plan, showAll bool, nextTaskID, nextTaskFile, agentName, dir string, showLog bool, approvalRequired bool, approvals approval.Approvals, featureStore stores.FeatureStore, bugStore stores.BugStore) statusModel {
	m := statusModel{
		taskListComponent: taskListComponent{
			HeaderLines:  statusHeaderLines,
			featureStore: featureStore,
			bugStore:     bugStore,
		},
		plans:            features,
		expandedPlans:    make(map[string]bool),
		showAll:          showAll,
		nextTaskID:       nextTaskID,
		nextTaskFile:     nextTaskFile,
		agentName:        agentName,
		dir:              dir,
		approvalRequired: approvalRequired,
		approvals:        approvals,
		featureStore:     featureStore,
		bugStore:         bugStore,
		logAutoScroll:   true,
		leftFocused:     true,
		activeTab:       0,
		spinnerTicking:  true,
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

// selectedPlan returns the plan for the tree row at treeCursor.
// Returns a zero-value Plan if treeCursor is out of range.
func (m statusModel) selectedPlan() parser.Plan {
	items := m.buildTreeItems()
	if m.treeCursor < 0 || m.treeCursor >= len(items) {
		return parser.Plan{}
	}
	return items[m.treeCursor].plan
}

// rebuildForSelectedPlan rebuilds the selectable tasks and resets the cursor
// for the currently selected plan. It syncs treeCursor from planCursor so that
// both cursors stay consistent while planCursor is still used for navigation.
func (m *statusModel) rebuildForSelectedPlan() {
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		m.planCursor = 0
	}
	// Sync treeCursor: walk buildTreeItems to find the tree-row index of the
	// planCursor-th plan row (skipping any task rows from expanded plans).
	m.syncTreeCursorFromPlanCursor()
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(m.selectedPlan(), m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
	m.loadMetrics()
}

// syncTreeCursorFromPlanCursor sets treeCursor to the tree-row index of the
// planCursor-th plan row in buildTreeItems(). This keeps treeCursor consistent
// with planCursor when planCursor is changed outside of tree navigation.
func (m *statusModel) syncTreeCursorFromPlanCursor() {
	items := m.buildTreeItems()
	planRowIdx := 0
	for i, item := range items {
		if item.kind == treeItemKindPlan {
			if planRowIdx == m.planCursor {
				m.treeCursor = i
				return
			}
			planRowIdx++
		}
	}
	m.treeCursor = 0
}

// syncPlanCursorFromTreeCursor derives planCursor from the current treeCursor.
// planCursor is the index among plan rows only (0-based). For task rows,
// planCursor points to the parent plan.
func (m *statusModel) syncPlanCursorFromTreeCursor() {
	items := m.buildTreeItems()
	planIdx := 0
	for i, item := range items {
		if item.kind == treeItemKindPlan {
			if i <= m.treeCursor {
				planIdx++
			}
		}
	}
	if planIdx > 0 {
		m.planCursor = planIdx - 1
	} else {
		m.planCursor = 0
	}
}

// rebuildRightPane rebuilds the right-pane task list and metrics for the
// currently selected plan without altering treeCursor or planCursor.
func (m *statusModel) rebuildRightPane() {
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		m.planCursor = 0
	}
	if len(visible) > 0 {
		m.Tasks = buildSelectableTasksForFeature(m.selectedPlan(), m.showAll)
	} else {
		m.Tasks = nil
	}
	m.Cursor = 0
	m.ScrollOffset = 0
	m.loadMetrics()
}

// reloadPlans reloads all plans and approvals from disk and rebuilds the current view.
func (m *statusModel) reloadPlans() {
	plans, a, err := loadPlansWithApprovals(m.dir, m.featureStore, m.bugStore, true)
	if err != nil {
		// Surface the error so it is visible rather than silently leaving stale data.
		m.statusNote = "reload error: " + err.Error()
		m.syncDetailSuffix()
		m.rebuildForSelectedPlan()
		return
	}
	m.approvals = a
	m.plans = plans
	pruneStaleApprovals(m.dir, plans)
	m.nextTaskID, m.nextTaskFile = findNextTask(plans)
	m.treeScrollOffset = 0
	m.rebuildForSelectedPlan()
	// Clamp treeCursor to the new tree length so it never goes out of bounds.
	items := m.buildTreeItems()
	if len(items) == 0 {
		m.treeCursor = 0
	} else if m.treeCursor >= len(items) {
		m.treeCursor = len(items) - 1
	}
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
// It also migrates any filename-based approval keys to MaggusID-based keys,
// preventing stale-prune badge regression when a plan gains a maggus-id after
// its approval was first saved.
func loadPlansWithApprovals(dir string, featureStore stores.FeatureStore, bugStore stores.BugStore, includeCompleted bool) ([]parser.Plan, approval.Approvals, error) {
	bugPlans, err := bugStore.LoadAll(includeCompleted)
	if err != nil {
		return nil, nil, fmt.Errorf("load bug plans: %w", err)
	}
	featurePlans, err := featureStore.LoadAll(includeCompleted)
	if err != nil {
		return nil, nil, fmt.Errorf("load feature plans: %w", err)
	}
	plans := append(bugPlans, featurePlans...)
	a, err := approval.Load(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("load approvals: %w", err)
	}
	if migrateApprovalKeys(plans, a) {
		// Best-effort persist; in-memory map is already correct even if save fails.
		_ = approval.Save(dir, a)
	}
	return plans, a, nil
}

// migrateApprovalKeys rewrites any filename-based approval entries to their
// corresponding MaggusID keys, in place. Returns true when at least one entry
// was migrated so the caller can persist the updated map.
//
// Migration applies when a plan has a MaggusID AND the approval map contains
// an entry under the filename-based ID but NOT under the UUID.  This covers
// the case where a plan was approved before the <!-- maggus-id: ... --> comment
// was added, causing pruneStaleApprovals to delete the legitimate entry.
func migrateApprovalKeys(plans []parser.Plan, a approval.Approvals) bool {
	migrated := false
	for _, p := range plans {
		if p.MaggusID == "" {
			continue
		}
		if val, ok := a[p.ID]; ok {
			if _, hasUUID := a[p.MaggusID]; !hasUUID {
				a[p.MaggusID] = val
				delete(a, p.ID)
				migrated = true
			}
		}
	}
	return migrated
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

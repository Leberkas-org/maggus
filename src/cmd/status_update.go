package cmd

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func (m statusModel) Init() tea.Cmd {
	if m.presence != nil {
		_ = m.presence.Update(discord.PresenceState{
			FeatureTitle: "Viewing Status",
			StartTime:    time.Now(),
		})
	}
	var logCmd tea.Cmd
	if m.logWatcherCh != nil {
		logCmd = listenForLogFileUpdate(m.logWatcherCh)
	} else {
		logCmd = logPollTick()
	}
	return tea.Batch(
		func() tea.Msg {
			return claude2xResultMsg{status: claude2x.FetchStatus()}
		},
		logCmd,
		spinnerTick(),
		listenForWatcherUpdate(m.watcherCh),
		listenForDaemonCacheUpdate(m.daemonCacheCh),
	)
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.HandleResize(msg.Width, msg.Height)
		m.resizeCurrentTaskViewport()
		m.loadCurrentTaskDetail()
		// Keep Tab 2 detail viewport sized to the right pane if it's open.
		if m.taskListComponent.ShowDetail && m.taskListComponent.detailReady {
			m.resizeTab2DetailViewport()
		}
		return m, nil

	case claude2xResultMsg:
		m.is2x = msg.status.Is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		if m.is2x {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		is2x, _, tickCmd := fetch2xAndUpdate()
		m.is2x = is2x
		m.BorderColor = styles.ThemeColor(m.is2x)
		return m, tickCmd

	case spinnerTickMsg:
		if m.daemon.Running && m.snapshot != nil {
			isTerminal := m.snapshot.Status == "Done" || m.snapshot.Status == "Failed" || m.snapshot.Status == "Interrupted"
			if isTerminal {
				// Stop the tick loop; it will be restarted when a new run begins.
				m.spinnerTicking = false
				return m, nil
			}
			m.spinnerFrame = (m.spinnerFrame + 1) % len(styles.SpinnerFrames)
			return m, spinnerTick()
		}
		// Keep ticking even when idle so the spinner starts immediately when daemon resumes.
		return m, spinnerTick()

	case daemonCacheUpdateMsg:
		prevRunning := m.daemon.Running
		m.daemon.PID = msg.State.PID
		m.daemon.Running = msg.State.Running
		if prevRunning && !m.daemon.Running {
			m.snapshot = nil
		}
		return m, listenForDaemonCacheUpdate(m.daemonCacheCh)

	case logFileUpdateMsg:
		prevFeature := m.daemon.CurrentFeature
		runID, logPath := findLatestRunLog(m.dir)
		m.daemon.RunID = runID
		m.daemon.LogPath = logPath
		if logPath != "" {
			lines := readLastNLogLines(logPath, 200)
			m.daemon.CurrentFeature, m.daemon.CurrentTask = parseLogForCurrentState(lines)
		}
		// Auto-expand the active plan when CurrentFeature changes so the task row is visible.
		if m.daemon.Running && m.daemon.CurrentFeature != "" && m.daemon.CurrentFeature != prevFeature {
			if m.expandedPlans == nil {
				m.expandedPlans = make(map[string]bool)
			}
			m.expandedPlans[m.daemon.CurrentFeature] = true
		}
		prevSnapStatus := ""
		if m.snapshot != nil {
			prevSnapStatus = m.snapshot.Status
		}
		if m.daemon.Running && m.daemon.RunID != "" {
			snap, err := runlog.ReadSnapshot(m.dir)
			if err == nil {
				m.snapshot = snap
			}
			// else: keep previous snapshot or nil
		} else if !m.daemon.Running {
			m.snapshot = nil
		}

		newSnapStatus := ""
		if m.snapshot != nil {
			newSnapStatus = m.snapshot.Status
		}
		isTerminalStatus := func(s string) bool {
			return s == "Done" || s == "Failed" || s == "Interrupted"
		}

		// Freeze elapsed times the moment a run reaches a terminal state.
		if m.snapshot != nil && isTerminalStatus(newSnapStatus) && !isTerminalStatus(prevSnapStatus) {
			if t, err := time.Parse(time.RFC3339, m.snapshot.RunStartedAt); err == nil {
				m.frozenRunElapsed = formatHumanDuration(time.Since(t))
			}
			if t, err := time.Parse(time.RFC3339, m.snapshot.TaskStartedAt); err == nil {
				m.frozenTaskElapsed = formatHumanDuration(time.Since(t))
			}
		}
		// Clear frozen elapsed and restart the tick loop when a new run begins.
		if newSnapStatus != "" && !isTerminalStatus(newSnapStatus) && !m.spinnerTicking {
			m.frozenRunElapsed = ""
			m.frozenTaskElapsed = ""
			m.spinnerTicking = true
			if m.logWatcherCh != nil {
				return m, tea.Batch(listenForLogFileUpdate(m.logWatcherCh), spinnerTick())
			}
			return m, tea.Batch(logPollTick(), spinnerTick())
		}

		if m.logWatcherCh != nil {
			return m, listenForLogFileUpdate(m.logWatcherCh)
		}
		return m, logPollTick()

	case featureSummaryUpdateMsg:
		// Preserve selected feature, cursor, and scroll across reload
		visible := m.visiblePlans()
		var selectedFilename string
		if m.planCursor < len(visible) {
			selectedFilename = filepath.Base(visible[m.planCursor].File)
		}
		prevCursor := m.Cursor
		prevScroll := m.ScrollOffset
		m.reloadPlans()
		// Restore selection by filename
		if selectedFilename != "" {
			for i, f := range m.visiblePlans() {
				if filepath.Base(f.File) == selectedFilename {
					m.planCursor = i
					m.syncTreeCursorFromPlanCursor()
					m.Tasks = buildSelectableTasksForFeature(f, m.showAll)
					// Clamp cursor and scroll to new bounds
					if prevCursor < len(m.Tasks) {
						m.Cursor = prevCursor
					} else if len(m.Tasks) > 0 {
						m.Cursor = len(m.Tasks) - 1
					}
					m.ScrollOffset = prevScroll
					break
				}
			}
		}
		return m, listenForWatcherUpdate(m.watcherCh)

	case tea.KeyMsg:
		if m.daemonStopOverlay {
			return m.updateStatusDaemonStopOverlay(msg)
		}
		if m.exitDaemonOverlay {
			return m.updateExitDaemonOverlay(msg)
		}
		if m.confirmDeleteFeature {
			return m.updateStatusConfirmDeleteFeature(msg)
		}
		if m.ConfirmDelete {
			return m.updateStatusConfirmDelete(msg)
		}
		if m.ShowDetail {
			return m.updateStatusDetail(msg)
		}
		return m.updateList(msg)
	}

	cmd := m.UpdateViewport(msg)
	return m, cmd
}

// resizeTab2DetailViewport resizes the embedded taskListComponent's detail viewport
// to the right-pane content area dimensions so it renders correctly in Tab 2.
func (m *statusModel) resizeTab2DetailViewport() {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)
	leftW := m.width / 3
	if leftW > 50 {
		leftW = 50
	}
	rightW := innerW - leftW
	if rightW < 1 {
		rightW = 1
	}
	contentH := m.rightPaneContentHeight()
	// Full content height; footer is rendered in the shared split footer bar.
	vpH := contentH
	if vpH < 1 {
		vpH = 1
	}
	m.taskListComponent.detailViewport.Width = rightW
	m.taskListComponent.detailViewport.Height = vpH
}


// treeAvailableHeight returns the number of item rows visible in the left pane
// after subtracting the fixed header lines (label + separator + daemon status + separator).
// Used by both clampTreeScroll and renderLeftPane to keep scroll math consistent.
func (m *statusModel) treeAvailableHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	// innerH-1 (renderLeftPane receives innerH-1) + 5 header lines (label + sep + empty + daemon + sep)
	const treeOverhead = 6
	availH := innerH - treeOverhead
	if availH < 1 {
		availH = 1
	}
	return availH
}

// clampTreeScroll adjusts treeScrollOffset so the cursor stays visible with
// 2 lines of context above and below, then clamps the offset to valid bounds.
func (m *statusModel) clampTreeScroll() {
	items := m.buildTreeItems()
	availH := m.treeAvailableHeight()

	// Pull offset up when cursor is near the top
	if m.treeCursor < m.treeScrollOffset+2 {
		m.treeScrollOffset = max(0, m.treeCursor-2)
	}
	// Push offset down when cursor is near the bottom
	if m.treeCursor >= m.treeScrollOffset+availH-2 {
		m.treeScrollOffset = m.treeCursor - availH + 3
	}

	// Clamp to [0, max(0, len(items)-availH)]
	maxOffset := max(0, len(items)-availH)
	if m.treeScrollOffset < 0 {
		m.treeScrollOffset = 0
	}
	if m.treeScrollOffset > maxOffset {
		m.treeScrollOffset = maxOffset
	}
}

// skipSeparatorUp moves cursor backward (wrapping) past any separator items.
// Since only one separator exists, a single extra step is sufficient.
func skipSeparatorUp(cursor int, items []treeItem) int {
	if cursor >= 0 && cursor < len(items) && items[cursor].kind == treeItemKindSeparator {
		cursor = styles.CursorUp(cursor, len(items))
	}
	return cursor
}

// skipSeparatorDown moves cursor forward (wrapping) past any separator items.
func skipSeparatorDown(cursor int, items []treeItem) int {
	if cursor >= 0 && cursor < len(items) && items[cursor].kind == treeItemKindSeparator {
		cursor = styles.CursorDown(cursor, len(items))
	}
	return cursor
}

// maxLogScroll returns the maximum valid scroll offset for the log panel.
// When a snapshot is available, scrolling operates on tool entries.
func (m *statusModel) maxLogScroll() int {
	visible := m.logVisibleLines()
	count := m.logItemCount()
	max := count - visible
	if max < 0 {
		max = 0
	}
	return max
}

// logItemCount returns the number of scrollable items in the log panel.
func (m *statusModel) logItemCount() int {
	if m.snapshot != nil && m.daemon.Running {
		return len(m.snapshot.ToolEntries)
	}
	return 0
}

// logVisibleLines returns the number of visible lines available for the scrollable
// area in the log panel. In split-pane mode, delegates to outputTabScrollableLines.
func (m *statusModel) logVisibleLines() int {
	if m.width > 0 && m.height > 0 {
		return m.outputTabScrollableLines()
	}
	// Legacy compact (non-split) mode.
	total := m.visibleTaskLines()
	if m.snapshot != nil && m.daemon.Running {
		overhead := 11
		avail := total - overhead
		if avail < 3 {
			avail = 3
		}
		return avail
	}
	return total
}

func (m statusModel) updateStatusConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		t := m.Tasks[m.Cursor]
		if err := m.storeForFile(t.SourceFile).DeleteTask(t.SourceFile, t.ID); err != nil {
			m.DeleteErr = err.Error()
			m.ConfirmDelete = false
			return m, nil
		}
		m.reloadPlans()
		if m.Cursor >= len(m.Tasks) && m.Cursor > 0 {
			m.Cursor--
		}
		m.ConfirmDelete = false
		m.ShowDetail = false
		if len(m.Tasks) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc":
		m.ConfirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusConfirmDeleteFeature(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		visible := m.visiblePlans()
		if m.planCursor >= len(visible) {
			m.confirmDeleteFeature = false
			return m, nil
		}
		f := visible[m.planCursor]
		fullPath := m.featureFilePath(f)
		if err := os.Remove(fullPath); err != nil {
			m.deleteFeatureErr = err.Error()
			m.confirmDeleteFeature = false
			return m, nil
		}
		m.confirmDeleteFeature = false
		m.reloadPlans()
		// Clamp planCursor to valid range
		newVisible := m.visiblePlans()
		if m.planCursor >= len(newVisible) {
			m.planCursor = len(newVisible) - 1
		}
		if m.planCursor < 0 {
			m.planCursor = 0
		}
		m.rebuildForSelectedPlan()
		if len(newVisible) == 0 {
			return m, tea.Quit
		}
		return m, nil
	case "n", "N", "esc":
		m.confirmDeleteFeature = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusDaemonStopOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Graceful stop (stop after current task).
		m.daemonStopOverlay = false
		dir := m.dir
		return m, func() tea.Msg {
			_ = stopDaemonGracefully(dir)
			return nil
		}
	case "k", "K", "ctrl+c", "ctrl+C":
		// Immediate kill.
		m.daemonStopOverlay = false
		dir := m.dir
		pid := m.daemon.PID
		return m, func() tea.Msg {
			_ = forceKill(pid)
			removeDaemonPID(dir)
			return nil
		}
	case "esc":
		m.daemonStopOverlay = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateExitDaemonOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d", "D":
		return m, tea.Quit
	case "y", "Y":
		_ = stopDaemonGracefully(m.dir)
		return m, tea.Quit
	case "k", "K", "ctrl+c", "ctrl+C":
		_ = forceKill(m.daemon.PID)
		removeDaemonPID(m.dir)
		return m, tea.Quit
	case "esc", "q", "Q":
		m.exitDaemonOverlay = false
		return m, nil
	}
	return m, nil
}

// shouldPromptOnExit returns true when the daemon is running and auto-start is
// disabled for the current repo — meaning the user started it manually and
// should be asked before exiting.
func (m statusModel) shouldPromptOnExit() bool {
	if !m.daemon.Running {
		return false
	}
	cfg, err := globalconfig.Load()
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(m.dir)
	if err != nil {
		return false
	}
	for _, repo := range cfg.Repositories {
		if repo.Path == absDir {
			return !repo.IsAutoStartEnabled()
		}
	}
	return false
}

// handleQuitRequest either shows the exit daemon overlay or quits immediately,
// depending on whether the daemon is running with auto-start disabled.
func (m statusModel) handleQuitRequest() (statusModel, tea.Cmd) {
	if m.shouldPromptOnExit() {
		m.exitDaemonOverlay = true
		return m, nil
	}
	return m, tea.Quit
}
func (m statusModel) updateStatusDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Intercept status-specific keys before delegating to component
	if msg.String() == "alt+p" {
		return m.handleApproveToggle()
	}
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m, tea.Quit
	}
	return m, cmd
}

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear deleteFeatureErr on any key press
	if m.deleteFeatureErr != "" {
		m.deleteFeatureErr = ""
		return m, nil
	}

	// Clear status note on any key except alt+p
	if msg.String() != "alt+p" {
		m.statusNote = ""
	}
	m.syncDetailSuffix()

	key := msg.String()

	// Key 1 focuses the left pane; keys 2–5 focus the right pane and switch tabs.
	switch key {
	case "1":
		m.leftFocused = true
		return m, nil
	case "2", "3", "4", "5":
		m.leftFocused = false
		m.activeTab = int(key[0] - '2')
		if m.activeTab == 0 {
			m.logAutoScroll = true
			m.logScroll = m.maxLogScroll()
		}
		return m, nil
	}

	// Right pane is focused in split mode: route keys by active tab.
	if !m.leftFocused && m.width > 0 && m.height > 0 {
		// Tab 2 — Feature Details: delegate to the task list component.
		if m.activeTab == 1 {
			if key == "q" {
				return m.handleQuitRequest()
			}
			cmd, action := m.taskListComponent.Update(msg)
			switch action {
			case taskListQuit:
				return m.handleQuitRequest()
			case taskListDeleted:
				m.reloadPlans()
			}
			// After opening detail, resize its viewport to fit the right pane.
			if m.taskListComponent.ShowDetail && m.taskListComponent.detailReady {
				m.resizeTab2DetailViewport()
			}
			return m, cmd
		}

		// Tab 1 — Output: log scroll. Tab 3 — Current Task: viewport scroll.
		switch key {
		case "q":
			return m.handleQuitRequest()
		case "down":
			if m.activeTab == 0 {
				m.logAutoScroll = false
				m.logScroll = min(m.logScroll+1, m.maxLogScroll())
			} else if m.activeTab == 2 {
				m.currentTaskViewport.ScrollDown(1)
			}
		case "up":
			if m.activeTab == 0 {
				m.logAutoScroll = false
				m.logScroll = max(m.logScroll-1, 0)
			} else if m.activeTab == 2 {
				m.currentTaskViewport.ScrollUp(1)
			}
		case "G":
			if m.activeTab == 0 {
				m.logAutoScroll = true
				m.logScroll = m.maxLogScroll()
			}
		case "g":
			if m.activeTab == 0 {
				m.logAutoScroll = false
				m.logScroll = 0
			}
		}
		return m, nil
	}

	// Left pane is focused (default): feature navigation.
	switch key {
	case "up", "k":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			m.treeCursor = skipSeparatorUp(styles.CursorUp(m.treeCursor, len(items)), items)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "down", "j":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			m.treeCursor = skipSeparatorDown(styles.CursorDown(m.treeCursor, len(items)), items)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "home":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			m.treeCursor = 0
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "end":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			m.treeCursor = len(items) - 1
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "shift+tab":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			m.treeCursor = skipSeparatorUp(styles.CursorUp(m.treeCursor, len(items)), items)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "enter":
		// Move focus to right pane and switch to Tab 2 — Feature Details.
		m.leftFocused = false
		m.activeTab = 1
		return m, nil
	case "right", "l":
		items := m.buildTreeItems()
		if m.treeCursor < len(items) {
			item := items[m.treeCursor]
			if item.kind == treeItemKindPlan && len(item.plan.Tasks) > 0 && !m.expandedPlans[item.plan.ID] {
				if m.expandedPlans == nil {
					m.expandedPlans = make(map[string]bool)
				}
				m.expandedPlans[item.plan.ID] = true
				m.clampTreeScroll()
			}
		}
		return m, nil
	case "left", "h":
		items := m.buildTreeItems()
		if m.treeCursor < len(items) {
			item := items[m.treeCursor]
			if item.kind == treeItemKindPlan {
				delete(m.expandedPlans, item.plan.ID)
				m.clampTreeScroll()
			} else if item.kind == treeItemKindTask {
				parentID := item.plan.ID
				delete(m.expandedPlans, parentID)
				// After collapsing, find and select the parent plan row.
				newItems := m.buildTreeItems()
				for i, it := range newItems {
					if it.kind == treeItemKindPlan && it.plan.ID == parentID {
						m.treeCursor = i
						break
					}
				}
				m.syncPlanCursorFromTreeCursor()
				m.clampTreeScroll()
			}
		}
		return m, nil
	case "alt+a":
		m.showAll = !m.showAll
		plans, a, err := loadPlansWithApprovals(m.dir, m.featureStore, m.bugStore, true)
		if err == nil {
			m.approvals = a
			m.plans = plans
			pruneStaleApprovals(m.dir, plans)
		}
		m.nextTaskID, m.nextTaskFile = findNextTask(m.plans)
		m.rebuildForSelectedPlan()
		m.clampTreeScroll()
		return m, nil
	case "a":
		return m.handleApproveToggle()
	case "alt+d":
		visible := m.visiblePlans()
		if len(visible) > 0 && m.planCursor < len(visible) && !m.ConfirmDelete {
			m.confirmDeleteFeature = true
		}
		return m, nil
	case "s":
		if m.daemon.Running {
			m.daemonStopOverlay = true
		} else {
			// Start the daemon asynchronously.
			dir := m.dir
			return m, func() tea.Msg {
				_ = startDaemon(dir)
				return nil
			}
		}
		return m, nil
	case "alt+up":
		return m.movePlanUp()
	case "alt+down":
		return m.movePlanDown()
	}

	// Delegate to component for shared navigation (task list, detail view, etc.)
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit, taskListRun:
		return m.handleQuitRequest()
	case taskListDeleted:
		m.reloadPlans()
	}
	// In split mode, keep Tab 2 detail viewport sized to the right pane.
	if m.width > 0 && m.height > 0 && m.taskListComponent.ShowDetail && m.taskListComponent.detailReady {
		m.resizeTab2DetailViewport()
	}
	return m, cmd
}

// movePlanUp moves the selected plan one position up among visible plans of the same type.
// Reorder is memory-only (no file writes). cursor follows the moved item.
func (m statusModel) movePlanUp() (tea.Model, tea.Cmd) {
	visible := m.visiblePlans()
	if m.planCursor <= 0 || len(visible) == 0 {
		return m, nil
	}
	current := visible[m.planCursor]
	target := visible[m.planCursor-1]
	if current.IsBug != target.IsBug {
		return m, nil
	}
	swapPlansByFile(m.plans, current.File, target.File)
	m.planCursor--
	m.syncTreeCursorFromPlanCursor()
	return m, nil
}

// movePlanDown moves the selected plan one position down among visible plans of the same type.
// Reorder is memory-only (no file writes). cursor follows the moved item.
func (m statusModel) movePlanDown() (tea.Model, tea.Cmd) {
	visible := m.visiblePlans()
	if m.planCursor >= len(visible)-1 || len(visible) == 0 {
		return m, nil
	}
	current := visible[m.planCursor]
	target := visible[m.planCursor+1]
	if current.IsBug != target.IsBug {
		return m, nil
	}
	swapPlansByFile(m.plans, current.File, target.File)
	m.planCursor++
	m.syncTreeCursorFromPlanCursor()
	return m, nil
}

// swapPlansByFile swaps two plans in the slice by their file paths.
func swapPlansByFile(plans []parser.Plan, fileA, fileB string) {
	idxA, idxB := -1, -1
	for i, p := range plans {
		if p.File == fileA {
			idxA = i
		}
		if p.File == fileB {
			idxB = i
		}
	}
	if idxA >= 0 && idxB >= 0 {
		plans[idxA], plans[idxB] = plans[idxB], plans[idxA]
	}
}

func (m statusModel) handleApproveToggle() (tea.Model, tea.Cmd) {
	m.statusNote = ""
	visible := m.visiblePlans()
	if m.planCursor >= len(visible) {
		return m, nil
	}
	f := visible[m.planCursor]
	if f.Completed {
		m.statusNote = "cannot approve a completed feature"
		return m, nil
	}
	key := f.ApprovalKey()
	// Additive-only toggle: the toggle never writes an explicit false entry.
	// If there is an explicit true, remove the entry (back to default).
	// Otherwise (no entry or explicit false), write explicit true.
	// This prevents accidental unapproval in opt-out mode where the user presses
	// 'a' expecting to confirm approval of an already-default-approved plan.
	var err error
	if val, ok := m.approvals[key]; ok && val {
		err = approval.Remove(m.dir, key)
		if err == nil {
			m.statusNote = "feature approval removed"
		}
	} else {
		err = approval.Approve(m.dir, key)
		if err == nil {
			m.statusNote = "feature approved"
		}
	}
	if err != nil {
		m.statusNote = "error: " + err.Error()
		return m, nil
	}
	m.reloadPlans()
	return m, nil
}

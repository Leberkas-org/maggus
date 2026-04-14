package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/claude2x"
	"github.com/leberkas-org/maggus/internal/discord"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// editorDoneMsg is sent back to the Update loop after the external editor exits.
// It triggers a plan reload so the TUI reflects any file changes made in the editor.
type editorDoneMsg struct{}

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
		func() tea.Msg { return logFileUpdateMsg{} },
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
		m.isNerfed = msg.status.IsNerfed
		m.BorderColor = styles.ThemeColor(m.isNerfed)
		if m.isNerfed {
			return m, next2xTick()
		}
		return m, nil
	case claude2xTickMsg:
		isNerfed, _, tickCmd := fetch2xAndUpdate()
		m.isNerfed = isNerfed
		m.BorderColor = styles.ThemeColor(m.isNerfed)
		return m, tickCmd

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(styles.SpinnerFrames)
		if m.isParallelMode() {
			m.advanceWorkerSpinners()
		}
		// Supplementary snapshot poll: every 12th frame (~1s) re-read the
		// snapshot as a safety net for missed fsnotify events. On Windows,
		// atomic writes (temp + rename) can race with the channel buffer and
		// cause the watcher to miss an update.
		if m.daemon.Running && m.spinnerFrame == 0 {
			if snap, err := runlog.ReadSnapshot(m.dir); err == nil {
				m.snapshot = snap
				m.daemon.CurrentTask = snap.TaskID
				m.daemon.CurrentFeature = m.planIDForMaggusID(snap.MaggusID)
			}
		}
		// Always keep the tick loop alive so the spinner starts animating
		// immediately when a new task begins. Rendering code substitutes
		// static icons (✓/✗/⊘) for terminal statuses, so advancing the
		// frame during idle/terminal states is harmless.
		return m, spinnerTick()

	case daemonCacheUpdateMsg:
		prevRunning := m.daemon.Running
		m.daemon.PID = msg.State.PID
		m.daemon.Running = msg.State.Running
		if msg.State.StoppingAfterTask {
			m.daemonStoppingAfterTask = true
		} else if !msg.State.Running {
			// Only clear the flag when the daemon has fully exited, not during
			// the transition window where the sentinel was removed but the
			// process is still alive.
			m.daemonStoppingAfterTask = false
		}
		if prevRunning && !m.daemon.Running {
			m.snapshot = nil
		}
		return m, listenForDaemonCacheUpdate(m.daemonCacheCh)

	case logFileUpdateMsg:
		prevFeature := m.daemon.CurrentFeature
		prevSnapStatus := ""
		if m.snapshot != nil {
			prevSnapStatus = m.snapshot.Status
		}
		if m.daemon.Running {
			snap, err := runlog.ReadSnapshot(m.dir)
			if err == nil {
				m.snapshot = snap
				// Derive CurrentTask and CurrentFeature from the snapshot.
				m.daemon.CurrentTask = snap.TaskID
				m.daemon.CurrentFeature = m.planIDForMaggusID(snap.MaggusID)
			}
			// else: keep previous snapshot
		} else {
			m.snapshot = nil
		}
		// Auto-expand the active plan when CurrentFeature changes so the task row is visible.
		if m.daemon.Running && m.daemon.CurrentFeature != "" && m.daemon.CurrentFeature != prevFeature {
			if m.expandedPlans == nil {
				m.expandedPlans = make(map[string]bool)
			}
			m.expandedPlans[m.daemon.CurrentFeature] = true
		}

		// Read parallel worker state (nil when not in parallel mode).
		m.refreshWorkerSnapshots()

		// Auto-scroll the output tab to follow the latest tool entry.
		if m.logAutoScroll {
			m.logScroll = m.maxLogScroll()
		}

		// Refresh completed task output cache when viewing a completed task.
		if m.selectionCtx() == selCompletedTask {
			m.ensureCompletedTaskOutput()
		}
		// Refresh feature output cache when viewing the feature output tab.
		if m.selectionCtx() == selFeature && m.activeTabKey() == "featureoutput" {
			m.ensureFeatureOutput()
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
		// Clear frozen elapsed when a new run begins (terminal → non-terminal transition).
		if newSnapStatus != "" && !isTerminalStatus(newSnapStatus) && isTerminalStatus(prevSnapStatus) {
			m.frozenRunElapsed = ""
			m.frozenTaskElapsed = ""
		}

		if m.logWatcherCh != nil {
			return m, listenForLogFileUpdate(m.logWatcherCh)
		}
		return m, logPollTick()

	case editorDoneMsg:
		// Reload plans after the external editor exits so the TUI reflects any changes.
		m.reloadPlans()
		return m, nil

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
		// F1 help modal — handled before any other key processing.
		// F1 toggles the popup; Esc closes it; all other keys are consumed while it is open.
		if msg.String() == "f1" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			if msg.String() == "esc" {
				m.showHelp = false
			}
			return m, nil
		}
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
// after subtracting the fixed header lines (label + separator + empty + daemon + separator).
// Used by both clampTreeScroll and renderLeftPane to keep scroll math consistent.
func (m *statusModel) treeAvailableHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	// renderLeftPane receives innerH-2, then 5 header lines (label + sep + empty + daemon + sep)
	const treeOverhead = 7
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
// For running tasks, uses the appropriate live snapshot (main or per-worker).
// For completed tasks, uses the cached JSONL-loaded output.
// For feature output tab, sums cached completed entries plus any live running task entries.
func (m *statusModel) logItemCount() int {
	ctx := m.selectionCtx()
	if ctx == selRunningTask {
		snap := m.snapshotForSelectedTask()
		if snap != nil {
			return len(snap.ToolEntries)
		}
		return 0
	}
	if ctx == selCompletedTask && m.cachedTaskOutput != nil {
		return len(m.cachedTaskOutput.ToolEntries)
	}
	if ctx == selFeature && m.activeTabKey() == "featureoutput" {
		count := 0
		for _, snap := range m.cachedFeatureOutput {
			if snap != nil {
				count += len(snap.ToolEntries)
			}
		}
		// Add live entries for any running task in the selected feature.
		plan := m.selectedPlan()
		for _, task := range plan.Tasks {
			if m.isTaskRunning(task.ID) {
				if ws, ok := m.workerSnapshots[task.ID]; ok {
					count += len(ws.ToolEntries)
				} else if m.snapshot != nil && m.snapshot.TaskID == task.ID {
					count += len(m.snapshot.ToolEntries)
				}
				break
			}
		}
		return count
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
	switch normalizeKey(msg) {
	case "y", "enter":
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
			return m, func() tea.Msg { return navigateBackMsg{} }
		}
		return m, nil
	case "n", "esc":
		m.ConfirmDelete = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusConfirmDeleteFeature(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch normalizeKey(msg) {
	case "y", "enter":
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
			return m, func() tea.Msg { return navigateBackMsg{} }
		}
		return m, nil
	case "n", "esc":
		m.confirmDeleteFeature = false
		return m, nil
	}
	return m, nil
}

func (m statusModel) updateStatusDaemonStopOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch normalizeKey(msg) {
	case "s":
		// Stop after current task — send the stop-after-task signal, stay in status view.
		m.daemonStopOverlay = false
		m.daemonStoppingAfterTask = true
		dir := m.dir
		return m, func() tea.Msg {
			_ = sendStopAfterTaskSignal(dir)
			return nil
		}
	case "k", "ctrl+c":
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
	switch normalizeKey(msg) {
	case "d":
		return m, func() tea.Msg { return navigateBackMsg{} }
	case "s":
		// Stop after current task, then exit. Daemon finishes in background.
		_ = sendStopAfterTaskSignal(m.dir)
		return m, func() tea.Msg { return navigateBackMsg{} }
	case "k", "ctrl+c":
		_ = forceKill(m.daemon.PID)
		removeDaemonPID(m.dir)
		return m, func() tea.Msg { return navigateBackMsg{} }
	case "esc", "q":
		m.exitDaemonOverlay = false
		return m, nil
	}
	return m, nil
}

// shouldPromptOnExit returns true when the daemon is running, so the user is
// always asked before exiting to prevent orphaned daemon processes.
func (m statusModel) shouldPromptOnExit() bool {
	return m.daemon.Running
}

// handleQuitRequest either shows the exit daemon overlay or quits immediately,
// depending on whether the daemon is running with auto-start disabled.
func (m statusModel) handleQuitRequest() (statusModel, tea.Cmd) {
	m.showHelp = false
	if m.shouldPromptOnExit() {
		m.exitDaemonOverlay = true
		return m, nil
	}
	return m, func() tea.Msg { return navigateBackMsg{} }
}

// buildRunTaskMsg returns a tea.Cmd that emits an execProcessMsg asking the app
// router to run `maggus run --task <id>` in the foreground via tea.ExecProcess.
// After the run process exits, the router receives navigateBackMsg to return to
// the main menu.
func (m statusModel) buildRunTaskMsg() tea.Cmd {
	taskID := m.taskListComponent.RunTaskID
	return func() tea.Msg {
		execPath, _ := os.Executable()
		return execProcessMsg{
			cmd: exec.Command(execPath, "run", "--task", taskID),
			onDone: func(err error) tea.Msg {
				return navigateBackMsg{}
			},
		}
	}
}

// handleAltRunDispatch handles the Alt+R key press for the given task.
// When the daemon is running, writes a dispatch sentinel file so the orchestrator
// picks up the task immediately ahead of the normal queue.
// When the daemon is not running, falls back to foreground execution via tea.ExecProcess.
// No-ops when: task is nil, complete, blocked, or already running in a worker.
func (m statusModel) handleAltRunDispatch(task *parser.Task) (statusModel, tea.Cmd) {
	if task == nil {
		return m, nil
	}
	if task.IsComplete() || task.IsBlocked() {
		return m, nil
	}
	if m.isTaskRunning(task.ID) {
		m.statusNote = "Task already running"
		m.syncDetailSuffix()
		return m, nil
	}

	// When the daemon is running, use file-based dispatch signaling.
	// The orchestrator polls for sentinel files and runs the task in a worktree.
	if m.daemon.Running {
		taskID := task.ID
		dir := m.dir
		m.statusNote = "Dispatched " + taskID
		m.syncDetailSuffix()
		return m, func() tea.Msg {
			_ = writeDispatchSentinel(dir, taskID)
			return nil
		}
	}

	// Daemon not running: fall back to foreground execution.
	m.taskListComponent.RunTaskID = task.ID
	return m, m.buildRunTaskMsg()
}

func (m statusModel) updateStatusDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Intercept status-specific keys before delegating to component
	if msg.String() == "alt+p" {
		return m.handleApproveToggle()
	}
	if msg.String() == "alt+r" {
		return m.handleAltRunDispatch(m.taskListComponent.CurrentTask())
	}
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit:
		return m, func() tea.Msg { return navigateBackMsg{} }
	case taskListRun:
		return m, m.buildRunTaskMsg()
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

	// Number keys switch the right-pane tab positionally: 1 = first tab, 2 = second, etc.
	// Keys beyond the available tab count are ignored.
	switch key {
	case "1", "2", "3", "4", "5":
		idx := int(key[0] - '1')
		tabs := m.availableTabs()
		if idx >= len(tabs) {
			return m, nil // key beyond available tab count — ignore
		}
		m.setActiveTabIndex(idx)
		if tabs[idx].key == "output" {
			m.logAutoScroll = true
			m.logScroll = m.maxLogScroll()
		}
		return m, nil
	}

	// Content scroll keys — always consumed, but only act on scrollable tabs.
	// shift+up/shift+down scroll one line; g/G jump to top/bottom.
	switch key {
	case "shift+up":
		switch m.activeTabKey() {
		case "output", "featureoutput":
			if m.logScroll > 0 {
				m.logScroll--
				m.logAutoScroll = false
			}
		case "taskdetails":
			m.currentTaskViewport.ScrollUp(1)
		}
		return m, nil
	case "shift+down":
		switch m.activeTabKey() {
		case "output", "featureoutput":
			maxS := m.maxLogScroll()
			if m.logScroll < maxS {
				m.logScroll++
			}
			// Re-enable auto-scroll when we reach the bottom; disable it otherwise.
			m.logAutoScroll = m.logScroll >= m.maxLogScroll()
		case "taskdetails":
			m.currentTaskViewport.ScrollDown(1)
		}
		return m, nil
	case "g":
		switch m.activeTabKey() {
		case "output":
			m.logScroll = 0
			m.logAutoScroll = false
		case "taskdetails":
			m.currentTaskViewport.GotoTop()
		}
		return m, nil
	case "G":
		switch m.activeTabKey() {
		case "output":
			m.logScroll = m.maxLogScroll()
			m.logAutoScroll = true
		case "taskdetails":
			m.currentTaskViewport.GotoBottom()
		}
		return m, nil
	case "tab":
		// Cycle the right-pane tab forward (wrapping). Consumed here so it never
		// reaches the feature-tree navigation or the taskListComponent.
		tabs := m.availableTabs()
		if len(tabs) > 0 {
			idx := (m.activeTabIndex() + 1) % len(tabs)
			m.setActiveTabIndex(idx)
			if tabs[idx].key == "output" || tabs[idx].key == "featureoutput" {
				m.logAutoScroll = true
				m.logScroll = m.maxLogScroll()
			}
		}
		return m, nil
	case "shift+tab":
		// Cycle the right-pane tab backward (wrapping). Consumed here so it never
		// reaches the normalizeKey switch where it previously moved the left-pane cursor.
		tabs := m.availableTabs()
		if len(tabs) > 0 {
			idx := m.activeTabIndex() - 1
			if idx < 0 {
				idx = len(tabs) - 1
			}
			m.setActiveTabIndex(idx)
			if tabs[idx].key == "output" || tabs[idx].key == "featureoutput" {
				m.logAutoScroll = true
				m.logScroll = m.maxLogScroll()
			}
		}
		return m, nil
	}

	// Feature tree navigation — always active.
	switch normalizeKey(msg) {
	case "up", "k":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = skipSeparatorUp(styles.CursorUp(m.treeCursor, len(items)), items)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			} else {
				m.loadCurrentTaskDetail()
			}
		}
		return m, nil
	case "down", "j":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = skipSeparatorDown(styles.CursorDown(m.treeCursor, len(items)), items)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			} else {
				m.loadCurrentTaskDetail()
			}
		}
		return m, nil
	case "pgdown":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = findNextPlanRow(items, m.treeCursor)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "pgup":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = findPrevPlanRow(items, m.treeCursor)
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "home":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = 0
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "end":
		items := m.buildTreeItems()
		if len(items) > 0 {
			prevPlan := m.selectedPlan()
			prevCtx := m.selectionCtx()
			m.treeCursor = len(items) - 1
			m.clampTreeScroll()
			m.syncPlanCursorFromTreeCursor()
			m.updateTabsForSelectionChange(prevCtx)
			if m.selectedPlan().ID != prevPlan.ID {
				m.rebuildRightPane()
			}
		}
		return m, nil
	case "enter":
		// Open task detail when a task row is selected; switch to Details tab for plan rows.
		items := m.buildTreeItems()
		if m.treeCursor >= 0 && m.treeCursor < len(items) {
			item := items[m.treeCursor]
			if item.kind == treeItemKindTask && item.task != nil {
				found := false
				for i, t := range m.taskListComponent.Tasks {
					if t.ID == item.task.ID {
						m.taskListComponent.Cursor = i
						found = true
						break
					}
				}
				if found {
					m.taskListComponent.openDetail()
					m.resizeTab2DetailViewport()
				}
			} else if item.kind == treeItemKindPlan {
				// Switch to Details tab if available in current context.
				for i, td := range m.availableTabs() {
					if td.key == "details" {
						m.setActiveTabIndex(i)
						break
					}
				}
			}
		}
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
				prevCtx := m.selectionCtx()
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
				m.updateTabsForSelectionChange(prevCtx)
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
	case "x":
		return m.handleSkipToggle()
	case "b":
		task := m.selectedTask()
		if task == nil {
			return m, nil
		}
		found := false
		for i, t := range m.taskListComponent.Tasks {
			if t.ID == task.ID {
				m.taskListComponent.Cursor = i
				found = true
				break
			}
		}
		if !found {
			return m, nil
		}
		m.taskListComponent.openDetail()
		m.resizeTab2DetailViewport()
		if !m.taskListComponent.Detail.initCriteriaMode(*task) {
			m.taskListComponent.closeDetail()
		} else {
			m.taskListComponent.refreshDetailViewport()
		}
		return m, nil
	case "e":
		filePath := m.treeSelectedFilePath()
		if filePath == "" {
			return m, nil
		}
		editor := resolveEditor()
		return m, func() tea.Msg {
			return execProcessMsg{
				cmd:    exec.Command(editor, filePath),
				onDone: func(_ error) tea.Msg { return editorDoneMsg{} },
			}
		}
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
	case "alt+r":
		return m.handleAltRunDispatch(m.selectedTask())
	case "alt+up":
		return m.movePlanUp()
	case "alt+down":
		return m.movePlanDown()
	}

	// Delegate to component for shared navigation (task list, detail view, etc.)
	cmd, action := m.taskListComponent.Update(msg)
	switch action {
	case taskListQuit:
		return m.handleQuitRequest()
	case taskListRun:
		return m, m.buildRunTaskMsg()
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

// handleSkipToggle toggles the skip status of the first unchecked criterion on the
// currently selected task row. It is a no-op when a plan row is selected or when
// the task has no unchecked criteria (i.e. the task is already complete).
func (m statusModel) handleSkipToggle() (tea.Model, tea.Cmd) {
	items := m.buildTreeItems()
	if m.treeCursor < 0 || m.treeCursor >= len(items) {
		return m, nil
	}
	item := items[m.treeCursor]
	if item.kind != treeItemKindTask || item.task == nil {
		return m, nil // no-op for plan/separator rows
	}
	task := item.task

	// Find the first unchecked criterion.
	var target *parser.Criterion
	for i := range task.Criteria {
		if !task.Criteria[i].Checked {
			target = &task.Criteria[i]
			break
		}
	}
	if target == nil {
		return m, nil // task complete (all criteria checked), no-op
	}

	store := m.storeForFile(task.SourceFile)
	var err error
	if target.Skipped {
		err = store.UnskipCriterion(task.SourceFile, *target)
		if err == nil {
			m.statusNote = "task unskipped"
		}
	} else {
		err = store.SkipCriterion(task.SourceFile, *target)
		if err == nil {
			m.statusNote = "task skipped"
		}
	}
	if err != nil {
		m.statusNote = "error: " + err.Error()
		return m, nil
	}

	// Preserve cursor position across reload so the user stays on the task row.
	prevTreeCursor := m.treeCursor
	m.reloadPlans()
	newItems := m.buildTreeItems()
	if prevTreeCursor < len(newItems) {
		m.treeCursor = prevTreeCursor
	}
	m.syncPlanCursorFromTreeCursor()
	m.syncDetailSuffix()
	return m, nil
}

func (m statusModel) handleApproveToggle() (tea.Model, tea.Cmd) {
	m.statusNote = ""
	if !m.approvalRequired {
		m.statusNote = "approval not required (opt-out mode)"
		return m, nil
	}
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

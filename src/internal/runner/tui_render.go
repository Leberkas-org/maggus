package runner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

// renderBannerView renders the initial startup view before a task is active.
func (m TUIModel) renderBannerView() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)

	var b strings.Builder
	b.WriteString(m.renderHeaderInner(innerW))
	b.WriteString("\n")
	if m.banner.Agent != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Agent:"), m.banner.Agent))
	}
	b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Model:"), m.model))
	b.WriteString(fmt.Sprintf("%s  %d\n", boldStyle.Render("Features:"), m.banner.Iterations))
	if m.banner.Branch != "" {
		b.WriteString(fmt.Sprintf("%s %s\n", boldStyle.Render("Branch:"), m.banner.Branch))
	}
	b.WriteString(fmt.Sprintf("%s %s\n", boldStyle.Render("Run ID:"), m.banner.RunID))
	if m.banner.Worktree != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", boldStyle.Render("Worktree:"), m.banner.Worktree))
	}
	b.WriteString("\n")
	for _, msg := range m.infoMessages {
		b.WriteString(fmt.Sprintf("%s\n", msg))
	}
	if len(m.infoMessages) == 0 {
		b.WriteString(fmt.Sprintf("%s\n", grayStyle.Render("Starting...")))
	}

	footer := styles.StatusBar.Render("ctrl+c stop")

	if m.width > 0 && m.height > 0 {
		is2x := m.banner.TwoXExpiresIn != ""
		borderColor := styles.ThemeColor(is2x)
		return styles.FullScreenLeftColor(b.String(), footer, m.width, m.height, borderColor)
	}
	return styles.Box.Render(b.String()) + "\n"
}

// formatHHMMSS converts a time.Duration to HH:MM:SS format (e.g., "00:02:15", "01:30:00").
func formatHHMMSS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// truncateLeftPath truncates a path from the left, adding "..." prefix.
func truncateLeftPath(path string, maxWidth int) string {
	if maxWidth <= 0 || len(path) <= maxWidth {
		return path
	}
	if maxWidth <= 3 {
		return path[len(path)-maxWidth:]
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

// renderHeaderInner renders the header content for use inside a bordered box.
func (m TUIModel) renderHeaderInner(w int) string {
	if w < 40 {
		w = 40
	}

	var b strings.Builder

	// Line 1: version left, fingerprint right
	left := boldStyle.Render(fmt.Sprintf("Maggus v%s", m.version))
	right := ""
	if m.fingerprint != "" {
		right = grayStyle.Render(m.fingerprint)
	}
	leftRaw := fmt.Sprintf("Maggus v%s", m.version)
	rightRaw := m.fingerprint
	padding := w - len(leftRaw) - len(rightRaw)
	if padding < 2 {
		padding = 2
	}
	b.WriteString(fmt.Sprintf("%s%s%s\n", left, strings.Repeat(" ", padding), right))

	// Line 2: current working directory
	if m.banner.CWD != "" {
		cwdStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		cwdDisplay := truncateLeftPath(m.banner.CWD, w)
		b.WriteString(cwdStyle.Render(cwdDisplay) + "\n")
	}

	// Line 3: 2x remaining time (only when active)
	if m.banner.TwoXExpiresIn != "" {
		twoXStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		b.WriteString(twoXStyle.Render(fmt.Sprintf("2x: %s", m.banner.TwoXExpiresIn)) + "\n")
	}

	// Line 3: progress bar
	if m.totalIters > 0 {
		barWidth := 20
		bar := styles.ProgressBar(m.currentIter, m.totalIters, barWidth)
		var progressText string
		if m.featureMode && m.featureTotal > 0 {
			progressText = fmt.Sprintf("Feature %d/%d, Task %d/%d",
				m.featureCurrent, m.featureTotal, m.currentIter, m.totalIters)
		} else {
			progressText = fmt.Sprintf("%d/%d Tasks", m.currentIter, m.totalIters)
		}
		progress := fmt.Sprintf("[%s] %s", bar, greenStyle.Render(progressText))
		b.WriteString(progress + "\n")
	}

	// Bug hint line (shown when there are active/workable bug tasks)
	if m.activeBugs > 0 {
		bugHintStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		bugText := fmt.Sprintf("%d bugs active", m.activeBugs)
		if m.activeBugs == 1 {
			bugText = "1 bug active"
		}
		b.WriteString(bugHintStyle.Render(bugText) + "\n")
	}

	// Inline notification for new files (below progress bar / bug hint)
	if m.notification != "" && !m.showStopPicker && !m.done {
		// Notification contains styled segments; bugs part is yellow, features part is muted.
		// We render the pre-built text which may contain both.
		notifParts := strings.Split(m.notification, "  ·  ")
		var styledParts []string
		for _, p := range notifParts {
			if strings.Contains(p, "bug") {
				styledParts = append(styledParts, statusStyle.Render(p))
			} else {
				styledParts = append(styledParts, grayStyle.Render(p))
			}
		}
		b.WriteString(strings.Join(styledParts, grayStyle.Render("  ·  ")) + "\n")
	}

	// Stop indicator (when a stop point is set)
	if m.stopAfterTask {
		warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		if m.stopAtTaskID != "" {
			b.WriteString(warnStyle.Render(fmt.Sprintf("⊘ Stopping after %s", m.stopAtTaskID)) + "\n")
		} else {
			b.WriteString(warnStyle.Render("⊘ Stopping after current task") + "\n")
		}
	}

	// Separator line
	b.WriteString(styles.Separator(w) + "\n")

	return b.String()
}

// stopPickerEntry represents a single line in the stop picker: either a group header or a selectable item.
type stopPickerEntry struct {
	isHeader bool
	itemIdx  int // picker item index (-1 for headers)
	label    string
}

// buildStopPickerEntries builds the full list of render entries (headers + items) for the stop picker.
func (m TUIModel) buildStopPickerEntries(maxLabel int) []stopPickerEntry {
	totalItems := m.stopPickerItemCount()
	entries := make([]stopPickerEntry, 0, totalItems+8) // extra capacity for headers

	// Track current group to insert headers on source file change.
	// The current task's item short name is stored in m.itemShort.
	currentGroup := ""

	// Determine the group for "After current task" item.
	// Use the item short name (e.g. "feature_001").
	currentTaskGroup := m.itemShort
	if currentTaskGroup != "" {
		// Insert header for the current task's group
		entries = append(entries, stopPickerEntry{isHeader: true, itemIdx: -1, label: currentTaskGroup})
		currentGroup = currentTaskGroup
	}

	// Item 0: After current task
	label := fmt.Sprintf("After current task (%s)", m.taskID)
	label = styles.Truncate(label, maxLabel)
	entries = append(entries, stopPickerEntry{itemIdx: 0, label: label})

	// Items 1..N: After each remaining task
	for i, t := range m.remainingTasks {
		group := t.SourceFile
		if group != currentGroup && group != "" {
			entries = append(entries, stopPickerEntry{isHeader: true, itemIdx: -1, label: group})
			currentGroup = group
		}
		l := fmt.Sprintf("After %s: %s", t.ID, t.Title)
		l = styles.Truncate(l, maxLabel)
		entries = append(entries, stopPickerEntry{itemIdx: i + 1, label: l})
	}

	// Last item: Complete the plan
	lastIdx := totalItems - 1
	entries = append(entries, stopPickerEntry{itemIdx: lastIdx, label: "Complete the plan"})

	return entries
}

// stopPickerCursorRenderIndex returns the render-line index of the given picker cursor within the entries list.
func stopPickerCursorRenderIndex(entries []stopPickerEntry, cursor int) int {
	for i, e := range entries {
		if !e.isHeader && e.itemIdx == cursor {
			return i
		}
	}
	return 0
}

// renderStopPicker renders the stop point picker overlay.
func (m TUIModel) renderStopPicker(w int) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	checkStyle := lipgloss.NewStyle().Foreground(styles.Success)
	indicatorStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	headerStyle := lipgloss.NewStyle().Foreground(styles.Muted).Faint(true)

	b.WriteString(titleStyle.Render("Stop after…") + "\n")
	b.WriteString(styles.Separator(w) + "\n\n")

	maxLabel := w - 6 // margin for cursor + padding
	if maxLabel < 20 {
		maxLabel = 20
	}

	entries := m.buildStopPickerEntries(maxLabel)
	totalLines := len(entries)

	// Viewport: determine visible slice (in render-line space)
	visible := m.stopPickerVisibleHeight()
	offset := m.stopPickerScroll
	if totalLines <= visible {
		offset = 0
	}

	// Scroll-up indicator
	if offset > 0 {
		b.WriteString(fmt.Sprintf("  %s\n", indicatorStyle.Render("▲ more")))
	}

	// Render only the visible window
	end := offset + visible
	if end > totalLines {
		end = totalLines
	}
	for _, entry := range entries[offset:end] {
		if entry.isHeader {
			headerLabel := styles.Truncate(entry.label, maxLabel)
			b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(fmt.Sprintf("── %s ──", headerLabel))))
		} else {
			m.renderPickerItem(&b, entry.itemIdx, entry.label, selectedStyle, normalStyle, checkStyle)
		}
	}

	// Scroll-down indicator
	if end < totalLines {
		b.WriteString(fmt.Sprintf("  %s\n", indicatorStyle.Render("▼ more")))
	}

	return b.String()
}

// renderPickerItem renders a single stop picker item with cursor and active marker.
func (m TUIModel) renderPickerItem(b *strings.Builder, idx int, label string, selected, normal, check lipgloss.Style) {
	cursor := "  "
	style := normal
	if idx == m.stopPickerCursor {
		cursor = selected.Render("▸ ")
		style = selected
	}

	// Show a check mark if this item is the currently active stop point
	marker := ""
	if m.stopAfterTask {
		lastIdx := m.stopPickerItemCount() - 1
		isActive := false
		switch {
		case idx == 0 && m.stopAtTaskID == "" && idx != lastIdx:
			isActive = true
		case idx > 0 && idx < lastIdx:
			taskIdx := idx - 1
			if taskIdx < len(m.remainingTasks) && m.remainingTasks[taskIdx].ID == m.stopAtTaskID {
				isActive = true
			}
		}
		if isActive {
			marker = " " + check.Render("●")
		}
	}

	b.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, style.Render(label), marker))
}

// renderTabBar renders the horizontal tab bar for the work view.
func (m TUIModel) renderTabBar(w int) string {
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	unselectedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	sep := grayStyle.Render("│")

	labels := []string{
		" Progress ",
		fmt.Sprintf(" Detail (%d) ", m.toolCount),
		" Task ",
		" Commits ",
	}
	if len(m.commits) > 0 {
		labels[3] = fmt.Sprintf(" Commits (%d) ", len(m.commits))
	}

	var parts []string
	for i, label := range labels {
		if i == m.activeTab {
			parts = append(parts, selectedStyle.Render(label))
		} else {
			parts = append(parts, unselectedStyle.Render(label))
		}
	}

	return strings.Join(parts, sep) + "\n" + styles.Separator(w) + "\n"
}

// toolIcon returns an emoji icon for a tool type.
func toolIcon(toolType string) string {
	switch toolType {
	case "Read":
		return "📖"
	case "Edit":
		return "✏️"
	case "Write":
		return "📝"
	case "Bash":
		return "⚡"
	case "Glob":
		return "🔍"
	case "Grep":
		return "🔎"
	case "Skill":
		return "🎯"
	case "Agent":
		return "🤖"
	default:
		if strings.HasPrefix(toolType, "mcp__") {
			return "🔌"
		}
		return "🥚"
	}
}

// detailAvailableHeight returns the number of visible lines in the detail panel viewport.
func (m TUIModel) detailAvailableHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	// Reserve lines for: header section (~5), task info (2), detail header+separator (2), footer (1)
	available := innerH - 10
	if available < 1 {
		available = 1
	}
	return available
}

// progressAvailableHeight returns the number of visible lines in the progress middle zone.
// The middle zone gets all remaining space after the top zone, bottom zone, header, tab bar, and footer.
func (m TUIModel) progressAvailableHeight() int {
	_, innerH := styles.FullScreenInnerSize(m.width, m.height)
	// Header lines (variable, but approximately 5-6 lines):
	//   version+fingerprint (1), CWD (1), optional 2x (0-1), progress bar (0-1),
	//   optional bugs (0-1), optional notification (0-1), optional stop (0-1), separator (1)
	// Task ID line + blank (2)
	// Tab bar + separator (2)
	// Top zone: status (1), output (1), separator (1) = 3
	// Bottom zone: model (1), extras (1), commits (1), elapsed (1), tokens (1), cost (1), separator (1) = 7
	// Footer (1) — handled by FullScreenLeftColor
	headerLines := m.headerLineCount()
	const taskInfoLines = 2   // "TASK-ID: Title" + blank
	const tabBarLines = 2     // tab labels + separator
	const topZoneLines = 3    // status + output + separator
	const bottomZoneLines = 8 // separator + model + extras + commits + elapsed + tokens + per-model tokens + cost
	const footerLine = 1      // footer (accounted for by FullScreenLeftColor)

	reserved := headerLines + taskInfoLines + tabBarLines + topZoneLines + bottomZoneLines + footerLine
	available := innerH - reserved
	if available < 1 {
		available = 1
	}
	return available
}

// headerLineCount returns the number of lines rendered by renderHeaderInner.
func (m TUIModel) headerLineCount() int {
	lines := 1 // version + fingerprint
	if m.banner.CWD != "" {
		lines++ // CWD
	}
	if m.banner.TwoXExpiresIn != "" {
		lines++ // 2x timer
	}
	if m.totalIters > 0 {
		lines++ // progress bar
	}
	if m.activeBugs > 0 {
		lines++ // bug hint
	}
	if m.notification != "" && !m.showStopPicker && !m.done {
		lines++ // notification
	}
	if m.stopAfterTask {
		lines++ // stop indicator
	}
	lines++ // separator
	return lines
}

// clampProgressScroll ensures progressScrollOffset is within valid bounds and updates auto-scroll state.
func clampProgressScroll(m *TUIModel) {
	available := m.progressAvailableHeight()
	maxOffset := m.progressTotalLines - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.progressScrollOffset > maxOffset {
		m.progressScrollOffset = maxOffset
	}
	if m.progressScrollOffset < 0 {
		m.progressScrollOffset = 0
	}
	// Re-enable auto-scroll if scrolled to bottom
	if m.progressScrollOffset >= maxOffset && maxOffset > 0 {
		m.progressAutoScroll = true
	}
}

// clampDetailScroll ensures detailScrollOffset is within valid bounds and updates auto-scroll state.
func clampDetailScroll(m *TUIModel) {
	available := m.detailAvailableHeight()
	maxOffset := m.detailTotalLines - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.detailScrollOffset > maxOffset {
		m.detailScrollOffset = maxOffset
	}
	if m.detailScrollOffset < 0 {
		m.detailScrollOffset = 0
	}
	// Re-enable auto-scroll if scrolled to bottom
	if m.detailScrollOffset >= maxOffset && maxOffset > 0 {
		m.detailAutoScroll = true
	}
}

// countDetailLines calculates the total number of rendered lines for the current tool entries.
func (m TUIModel) countDetailLines() int {
	total := 0
	for i, entry := range m.toolEntries {
		if i > 0 {
			total++ // separator line
		}
		total++ // header line
		total += len(entry.Params)
	}
	return total
}

// renderDetailPanel renders the right-side tool detail panel content.
func (m TUIModel) renderDetailPanel(w, h int) string {
	if w < 10 {
		w = 10
	}

	var b strings.Builder

	if len(m.toolEntries) == 0 {
		b.WriteString(grayStyle.Render("No tool invocations yet.") + "\n")
		return b.String()
	}

	// Build all entry lines
	var entryLines []string
	for i, entry := range m.toolEntries {
		if i > 0 {
			entryLines = append(entryLines, grayStyle.Render(strings.Repeat("·", w)))
		}

		icon := toolIcon(entry.Type)
		styledIcon := cyanStyle.Render(icon)
		ts := entry.Timestamp.Format("15:04:05")
		styledTs := grayStyle.Render(ts)
		desc := entry.Description
		// Reserve 2 extra chars of margin so emojis with inconsistent
		// terminal widths don't push the timestamp to the next line.
		const emojiMargin = 2
		iconW := lipgloss.Width(styledIcon)
		tsW := 8 // "15:04:05" is always 8 chars
		fixedCols := iconW + 1 + 1 + tsW + emojiMargin
		maxDesc := w - fixedCols
		if maxDesc < 0 {
			maxDesc = 0
		}
		desc = styles.Truncate(desc, maxDesc)
		styledDesc := blueStyle.Render(desc)
		// Right-align timestamp: measure the composed left part and pad.
		// Subtract emojiMargin from available width so the ts sits 2 chars from the edge.
		leftW := lipgloss.Width(styledIcon) + 1 + lipgloss.Width(styledDesc)
		pad := (w - emojiMargin) - leftW - tsW
		if pad < 1 {
			pad = 1
		}
		header := styledIcon + " " + styledDesc + strings.Repeat(" ", pad) + styledTs
		entryLines = append(entryLines, header)

		// Sort param keys for stable render order
		paramKeys := make([]string, 0, len(entry.Params))
		for k := range entry.Params {
			paramKeys = append(paramKeys, k)
		}
		sort.Strings(paramKeys)
		for _, k := range paramKeys {
			v := entry.Params[k]
			// "  " indent=2 + key + ":" + space=1 = len(k)+4
			maxVal := w - len(k) - 4
			if maxVal < 0 {
				maxVal = 0
			}
			paramLine := fmt.Sprintf("  %s %s", grayStyle.Render(k+":"), styles.Truncate(v, maxVal))
			entryLines = append(entryLines, paramLine)
		}
	}

	// Viewport calculation
	available := h - 3 // header + separator lines
	if available < 1 {
		available = 1
	}

	// Clamp offset for this render
	offset := m.detailScrollOffset
	maxOffset := len(entryLines) - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	// Scroll indicator when content overflows
	if len(entryLines) > available {
		end := offset + available
		if end > len(entryLines) {
			end = len(entryLines)
		}
		indicator := grayStyle.Render(fmt.Sprintf("[%d-%d of %d]", offset+1, end, len(entryLines)))
		b.WriteString(indicator + "\n")
	}

	// Render visible window
	end := offset + available
	if end > len(entryLines) {
		end = len(entryLines)
	}
	for _, line := range entryLines[offset:end] {
		b.WriteString(line + "\n")
	}

	return b.String()
}

// renderTaskTab renders the task description and acceptance criteria for the Task tab.
func (m TUIModel) renderTaskTab(w int) string {
	var b strings.Builder

	// Task metadata
	labelStyle := styles.Label.Width(12).Align(lipgloss.Right)
	valStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Feature:"), valStyle.Render(m.itemTitle)))

	done := 0
	for _, c := range m.taskCriteria {
		if c.Checked {
			done++
		}
	}
	b.WriteString(fmt.Sprintf("%s  %s\n",
		labelStyle.Render("Criteria:"),
		valStyle.Render(fmt.Sprintf("%d/%d", done, len(m.taskCriteria)))))

	b.WriteString("\n")

	// Description
	if m.taskDescription != "" {
		b.WriteString(styles.Subtitle.Render("Description") + "\n")
		b.WriteString(styles.Separator(w) + "\n")
		for _, line := range strings.Split(m.taskDescription, "\n") {
			if len(line) > w {
				line = styles.Truncate(line, w)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// Acceptance criteria
	if len(m.taskCriteria) > 0 {
		b.WriteString(styles.Subtitle.Render("Acceptance Criteria") + "\n")
		b.WriteString(styles.Separator(w) + "\n")
		for _, c := range m.taskCriteria {
			var icon string
			if c.Checked {
				icon = greenStyle.Render("✓")
			} else if c.Blocked {
				icon = redStyle.Render("⚠")
			} else {
				icon = grayStyle.Render("○")
			}
			text := styles.Truncate(c.Text, w-4)
			b.WriteString(fmt.Sprintf("  %s %s\n", icon, text))
		}
	}

	return b.String()
}

// renderProgressTab renders the 3-zone Progress tab layout:
//   - Top zone (fixed): spinner+status, last output line, separator
//   - Middle zone (scrollable): compact tool list with scroll indicator
//   - Bottom zone (fixed): separator, model, extras, commits, elapsed, tokens (per-model), cost
func (m TUIModel) renderProgressTab(w int) string {
	var b strings.Builder

	contentWidth := w - 11
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Spinner + status color
	spinner := cyanStyle.Render(styles.SpinnerFrames[m.frame])
	sColor := statusStyle
	switch m.status {
	case "Done":
		sColor = greenStyle
		spinner = greenStyle.Render("✓")
	case "Failed":
		sColor = redStyle
		spinner = redStyle.Render("✗")
	case "Interrupted":
		sColor = redStyle
		spinner = redStyle.Render("⊘")
	}

	// ── Top zone (fixed) ──
	b.WriteString(fmt.Sprintf("%s %s  %s\n", spinner, boldStyle.Render("Status:"), sColor.Render(m.status)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Output:"), styles.Truncate(m.output, contentWidth)))
	b.WriteString(styles.Separator(w) + "\n")

	// ── Middle zone (scrollable compact tool list) ──
	available := m.progressAvailableHeight()
	totalTools := len(m.toolEntries)

	if totalTools == 0 {
		b.WriteString(grayStyle.Render("  No tool invocations yet.") + "\n")
		// Fill remaining height
		for i := 1; i < available; i++ {
			b.WriteString("\n")
		}
	} else {
		// Build compact tool lines
		toolLines := make([]string, totalTools)
		for i, entry := range m.toolEntries {
			icon := toolIcon(entry.Type)
			styledType := cyanStyle.Render(entry.Type)
			ts := entry.Timestamp.Format("15:04:05")
			tsW := 8 // "15:04:05" is always 8 chars
			// Fixed overhead: 2 (indent) + iconW + 1 (space) + typeW + 2 (": ") + 2 (spaces before ts) + tsW
			fixedCols := lipgloss.Width(icon) + lipgloss.Width(styledType) + 2 + 1 + 2 + 2 + tsW
			maxDesc := w - fixedCols
			if maxDesc < 1 {
				maxDesc = 1
			}
			desc := styles.Truncate(entry.Description, maxDesc)
			toolLines[i] = fmt.Sprintf("  %s %s: %s  %s",
				icon,
				styledType,
				blueStyle.Render(desc),
				grayStyle.Render(ts))
		}

		// Viewport: clamp offset
		offset := m.progressScrollOffset
		maxOffset := totalTools - available
		if maxOffset < 0 {
			maxOffset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}

		// Scroll indicator
		if totalTools > available {
			end := offset + available
			if end > totalTools {
				end = totalTools
			}
			indicator := grayStyle.Render(fmt.Sprintf("[%d-%d of %d]", offset+1, end, totalTools))
			b.WriteString(indicator + "\n")
			// The indicator takes one line from available
			viewH := available - 1
			if viewH < 1 {
				viewH = 1
			}
			end = offset + viewH
			if end > totalTools {
				end = totalTools
			}
			for _, line := range toolLines[offset:end] {
				b.WriteString(line + "\n")
			}
			// Pad remaining
			rendered := end - offset
			for i := rendered; i < viewH; i++ {
				b.WriteString("\n")
			}
		} else {
			// All fit — no indicator needed
			for _, line := range toolLines {
				b.WriteString(line + "\n")
			}
			// Pad remaining
			for i := totalTools; i < available; i++ {
				b.WriteString("\n")
			}
		}
	}

	// ── Bottom zone (fixed) ──
	b.WriteString(styles.Separator(w) + "\n")

	extrasStr := m.extras
	if extrasStr == "" {
		extrasStr = "-"
	}

	modelDisplay := m.model
	if m.modelIsOverride {
		modelDisplay = m.model + " (task override)"
	}
	b.WriteString(fmt.Sprintf("  %s   %s\n", boldStyle.Render("Model:"), grayStyle.Render(modelDisplay)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Extras:"), cyanStyle.Render(styles.Truncate(extrasStr, contentWidth))))

	// Commits count
	b.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Commits:"), grayStyle.Render(fmt.Sprintf("%d", len(m.commits)))))

	// Elapsed: task + active run + avg per task
	taskElapsed := time.Since(m.startTime).Truncate(time.Second)
	activeRunElapsed := m.ActiveRunElapsed().Truncate(time.Second)
	avgPerTask := m.avgTimePerTask()
	elapsedStr := fmt.Sprintf("Task: %s   ·   Run: %s",
		grayStyle.Render(formatHHMMSS(taskElapsed)),
		grayStyle.Render(formatHHMMSS(activeRunElapsed)))
	if avgPerTask > 0 {
		elapsedStr += fmt.Sprintf("   ·   Avg: %s", grayStyle.Render(formatHHMMSS(avgPerTask)))
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", boldStyle.Render("Elapsed:"), elapsedStr))

	// Tokens — per-model breakdown
	if m.tokens.hasData {
		totalIn := m.tokens.totalInput + m.tokens.totalCacheCreation + m.tokens.totalCacheRead
		var tokenStr string
		if m.tokens.totalCacheCreation > 0 || m.tokens.totalCacheRead > 0 {
			tokenStr = fmt.Sprintf("%s in / %s out (cache: %s write, %s read)",
				FormatTokens(totalIn), FormatTokens(m.tokens.totalOutput),
				FormatTokens(m.tokens.totalCacheCreation), FormatTokens(m.tokens.totalCacheRead))
		} else {
			tokenStr = fmt.Sprintf("%s in / %s out", FormatTokens(totalIn), FormatTokens(m.tokens.totalOutput))
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render(tokenStr)))

		// Per-model token lines
		if len(m.tokens.totalModelUsage) > 0 {
			b.WriteString("          " + grayStyle.Render(m.formatPerModelTokens()) + "\n")
		}

		costStr := "N/A"
		if m.tokens.totalCost > 0 {
			costStr = FormatCost(m.tokens.totalCost)
		}
		b.WriteString(fmt.Sprintf("  %s    %s\n", boldStyle.Render("Cost:"), grayStyle.Render(costStr)))
	} else {
		b.WriteString(fmt.Sprintf("  %s  %s\n", boldStyle.Render("Tokens:"), grayStyle.Render("N/A")))
		b.WriteString(fmt.Sprintf("  %s    %s\n", boldStyle.Render("Cost:"), grayStyle.Render("N/A")))
	}

	return b.String()
}

// formatPerModelTokens formats per-model token usage as a single line.
// e.g., "opus: 45.2k in / 12.1k out  ·  sonnet: 8k in / 2k out"
func (m TUIModel) formatPerModelTokens() string {
	if len(m.tokens.totalModelUsage) == 0 {
		return ""
	}

	// Sort model names for stable output
	names := make([]string, 0, len(m.tokens.totalModelUsage))
	for name := range m.tokens.totalModelUsage {
		names = append(names, name)
	}
	sort.Strings(names)

	var parts []string
	for _, name := range names {
		mt := m.tokens.totalModelUsage[name]
		totalIn := mt.InputTokens + mt.CacheCreationInputTokens + mt.CacheReadInputTokens
		parts = append(parts, fmt.Sprintf("%s: %s in / %s out",
			shortModelName(name), FormatTokens(totalIn), FormatTokens(mt.OutputTokens)))
	}
	return strings.Join(parts, "  ·  ")
}

// shortModelName extracts a short display name from a full model ID.
// e.g., "claude-opus-4-6" → "opus", "claude-sonnet-4-6" → "sonnet"
func shortModelName(fullName string) string {
	// Known patterns
	for _, short := range []string{"opus", "sonnet", "haiku"} {
		if strings.Contains(strings.ToLower(fullName), short) {
			return short
		}
	}
	// Fallback: return the full name, truncated
	if len(fullName) > 20 {
		return fullName[:20]
	}
	return fullName
}

// avgTimePerTask calculates the average elapsed time per completed task from usage history.
func (m TUIModel) avgTimePerTask() time.Duration {
	if len(m.tokens.usages) == 0 {
		return 0
	}
	var total time.Duration
	for _, u := range m.tokens.usages {
		total += u.EndTime.Sub(u.StartTime)
	}
	return (total / time.Duration(len(m.tokens.usages))).Truncate(time.Second)
}

// renderView renders the main work-in-progress view with tabs.
func (m TUIModel) renderView() string {
	innerW, _ := styles.FullScreenInnerSize(m.width, m.height)

	var b strings.Builder

	// Render header inside the box (full width)
	b.WriteString(m.renderHeaderInner(innerW))

	// Render task info (full width)
	if m.taskID != "" {
		taskLine := fmt.Sprintf("%s %s", cyanStyle.Render(m.taskID+":"), m.taskTitle)
		b.WriteString(taskLine + "\n\n")
	}

	// Stop picker overlay replaces tab content when active
	if m.showStopPicker {
		b.WriteString(m.renderStopPicker(innerW))
	} else {
		// Tab bar
		b.WriteString(m.renderTabBar(innerW))

		// Tab content
		switch m.activeTab {
		case 0: // Progress — 3-zone layout
			b.WriteString(m.renderProgressTab(innerW))

		case 1: // Detail (tool log)
			b.WriteString(m.renderDetailPanel(innerW, m.detailAvailableHeight()))

		case 2: // Task
			b.WriteString(m.renderTaskTab(innerW))

		case 3: // Commits
			if len(m.commits) == 0 {
				b.WriteString(grayStyle.Render("No commits yet.") + "\n")
			} else {
				for _, c := range m.commits {
					line := styles.Truncate(c, innerW-4)
					b.WriteString(fmt.Sprintf("  %s %s\n",
						grayStyle.Render("•"),
						grayStyle.Render(line)))
				}
			}
		}
	}

	// Footer with context-sensitive keybindings
	var footer string
	if m.showStopPicker {
		footer = styles.StatusBar.Render("↑/↓ select · enter confirm · esc cancel")
	} else {
		var footerParts []string
		footerParts = append(footerParts, "←/→ tabs")
		if m.activeTab == 0 || m.activeTab == 1 {
			footerParts = append(footerParts, "↑/↓ scroll · home/end jump")
		}
		if m.stopAfterTask {
			footerParts = append(footerParts, "alt+s change stop point")
		} else {
			footerParts = append(footerParts, "alt+s stop")
		}
		footerParts = append(footerParts, "ctrl+c stop now")
		footer = styles.StatusBar.Render(strings.Join(footerParts, " · "))
	}

	// Border color: 2x mode → yellow, stop-after-task → yellow, otherwise → cyan.
	// Both 2x and stop use Warning, so they combine naturally.
	if m.width > 0 && m.height > 0 {
		is2x := m.banner.TwoXExpiresIn != ""
		borderColor := styles.ThemeColor(is2x)
		if m.stopAfterTask || m.showStopPicker {
			borderColor = styles.Warning
		}
		return styles.FullScreenLeftColor(b.String(), footer, m.width, m.height, borderColor)
	}
	return styles.Box.Render(b.String()) + "\n"
}

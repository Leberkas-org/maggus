package runner

import tea "github.com/charmbracelet/bubbletea"

// handleKeyMsg processes all key events during the normal work view.
// Sync and summary/done screens are handled before this is called.
func (m *TUIModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C on summary exits immediately
	if m.summary.show && msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// Ctrl+C cancels the running agent
	if msg.Type == tea.KeyCtrlC {
		m.showStopPicker = false
		m.status = "Interrupting..."
		if m.cancelFunc != nil {
			m.cancelFunc()
			m.cancelFunc = nil
		}
		return m, nil
	}

	// Handle stop picker when active
	if m.showStopPicker {
		return m.handleStopPicker(msg)
	}

	// Alt+S opens/closes the stop picker
	if m.taskID != "" && !m.summary.show && msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 's' || msg.Runes[0] == 'S') {
		m.showStopPicker = true
		m.stopPickerCursor = 0
		// If a stop point is already set, pre-select it in the picker
		if m.stopAfterTask {
			m.stopPickerCursor = m.stopPickerCurrentIndex()
		}
		return m, nil
	}

	// Detail tab scrolling (tab 1)
	if m.activeTab == 1 && m.taskID != "" {
		if cmd, handled := m.handleDetailScroll(msg); handled {
			return m, cmd
		}
	}

	// Tab switching
	if m.taskID != "" {
		if cmd, handled := m.handleTabSwitch(msg); handled {
			return m, cmd
		}
	}

	return m, nil
}

// stopPickerItemCount returns the total number of items in the stop picker.
// Layout: "After current task" + one per remaining task + "Complete the plan".
func (m TUIModel) stopPickerItemCount() int {
	return 1 + len(m.remainingTasks) + 1
}

// stopPickerCurrentIndex returns the picker index that matches the current stop point.
func (m TUIModel) stopPickerCurrentIndex() int {
	if m.stopAtTaskID == "" {
		return 0 // "After current task"
	}
	for i, t := range m.remainingTasks {
		if t.ID == m.stopAtTaskID {
			return i + 1
		}
	}
	return 0
}

// handleStopPicker processes keys while the stop picker is visible.
func (m TUIModel) handleStopPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxIdx := m.stopPickerItemCount() - 1

	switch msg.Type {
	case tea.KeyUp:
		if m.stopPickerCursor > 0 {
			m.stopPickerCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.stopPickerCursor < maxIdx {
			m.stopPickerCursor++
		}
		return m, nil
	case tea.KeyEnter:
		return m.applyStopPickerSelection()
	case tea.KeyEscape:
		m.showStopPicker = false
		return m, nil
	}

	// Alt+S again closes the picker
	if msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 's' || msg.Runes[0] == 'S') {
		m.showStopPicker = false
		return m, nil
	}

	return m, nil
}

// applyStopPickerSelection applies the user's choice from the stop picker.
func (m TUIModel) applyStopPickerSelection() (tea.Model, tea.Cmd) {
	m.showStopPicker = false
	lastIdx := m.stopPickerItemCount() - 1

	switch {
	case m.stopPickerCursor == 0:
		// "After current task"
		m.stopAfterTask = true
		m.stopAtTaskID = ""
		m.stopFlag.Store(true)
		m.stopAtTaskIDFlag.Store("")

	case m.stopPickerCursor == lastIdx:
		// "Complete the plan" — cancel any stop
		m.stopAfterTask = false
		m.stopAtTaskID = ""
		m.stopFlag.Store(false)
		m.stopAtTaskIDFlag.Store("")

	default:
		// "After TASK-XXX"
		idx := m.stopPickerCursor - 1
		if idx >= 0 && idx < len(m.remainingTasks) {
			taskID := m.remainingTasks[idx].ID
			m.stopAfterTask = true
			m.stopAtTaskID = taskID
			m.stopFlag.Store(true)
			m.stopAtTaskIDFlag.Store(taskID)
		}
	}

	return m, nil
}

// handleDetailScroll processes scroll keys for the detail tab.
// Returns the command and true if the key was handled.
func (m *TUIModel) handleDetailScroll(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp:
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
			m.detailAutoScroll = false
		}
		return nil, true
	case tea.KeyDown:
		m.detailScrollOffset++
		clampDetailScroll(m)
		return nil, true
	case tea.KeyHome:
		m.detailScrollOffset = 0
		m.detailAutoScroll = false
		return nil, true
	case tea.KeyEnd:
		m.detailScrollOffset = m.detailTotalLines
		m.detailAutoScroll = true
		clampDetailScroll(m)
		return nil, true
	}
	return nil, false
}

// handleTabSwitch processes tab-switching keys (arrow keys and number keys).
// Returns the command and true if the key was handled.
func (m *TUIModel) handleTabSwitch(msg tea.KeyMsg) (tea.Cmd, bool) {
	const maxTab = 3
	switch msg.Type {
	case tea.KeyLeft:
		if m.activeTab > 0 {
			m.activeTab--
		}
		return nil, true
	case tea.KeyRight:
		if m.activeTab < maxTab {
			m.activeTab++
		}
		return nil, true
	}
	if len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case '1':
			m.activeTab = 0
			return nil, true
		case '2':
			m.activeTab = 1
			return nil, true
		case '3':
			m.activeTab = 2
			return nil, true
		case '4':
			m.activeTab = 3
			return nil, true
		}
	}
	return nil, false
}

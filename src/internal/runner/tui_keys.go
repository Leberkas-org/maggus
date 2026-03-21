package runner

import tea "github.com/charmbracelet/bubbletea"

// handleKeyMsg processes all key events during the normal work view.
// Sync and summary/done screens are handled before this is called.
func (m TUIModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C on summary exits immediately
	if m.summary.show && msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	// Ctrl+C cancels the running agent
	if msg.Type == tea.KeyCtrlC {
		m.confirmingStop = false
		m.status = "Interrupting..."
		if m.cancelFunc != nil {
			m.cancelFunc()
			m.cancelFunc = nil
		}
		return m, nil
	}

	// Handle stop-after-task confirmation prompt
	if m.confirmingStop {
		return m.handleStopConfirmation(msg)
	}

	// Alt+S toggles stop-after-task (confirm to enable, instant to revert)
	if m.taskID != "" && !m.summary.show && msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 's' || msg.Runes[0] == 'S') {
		if m.stopAfterTask {
			m.stopAfterTask = false
			m.stopFlag.Store(false)
		} else {
			m.confirmingStop = true
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

// handleStopConfirmation processes keys while the "stop after task?" prompt is visible.
func (m TUIModel) handleStopConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'y', 'Y':
			m.confirmingStop = false
			m.stopAfterTask = true
			m.stopFlag.Store(true)
			return m, nil
		case 'n', 'N':
			m.confirmingStop = false
			return m, nil
		}
	}
	if msg.Type == tea.KeyEscape {
		m.confirmingStop = false
		return m, nil
	}
	return m, nil
}

// handleDetailScroll processes scroll keys for the detail tab.
// Returns the command and true if the key was handled.
func (m TUIModel) handleDetailScroll(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp:
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
			m.detailAutoScroll = false
		}
		return nil, true
	case tea.KeyDown:
		m.detailScrollOffset++
		clampDetailScroll(&m)
		return nil, true
	case tea.KeyHome:
		m.detailScrollOffset = 0
		m.detailAutoScroll = false
		return nil, true
	case tea.KeyEnd:
		m.detailScrollOffset = m.detailTotalLines
		m.detailAutoScroll = true
		clampDetailScroll(&m)
		return nil, true
	}
	return nil, false
}

// handleTabSwitch processes tab-switching keys (arrow keys and number keys).
// Returns the command and true if the key was handled.
func (m TUIModel) handleTabSwitch(msg tea.KeyMsg) (tea.Cmd, bool) {
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

package runner

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/parser"
)

// handleIterationStart resets per-iteration state when a new task begins.
func (m *TUIModel) handleIterationStart(msg IterationStartMsg) {
	m.tokens.saveAndReset(m.taskID, m.taskTitle, m.taskFeatureFile, m.startTime)
	m.currentIter = msg.Current
	m.totalIters = max(msg.Total, m.totalIters)
	m.taskID = msg.TaskID
	m.taskTitle = msg.TaskTitle
	m.taskFeatureFile = msg.FeatureFile
	m.taskDescription = msg.TaskDescription
	m.taskCriteria = msg.TaskCriteria
	m.remainingTasks = msg.RemainingTasks
	// Reset per-iteration state
	m.status = "Starting..."
	m.output = "-"
	m.toolEntries = nil
	m.toolCount = 0
	m.extras = ""
	m.skills = nil
	m.mcps = nil
	m.detailScrollOffset = 0
	m.detailAutoScroll = true
	m.detailTotalLines = 0
	m.startTime = time.Now()
}

// handleOutputMsg updates the last-line output display.
func (m *TUIModel) handleOutputMsg(msg agent.OutputMsg) {
	text := strings.TrimSpace(msg.Text)
	if idx := strings.LastIndex(text, "\n"); idx >= 0 {
		text = strings.TrimSpace(text[idx+1:])
	}
	if text != "" {
		m.output = text
	}
}

// handleToolMsg appends a tool entry and updates scroll state.
func (m *TUIModel) handleToolMsg(msg agent.ToolMsg) {
	m.toolEntries = append(m.toolEntries, msg)
	m.toolCount++
	m.detailTotalLines = m.countDetailLines()
	if m.detailAutoScroll {
		m.detailScrollOffset = m.detailTotalLines
	}
	clampDetailScroll(m)
}

// handleSkillMsg adds a unique skill name and rebuilds the extras display.
func (m *TUIModel) handleSkillMsg(msg agent.SkillMsg) {
	for _, s := range m.skills {
		if s == msg.Name {
			return
		}
	}
	m.skills = append(m.skills, msg.Name)
	m.rebuildExtras()
}

// handleMCPMsg adds a unique MCP name and rebuilds the extras display.
func (m *TUIModel) handleMCPMsg(msg agent.MCPMsg) {
	for _, s := range m.mcps {
		if s == msg.Name {
			return
		}
	}
	m.mcps = append(m.mcps, msg.Name)
	m.rebuildExtras()
}

// handleCommitMsg appends a commit message, keeping at most maxCommitHistory entries.
func (m *TUIModel) handleCommitMsg(msg CommitMsg) {
	m.commits = append(m.commits, msg.Message)
	if len(m.commits) > maxCommitHistory {
		m.commits = m.commits[len(m.commits)-maxCommitHistory:]
	}
}

// rebuildExtras rebuilds the extras display string from skills and MCPs.
func (m *TUIModel) rebuildExtras() {
	var parts []string
	for _, s := range m.skills {
		parts = append(parts, "skill:"+s)
	}
	for _, s := range m.mcps {
		parts = append(parts, "mcp:"+s)
	}
	m.extras = strings.Join(parts, "  ")
}

// handleFileChange re-parses all feature and bug files to update totalIters and activeBugs.
// currentIter is NOT changed — only totalIters is adjusted.
// Returns a tea.Cmd to schedule notification timeout if new tasks were detected.
func (m *TUIModel) handleFileChange() tea.Cmd {
	if m.workDir == "" {
		return nil
	}

	bugTasks, bugErr := parser.ParseBugs(m.workDir)
	if bugErr != nil {
		return nil
	}
	featureTasks, featureErr := parser.ParseFeatures(m.workDir)
	if featureErr != nil {
		return nil
	}

	workableBugs := 0
	workableFeatures := 0
	for i := range bugTasks {
		if bugTasks[i].IsWorkable() {
			workableBugs++
		}
	}
	for i := range featureTasks {
		if featureTasks[i].IsWorkable() {
			workableFeatures++
		}
	}

	m.totalIters = m.currentIter + workableBugs + workableFeatures
	m.activeBugs = workableBugs

	// Detect new tasks compared to previous counts
	newBugs := workableBugs - m.prevWorkableBugs
	newFeatures := workableFeatures - m.prevWorkableFeatures
	m.prevWorkableBugs = workableBugs
	m.prevWorkableFeatures = workableFeatures

	if newBugs > 0 || newFeatures > 0 {
		return m.setNotification(newBugs, newFeatures)
	}
	return nil
}

// setNotification builds the notification text and returns a delayed Cmd to clear it.
func (m *TUIModel) setNotification(newBugs, newFeatures int) tea.Cmd {
	var parts []string
	if newBugs > 0 {
		label := "bugs"
		if newBugs == 1 {
			label = "bug"
		}
		parts = append(parts, fmt.Sprintf("+%d %s added (will run next)", newBugs, label))
	}
	if newFeatures > 0 {
		label := "features"
		if newFeatures == 1 {
			label = "feature"
		}
		parts = append(parts, fmt.Sprintf("+%d %s added", newFeatures, label))
	}
	m.notification = strings.Join(parts, "  ·  ")
	m.notificationTimerID++
	timerID := m.notificationTimerID
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return notificationExpiredMsg{timerID: timerID}
	})
}

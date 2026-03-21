package runner

import (
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// handleIterationStart resets per-iteration state when a new task begins.
func (m *TUIModel) handleIterationStart(msg IterationStartMsg) {
	m.tokens.saveAndReset(m.taskID, m.taskTitle, m.taskPlanFile, m.startTime)
	m.currentIter = msg.Current
	m.totalIters = msg.Total
	m.taskID = msg.TaskID
	m.taskTitle = msg.TaskTitle
	m.taskPlanFile = msg.PlanFile
	m.taskDescription = msg.TaskDescription
	m.taskCriteria = msg.TaskCriteria
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

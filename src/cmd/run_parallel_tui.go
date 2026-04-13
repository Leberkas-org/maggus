package cmd

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// workerSnapshotWriter implements agent.MessageSender for a single parallel worker.
// It accumulates tool invocations, token usage, and status updates, writing a
// per-worker snapshot file (state-<taskID>.json) on each significant event.
// This enables the status view to render a split pane per active worker.
type workerSnapshotWriter struct {
	mu          sync.Mutex
	dir         string
	maggusID    string
	taskID      string
	taskTitle   string
	itemTitle   string
	status      string
	toolEntries []runlog.SnapshotToolEntry
	tokenInput  int
	tokenOutput int
	tokenCost   float64
	modelUsage  map[string]agent.ModelTokens
	commits     []string
	runStarted  time.Time
	taskStarted time.Time
}

func newWorkerSnapshotWriter(dir, maggusID, taskID, taskTitle, itemTitle string, runStarted time.Time) *workerSnapshotWriter {
	w := &workerSnapshotWriter{
		dir:         dir,
		maggusID:    maggusID,
		taskID:      taskID,
		taskTitle:   taskTitle,
		itemTitle:   itemTitle,
		status:      "Working",
		runStarted:  runStarted,
		taskStarted: time.Now(),
	}
	w.writeSnapshot()
	return w
}

// Send processes a tea.Msg from the agent, updates internal state, and writes the snapshot.
func (w *workerSnapshotWriter) Send(msg tea.Msg) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch msg := msg.(type) {
	case agent.StatusMsg:
		w.status = msg.Status
	case agent.ToolMsg:
		w.toolEntries = append(w.toolEntries, runlog.SnapshotToolEntry{
			Type:        msg.Type,
			Icon:        toolIconForSnapshot(msg.Type),
			Description: msg.Description,
			Timestamp:   msg.Timestamp.UTC().Format(time.RFC3339),
		})
	case agent.UsageMsg:
		w.tokenInput += msg.InputTokens
		w.tokenOutput += msg.OutputTokens
		w.tokenCost += msg.CostUSD
	case agent.ModelUsageMsg:
		if w.modelUsage == nil {
			w.modelUsage = make(map[string]agent.ModelTokens)
		}
		for name, entry := range msg.Models {
			existing := w.modelUsage[name]
			existing.InputTokens += entry.InputTokens
			existing.OutputTokens += entry.OutputTokens
			existing.CacheCreationInputTokens += entry.CacheCreationInputTokens
			existing.CacheReadInputTokens += entry.CacheReadInputTokens
			existing.CostUSD += entry.CostUSD
			w.modelUsage[name] = existing
		}
	case CommitMsg:
		w.commits = append(w.commits, msg.Message)
	case agent.OutputMsg, agent.SkillMsg, agent.MCPMsg:
		// No snapshot state to update; skip write.
		return
	default:
		return
	}

	w.writeSnapshot()
}

// SetStatus updates the worker status and writes the snapshot. Called by the
// orchestrator when a task finishes (Done/Failed/Blocked).
func (w *workerSnapshotWriter) SetStatus(status string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
	w.writeSnapshot()
}

func (w *workerSnapshotWriter) writeSnapshot() {
	snap := runlog.StateSnapshot{
		MaggusID:       w.maggusID,
		TaskID:         w.taskID,
		TaskTitle:      w.taskTitle,
		ItemTitle:      w.itemTitle,
		Status:         w.status,
		ToolEntries:    w.toolEntries,
		TokenInput:     w.tokenInput,
		TokenOutput:    w.tokenOutput,
		TokenCost:      w.tokenCost,
		ModelBreakdown: w.modelUsage,
		Commits:        w.commits,
		RunStartedAt:   w.runStarted.UTC().Format(time.RFC3339),
		TaskStartedAt:  w.taskStarted.UTC().Format(time.RFC3339),
	}
	_ = runlog.WriteWorkerSnapshot(w.dir, w.taskID, snap)
}

// workerEntry tracks a worker's identity and status in the index.
type workerEntry struct {
	taskID    string
	taskTitle string
	status    string // "working", "done", "failed", "blocked"
	startedAt string
}

// cleanupWorkerSnapshots removes all per-worker snapshot files and the index.
func cleanupWorkerSnapshots(dir string) {
	runlog.RemoveAllWorkerSnapshots(dir)
}

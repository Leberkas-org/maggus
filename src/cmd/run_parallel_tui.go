package cmd

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// workerSnapshotWriter implements agent.MessageSender for a single parallel worker.
// It accumulates tool invocations, token usage, and status updates, writing a
// per-worker snapshot file (state-<taskID>.json) on each significant event.
// This enables the status view to render a split pane per active worker.
type workerSnapshotWriter struct {
	mu          sync.Mutex
	dir         string
	runID       string
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

func newWorkerSnapshotWriter(dir, runID, taskID, taskTitle, itemTitle string, runStarted time.Time) *workerSnapshotWriter {
	w := &workerSnapshotWriter{
		dir:         dir,
		runID:       runID,
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
		RunID:          w.runID,
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

// updateWorkersIndex writes the workers index file. Must be called with o.mu held.
func (o *parallelOrchestrator) updateWorkersIndex(tasks []workerEntry) {
	entries := make([]runlog.WorkerIndexEntry, len(tasks))
	for i, t := range tasks {
		entries[i] = runlog.WorkerIndexEntry{
			TaskID:    t.taskID,
			TaskTitle: t.taskTitle,
			Status:    t.status,
			StartedAt: t.startedAt,
		}
	}
	_ = runlog.WriteWorkersIndex(o.repoDir, entries)
}

// workerEntry tracks a worker's identity and status in the index.
type workerEntry struct {
	taskID    string
	taskTitle string
	status    string // "working", "done", "failed", "blocked"
	startedAt string
}

// buildWorkerEntries returns the current worker entries from tracking maps.
// Must be called with o.mu held.
func (o *parallelOrchestrator) buildWorkerEntries() []workerEntry {
	var entries []workerEntry
	for _, id := range o.workerOrder {
		entry := workerEntry{
			taskID:    id,
			taskTitle: o.workerTitles[id],
			status:    o.workerStatuses[id],
			startedAt: o.workerStartedAt[id],
		}
		entries = append(entries, entry)
	}
	return entries
}

// setWorkerStatus records a worker's status and updates the index file.
// Must be called with o.mu held.
func (o *parallelOrchestrator) setWorkerStatus(taskID, status string) {
	o.workerStatuses[taskID] = status
	o.updateWorkersIndex(o.buildWorkerEntries())
}

// cleanupWorkerSnapshots removes all per-worker snapshot files and the index.
func cleanupWorkerSnapshots(dir string) {
	runlog.RemoveAllWorkerSnapshots(dir)
}

// hasWorkerMaps returns true if the worker tracking maps are initialized.
func (o *parallelOrchestrator) hasWorkerMaps() bool {
	return o.workerStatuses != nil
}

// ensureWorkerMaps initializes the worker tracking maps if not already done.
func (o *parallelOrchestrator) ensureWorkerMaps() {
	if o.workerStatuses == nil {
		o.workerStatuses = make(map[string]string)
		o.workerTitles = make(map[string]string)
		o.workerStartedAt = make(map[string]string)
	}
}

// registerWorker adds a worker to the tracking maps. Must be called with o.mu held.
// If the task ID is already registered, this is a no-op (prevents duplicate entries on retry).
func (o *parallelOrchestrator) registerWorker(taskID, taskTitle string) {
	o.ensureWorkerMaps()
	if _, exists := o.workerStatuses[taskID]; exists {
		return
	}
	o.workerOrder = append(o.workerOrder, taskID)
	o.workerTitles[taskID] = taskTitle
	o.workerStatuses[taskID] = "working"
	o.workerStartedAt[taskID] = time.Now().UTC().Format(time.RFC3339)
}

// workerSnapshotWriterFor creates a per-worker snapshot writer and registers
// the worker in the index. Must be called with o.mu held.
func (o *parallelOrchestrator) workerSnapshotWriterFor(task parser.Task) *workerSnapshotWriter {
	o.registerWorker(task.ID, task.Title)
	o.updateWorkersIndex(o.buildWorkerEntries())
	return newWorkerSnapshotWriter(o.repoDir, o.runID, task.ID, task.Title, "", o.runStartedAt)
}

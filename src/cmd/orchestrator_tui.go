package cmd

import (
	"time"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// updateWorkersIndex writes the workers index file. Must be called with o.mu held.
func (o *Orchestrator) updateWorkersIndex(tasks []workerEntry) {
	entries := make([]runlog.WorkerIndexEntry, len(tasks))
	for i, t := range tasks {
		entries[i] = runlog.WorkerIndexEntry{
			TaskID:    t.taskID,
			TaskTitle: t.taskTitle,
			Status:    t.status,
			StartedAt: t.startedAt,
		}
	}
	_ = runlog.WriteWorkersIndex(o.cfg.RepoDir, entries)
}

// buildWorkerEntries returns the current worker entries from tracking maps.
// Must be called with o.mu held.
func (o *Orchestrator) buildWorkerEntries() []workerEntry {
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
func (o *Orchestrator) setWorkerStatus(taskID, status string) {
	o.workerStatuses[taskID] = status
	o.updateWorkersIndex(o.buildWorkerEntries())
}

// hasWorkerMaps returns true if the worker tracking maps are initialized.
func (o *Orchestrator) hasWorkerMaps() bool {
	return o.workerStatuses != nil
}

// ensureWorkerMaps initializes the worker tracking maps if not already done.
func (o *Orchestrator) ensureWorkerMaps() {
	if o.workerStatuses == nil {
		o.workerStatuses = make(map[string]string)
		o.workerTitles = make(map[string]string)
		o.workerStartedAt = make(map[string]string)
	}
}

// registerWorker adds a worker to the tracking maps. Must be called with o.mu held.
// If the task ID is already registered, this is a no-op (prevents duplicate entries on retry).
func (o *Orchestrator) registerWorker(taskID, taskTitle string) {
	o.ensureWorkerMaps()
	if _, exists := o.workerStatuses[taskID]; exists {
		return
	}
	o.workerOrder = append(o.workerOrder, taskID)
	o.workerTitles[taskID] = taskTitle
	o.workerStatuses[taskID] = "working"
	o.workerStartedAt[taskID] = time.Now().UTC().Format(time.RFC3339)
}

// newWorkerSnapshotWriterForTask creates a per-worker snapshot writer for a parallel task.
// Called outside of o.mu (does not acquire the lock).
func (o *Orchestrator) newWorkerSnapshotWriterForTask(task parser.Task, group *parser.Plan) *workerSnapshotWriter {
	maggusID := ""
	itemTitle := ""
	if group != nil {
		maggusID = group.MaggusID
		itemTitle = group.Title
	}
	return newWorkerSnapshotWriter(o.cfg.RepoDir, maggusID, task.ID, task.Title, itemTitle, o.cfg.StartTime)
}

// markWorkerDone updates a worker's status to "done" in the index and snapshot.
func (o *Orchestrator) markWorkerDone(taskID string, wsw *workerSnapshotWriter) {
	if wsw != nil {
		wsw.SetStatus("Done")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "done")
	}
	o.mu.Unlock()
}

// markWorkerFailed updates a worker's status to "failed" in the index and snapshot.
func (o *Orchestrator) markWorkerFailed(taskID string, wsw *workerSnapshotWriter) {
	if wsw != nil {
		wsw.SetStatus("Failed")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "failed")
	}
	o.mu.Unlock()
}

// markWorkerBlocked updates a worker's status to "blocked" in the index and snapshot.
func (o *Orchestrator) markWorkerBlocked(taskID string, wsw *workerSnapshotWriter) {
	if wsw != nil {
		wsw.SetStatus("Blocked")
	}
	o.mu.Lock()
	if o.hasWorkerMaps() {
		o.setWorkerStatus(taskID, "blocked")
	}
	o.mu.Unlock()
}

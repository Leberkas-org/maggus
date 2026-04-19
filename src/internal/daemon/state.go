package daemon

import (
	"sync"

	"github.com/leberkas-org/maggus/internal/ipc"
)

type State struct {
	mu      sync.RWMutex
	writer  ipc.StateWriter
	queue   *TaskQueue
	workers map[string]*WorkerState
}

type WorkerState struct {
	TaskID      string
	ItemID      string
	TaskTitle   string
	RepoURL     string
	Status      string
	AgentOutput []string
	Usage       ipc.TokenUsage
}

func NewState(writer ipc.StateWriter, queue *TaskQueue) *State {
	return &State{
		writer:  writer,
		queue:   queue,
		workers: make(map[string]*WorkerState),
	}
}

func (s *State) SetWorker(taskID string, ws *WorkerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[taskID] = ws
}

func (s *State) RemoveWorker(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workers, taskID)
}

func (s *State) UpdateWorkerStatus(taskID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ws, ok := s.workers[taskID]; ok {
		ws.Status = status
	}
}

func (s *State) AppendWorkerOutput(taskID, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ws, ok := s.workers[taskID]; ok {
		ws.AgentOutput = append(ws.AgentOutput, line)
		if len(ws.AgentOutput) > 100 {
			ws.AgentOutput = ws.AgentOutput[len(ws.AgentOutput)-100:]
		}
	}
}

func (s *State) Flush() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := s.buildSnapshot()
	return s.writer.WriteState(snap)
}

func (s *State) buildSnapshot() ipc.DaemonSnapshot {
	snap := ipc.DaemonSnapshot{
		ActiveTasks: len(s.workers),
	}

	for _, item := range s.queue.All() {
		done := 0
		for _, t := range item.Tasks {
			if t.Status == ItemDone {
				done++
			}
		}
		snap.Queue = append(snap.Queue, ipc.QueueItem{
			ID:       item.ID,
			Title:    item.Title,
			RepoURL:  item.RepoURL,
			Status:   string(item.Status),
			Priority: item.Priority,
			Tasks:    len(item.Tasks),
			Done:     done,
		})
	}

	for _, ws := range s.workers {
		output := ""
		if len(ws.AgentOutput) > 0 {
			output = ws.AgentOutput[len(ws.AgentOutput)-1]
		}
		snap.Workers = append(snap.Workers, ipc.WorkerSnapshot{
			TaskID:      ws.TaskID,
			ItemID:      ws.ItemID,
			TaskTitle:   ws.TaskTitle,
			RepoURL:     ws.RepoURL,
			Status:      ws.Status,
			AgentOutput: output,
			TokenUsage:  ws.Usage,
		})
	}

	return snap
}

package daemon

import (
	"log/slog"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/runlog"
)

type Dispatcher struct {
	queue  *TaskQueue
	pool   *WorkerPool
	gitOps git.Operations
	agents *agent.Registry
	cfg    config.Config
	state  *State
	log    *slog.Logger
}

func NewDispatcher(queue *TaskQueue, pool *WorkerPool, gitOps git.Operations, agents *agent.Registry, cfg config.Config, state *State, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		queue:  queue,
		pool:   pool,
		gitOps: gitOps,
		agents: agents,
		cfg:    cfg,
		state:  state,
		log:    logger,
	}
}

func (d *Dispatcher) Tick() {
	if d.pool.ActiveCount() >= d.cfg.MaxWorkers {
		return
	}

	item := d.queue.NextReady()
	if item == nil {
		return
	}

	var nextTask *TaskSpec
	for i := range item.Tasks {
		t := &item.Tasks[i]
		if t.Status != "" && t.Status != ItemPending {
			continue
		}
		if !predecessorsDone(t, item) {
			continue
		}
		nextTask = t
		break
	}
	if nextTask == nil {
		return
	}

	ag, err := d.agents.Get(d.cfg.Agent)
	if err != nil {
		d.log.Error("get agent failed", "agent", d.cfg.Agent, "error", err)
		return
	}

	logsDir := item.RepoPath
	if logsDir == "" {
		logsDir = "."
	}

	logger, err := runlog.New(logsDir+"/.maggus/logs", item.ID)
	if err != nil {
		d.log.Warn("create run logger failed", "item_id", item.ID, "error", err)
	}

	item.Status = ItemActive
	nextTask.Status = ItemActive

	worker := NewTaskWorker(item, *nextTask, d.cfg, d.state, d.gitOps, ag, d.pool, logger, d.log)
	if err := d.pool.Submit(worker); err != nil {
		d.log.Warn("submit worker failed", "task_id", nextTask.ID, "error", err)
		nextTask.Status = ItemPending
		item.Status = ItemReady
	}

	_ = d.state.Flush()
}

func predecessorsDone(task *TaskSpec, item *WorkItem) bool {
	if len(task.Predecessors) == 0 {
		return true
	}
	for _, predID := range task.Predecessors {
		for _, t := range item.Tasks {
			if t.ID == predID && t.Status != ItemDone {
				return false
			}
		}
	}
	return true
}

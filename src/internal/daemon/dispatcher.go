package daemon

import (
	"log"

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
}

func NewDispatcher(queue *TaskQueue, pool *WorkerPool, gitOps git.Operations, agents *agent.Registry, cfg config.Config, state *State) *Dispatcher {
	return &Dispatcher{
		queue:  queue,
		pool:   pool,
		gitOps: gitOps,
		agents: agents,
		cfg:    cfg,
		state:  state,
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

	// Find next pending task in the item
	var nextTask *TaskSpec
	for i := range item.Tasks {
		if item.Tasks[i].Status == "" || item.Tasks[i].Status == ItemPending {
			nextTask = &item.Tasks[i]
			break
		}
	}
	if nextTask == nil {
		return
	}

	ag, err := d.agents.Get(d.cfg.Agent)
	if err != nil {
		log.Printf("get agent: %v", err)
		return
	}

	logsDir := item.RepoPath
	if logsDir == "" {
		logsDir = "."
	}

	logger, err := runlog.New(logsDir+"/.maggus/logs", item.ID)
	if err != nil {
		log.Printf("create logger: %v", err)
	}

	item.Status = ItemActive
	nextTask.Status = ItemActive

	worker := NewTaskWorker(item, *nextTask, d.cfg, d.state, d.gitOps, ag, d.pool, logger)
	if err := d.pool.Submit(worker); err != nil {
		log.Printf("submit worker: %v", err)
		nextTask.Status = ItemPending
		item.Status = ItemReady
	}

	_ = d.state.Flush()
}

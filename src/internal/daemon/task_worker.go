package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/git"
	"github.com/leberkas-org/maggus/internal/ipc"
	"github.com/leberkas-org/maggus/internal/prompt"
	"github.com/leberkas-org/maggus/internal/runlog"
)

type TaskWorker struct {
	item   *WorkItem
	task   TaskSpec
	cfg    config.Config
	state  *State
	gitOps git.Operations
	agent  agent.Agent
	pool   *WorkerPool
	logger *runlog.Logger
	cancel context.CancelFunc
	ctx    context.Context
}

func NewTaskWorker(
	item *WorkItem,
	task TaskSpec,
	cfg config.Config,
	state *State,
	gitOps git.Operations,
	ag agent.Agent,
	pool *WorkerPool,
	logger *runlog.Logger,
) *TaskWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskWorker{
		item:   item,
		task:   task,
		cfg:    cfg,
		state:  state,
		gitOps: gitOps,
		agent:  ag,
		pool:   pool,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (w *TaskWorker) Cancel() {
	w.cancel()
}

func (w *TaskWorker) Run() {
	w.state.SetWorker(w.task.ID, &WorkerState{
		TaskID:    w.task.ID,
		ItemID:    w.item.ID,
		TaskTitle: w.task.Title,
		RepoURL:   w.item.RepoURL,
		Status:    "branching",
	})
	defer w.state.RemoveWorker(w.task.ID)

	if err := w.execute(); err != nil {
		log.Printf("task %s failed: %v", w.task.ID, err)
		w.task.Status = ItemFailed
		w.logEvent("task_failed", err.Error())
	} else {
		w.task.Status = ItemDone
		w.logEvent("task_complete", "")
	}

	w.checkItemComplete()
	_ = w.state.Flush()
}

func (w *TaskWorker) execute() error {
	featureBranch := fmt.Sprintf("feature/%s", w.item.ID)
	taskBranch := fmt.Sprintf("%s/%s", featureBranch, w.task.ID)

	// Ensure feature branch exists
	if !w.gitOps.BranchExists(w.item.RepoPath, featureBranch) {
		defaultBranch, err := w.gitOps.DefaultBranch(w.item.RepoPath)
		if err != nil {
			return fmt.Errorf("get default branch: %w", err)
		}
		if err := w.gitOps.CreateBranch(w.item.RepoPath, featureBranch, defaultBranch); err != nil {
			return fmt.Errorf("create feature branch: %w", err)
		}
	}

	// Create task branch from feature branch
	if err := w.gitOps.CreateBranch(w.item.RepoPath, taskBranch, featureBranch); err != nil {
		return fmt.Errorf("create task branch: %w", err)
	}

	// Create worktree
	wtPath := filepath.Join(w.item.RepoPath, ".maggus", "worktrees", w.task.ID)
	if err := w.gitOps.CreateWorktree(w.item.RepoPath, wtPath, taskBranch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	defer w.cleanup(wtPath, taskBranch)

	// Build prompt
	w.state.UpdateWorkerStatus(w.task.ID, "building prompt")
	contextMD := w.item.Description
	p := prompt.Build(prompt.BuildOptions{
		TaskContent:    w.task.Content,
		FeatureContext: contextMD,
		RepoPath:       w.item.RepoPath,
	})

	// Run agent
	w.state.UpdateWorkerStatus(w.task.ID, "running agent")
	w.logEvent("task_start", w.task.Title)

	sink := &workerSink{worker: w}
	err := w.agent.Run(w.ctx, agent.RunOptions{
		Prompt:  p,
		Model:   w.cfg.Model,
		WorkDir: wtPath,
		Output:  sink,
	})
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	// Post-completion commit
	w.state.UpdateWorkerStatus(w.task.ID, "committing")
	if w.gitOps.HasChanges(wtPath) {
		msg := fmt.Sprintf("chore(%s): auto-commit after agent completion", w.task.ID)
		if commitMsg, err := w.gitOps.ReadCommitFile(wtPath); err == nil && commitMsg != "" {
			msg = commitMsg
		}
		if err := w.gitOps.StageAll(wtPath); err != nil {
			return fmt.Errorf("stage: %w", err)
		}
		if _, err := w.gitOps.Commit(wtPath, msg); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
	}

	// Merge back (serialized per feature branch)
	w.state.UpdateWorkerStatus(w.task.ID, "merging")
	mu := w.pool.MergeMutex(featureBranch)
	mu.Lock()
	err = w.gitOps.MergeTaskBranch(w.item.RepoPath, featureBranch, taskBranch)
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	return nil
}

func (w *TaskWorker) cleanup(wtPath, taskBranch string) {
	_ = w.gitOps.RemoveWorktree(w.item.RepoPath, wtPath)
	_ = os.RemoveAll(wtPath)
	_ = w.gitOps.DeleteBranch(w.item.RepoPath, taskBranch)
}

func (w *TaskWorker) checkItemComplete() {
	allDone := true
	for _, t := range w.item.Tasks {
		if t.Status != ItemDone && t.Status != ItemFailed && t.Status != ItemSkipped {
			allDone = false
			break
		}
	}
	if allDone {
		w.item.Status = ItemDone
	}
}

func (w *TaskWorker) logEvent(event, text string) {
	if w.logger == nil {
		return
	}
	_ = w.logger.Log(runlog.Entry{
		Level:  "info",
		Event:  event,
		ItemID: w.item.ID,
		TaskID: w.task.ID,
		Title:  w.task.Title,
		Text:   text,
	})
}

// workerSink adapts OutputSink to update daemon state
type workerSink struct {
	worker *TaskWorker
}

func (s *workerSink) OnStatus(status string) {
	s.worker.state.UpdateWorkerStatus(s.worker.task.ID, status)
}

func (s *workerSink) OnOutput(text string) {
	s.worker.state.AppendWorkerOutput(s.worker.task.ID, text)
}

func (s *workerSink) OnTool(tool agent.ToolEvent) {
	s.worker.logEvent("tool_use", tool.Name)
}

func (s *workerSink) OnUsage(usage agent.UsageEvent) {
	s.worker.state.mu.Lock()
	if ws, ok := s.worker.state.workers[s.worker.task.ID]; ok {
		ws.Usage = ipc.TokenUsage{
			InputTokens:       usage.InputTokens,
			OutputTokens:      usage.OutputTokens,
			CacheReadTokens:   usage.CacheReadTokens,
			CacheCreateTokens: usage.CacheCreateTokens,
			CostUSD:           usage.CostUSD,
		}
	}
	s.worker.state.mu.Unlock()
}

func (s *workerSink) OnComplete(success bool) {
	if success {
		s.worker.state.UpdateWorkerStatus(s.worker.task.ID, "completed")
	} else {
		s.worker.state.UpdateWorkerStatus(s.worker.task.ID, "failed")
	}
}

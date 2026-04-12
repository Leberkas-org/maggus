package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitcommit"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/globalconfig"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/prompt"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// WorkerConfig holds everything needed to execute a single task's complete
// lifecycle: create branch → build prompt → run agent → commit → merge back
// → cleanup.
type WorkerConfig struct {
	// Task execution context.
	Ctx  context.Context
	Task parser.Task

	// Plan metadata (for criteria marking and snapshot writing).
	PlanFile  string // plan file path (for criteria marking on merge success)
	MaggusID  string // stable UUID from <!-- maggus-id: ... -->
	PlanTitle string // parsed H1 title from the plan file

	// Agent configuration.
	Agent          agent.Agent
	Model          string // resolved model name (run through config.ResolveModel)
	SessionPersist bool
	ValidIncludes  []string
	Iteration      int // prompt iteration counter

	// Repository configuration.
	RepoDir string // main repository root (always the real repo, not a worktree)

	// Branch and merge configuration.
	// When PlanBranch is non-empty, the worker creates a task branch from it,
	// merges back after commit, and cleans up the task branch.
	// When empty, the worker runs in the current directory without branching.
	PlanBranch string

	// UseWorktree creates a git worktree for task isolation. Only meaningful
	// when PlanBranch is set. The worktree is created at
	// .maggus/worktrees/<taskID> inside RepoDir.
	UseWorktree bool

	// ExistingWorktreePath is the path to a pre-existing worktree directory.
	// When non-empty, the worker skips branch and worktree creation entirely
	// and runs the agent in this directory instead of RepoDir. After the task
	// completes, the worker still merges the task branch back into PlanBranch
	// (in RepoDir), removes the worktree, and deletes the task branch.
	//
	// Used by dispatched workers where the worktree is created by dispatchTask()
	// before the worker process starts. UseWorktree is ignored when this is set.
	ExistingWorktreePath string

	// MergeMu serializes merge and criteria-marking operations when multiple
	// workers run concurrently. Leave nil for sequential (single-worker) mode.
	MergeMu *sync.Mutex

	// Output channels.
	Logger      *runlog.Logger
	AgentSender agent.MessageSender // receives agent output (status, tool, usage, output msgs)
	EventSender agent.MessageSender // receives lifecycle events (InfoMsg, CommitMsg)
	Notifier    *notify.Notifier

	// PreCommit is called after the agent completes successfully but before the
	// commit. Callers use this to perform pre-commit operations such as marking
	// completed feature files (renaming/deleting them) and firing lifecycle
	// hooks. The workDir argument is the directory where the agent ran (may
	// differ from RepoDir when UseWorktree is true). Leave nil for no
	// pre-commit operations.
	PreCommit func(workDir string)
}

// WorkerResult holds the outcome of a task worker execution.
type WorkerResult struct {
	// Completed is true if the task committed successfully (and merged, when
	// PlanBranch is set).
	Completed bool

	// CommitHash is the short git hash of the commit. Empty when no commit.
	CommitHash string

	// CommitMsg is the commit message text. Empty when no commit.
	CommitMsg string

	// Warning is a non-fatal message (e.g. "commit skipped").
	Warning string

	// Failed is non-nil when the task failed (agent error, commit error, etc.).
	Failed *failedTask

	// StopReason is set only for interruptions (context cancelled).
	StopReason StopReason

	// Blocked is true when the task was blocked by a merge conflict.
	// The conflict details are injected into the plan file automatically.
	Blocked bool
}

// RunTaskWorker executes the complete lifecycle for a single task:
//
//  1. Create task branch from PlanBranch (when set)
//  2. Build prompt with bootstrap context + task details
//  3. Run agent subprocess
//  4. Commit changes via COMMIT.md
//  5. Merge task branch back into PlanBranch (when set)
//  6. Delete task branch and worktree (best-effort cleanup)
//
// The function is designed to be called from sequential, parallel, and
// dispatched execution modes. Callers control concurrency and TUI
// coordination; the worker owns only the task lifecycle.
func RunTaskWorker(cfg WorkerConfig) WorkerResult {
	var result WorkerResult
	taskBranch := gitbranch.BranchName(cfg.Task.ID)

	workDir := cfg.RepoDir
	var worktreePath string

	// --- Step 1: Branch and worktree setup ---
	if cfg.ExistingWorktreePath != "" {
		// Dispatch mode: branch and worktree already created by the dispatcher
		// (dispatchTask). Run the agent in the pre-existing worktree; merge and
		// cleanup still happen in RepoDir after the task completes.
		worktreePath = cfg.ExistingWorktreePath
		workDir = cfg.ExistingWorktreePath
	} else if cfg.PlanBranch != "" {
		if cfg.UseWorktree {
			worktreePath = filepath.Join(cfg.RepoDir, ".maggus", "worktrees", cfg.Task.ID)
			if err := gitbranch.CreateBranchFrom(cfg.RepoDir, taskBranch, cfg.PlanBranch); err != nil {
				return workerFail(&result, cfg, fmt.Sprintf("create branch: %v", err))
			}
			if err := gitworktree.CreateWorktree(cfg.RepoDir, worktreePath, taskBranch); err != nil {
				return workerFail(&result, cfg, fmt.Sprintf("create worktree: %v", err))
			}
			workDir = worktreePath
		} else {
			if _, _, err := gitbranch.EnsureTaskBranchFromBase(cfg.RepoDir, cfg.Task.ID, cfg.PlanBranch); err != nil {
				return workerFail(&result, cfg, fmt.Sprintf("create task branch: %v", err))
			}
		}
	}

	cfg.Logger.TaskStart(cfg.Task.ID, cfg.Task.Title)

	// --- Step 2: Build prompt and run agent ---
	opts := prompt.Options{Include: cfg.ValidIncludes, Iteration: cfg.Iteration}
	builtPrompt := prompt.Build(&cfg.Task, opts)
	model := resolveTaskModel(cfg.Task.Model, cfg.Model)

	if err := cfg.Agent.Run(cfg.Ctx, builtPrompt, model, cfg.SessionPersist, cfg.AgentSender); err != nil {
		if cfg.Ctx.Err() != nil {
			result.StopReason = StopReasonInterrupted
			workerCleanupWorktree(cfg, worktreePath)
			return result
		}
		cfg.Notifier.PlayError()
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{AgentErrors: 1})
		reason := err.Error()
		cfg.Logger.TaskFailed(cfg.Task.ID, reason)
		sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("✗ %s failed: %s", cfg.Task.ID, reason)})
		result.Failed = &failedTask{ID: cfg.Task.ID, Title: cfg.Task.Title, Reason: reason}
		workerCleanupWorktree(cfg, worktreePath)
		return result
	}

	// --- Pre-commit operations (optional callback) ---
	if cfg.PreCommit != nil {
		cfg.PreCommit(workDir)
	}

	// --- Step 3: Commit ---
	commitResult, commitErr := gitcommit.CommitIteration(workDir, cfg.Task.ID+": "+cfg.Task.Title)
	if commitErr != nil {
		reason := commitErr.Error()
		cfg.Logger.TaskFailed(cfg.Task.ID, reason)
		sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("✗ %s commit failed: %s", cfg.Task.ID, reason)})
		result.Failed = &failedTask{ID: cfg.Task.ID, Title: cfg.Task.Title, Reason: reason}
		workerCleanupWorktree(cfg, worktreePath)
		return result
	}

	if !commitResult.Committed {
		msg := commitResult.Message
		if msg == "" {
			msg = "commit skipped (unknown reason)"
		}
		result.Warning = fmt.Sprintf("%s: %s", cfg.Task.ID, msg)
		sendEvent(cfg.EventSender, InfoMsg{Text: "⚠ " + result.Warning})
	} else {
		result.CommitHash = captureShortHash(workDir)
		result.CommitMsg = commitResult.Message
		cfg.Logger.TaskComplete(cfg.Task.ID, result.CommitHash)
		sendEvent(cfg.EventSender, CommitMsg{Message: commitResult.Message})
		cfg.Notifier.PlayTaskComplete()
		_ = globalconfig.IncrementMetrics(globalconfig.Metrics{GitCommits: 1})
	}

	// --- Step 4: Merge task branch back into plan branch ---
	if cfg.PlanBranch != "" {
		mergeErr := workerMerge(cfg, taskBranch)
		if mergeErr != nil {
			return workerHandleMergeErr(&result, cfg, mergeErr)
		}

		// Best-effort cleanup: remove worktree first (must precede branch deletion).
		if worktreePath != "" {
			if err := gitworktree.RemoveWorktree(cfg.RepoDir, worktreePath); err != nil {
				sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("⚠ %s: worktree cleanup failed: %v", cfg.Task.ID, err)})
			}
		}
		if err := gitbranch.DeleteBranch(cfg.RepoDir, taskBranch); err != nil {
			sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("⚠ %s: branch cleanup failed: %v", cfg.Task.ID, err)})
		}

		// Mark all acceptance criteria complete after successful merge.
		workerMarkCriteria(cfg)
	}

	if commitResult.Committed {
		result.Completed = true
	}
	return result
}

// workerMerge merges the task branch into the plan branch, using MergeMu
// for serialization when provided.
func workerMerge(cfg WorkerConfig, taskBranch string) error {
	if cfg.MergeMu != nil {
		cfg.MergeMu.Lock()
		defer cfg.MergeMu.Unlock()
	}
	return gitmerge.MergeTaskBranch(cfg.RepoDir, cfg.PlanBranch, taskBranch)
}

// workerMarkCriteria checks off all unchecked, non-blocked acceptance criteria
// for the task in its plan file. Uses MergeMu for serialization when provided.
func workerMarkCriteria(cfg WorkerConfig) {
	if cfg.PlanFile == "" {
		return
	}
	if cfg.MergeMu != nil {
		cfg.MergeMu.Lock()
		defer cfg.MergeMu.Unlock()
	}
	tasks, err := parser.ParseFile(cfg.PlanFile)
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.ID != cfg.Task.ID {
			continue
		}
		for _, c := range t.Criteria {
			if !c.Checked && !c.Blocked {
				_ = checkCriterionInFile(cfg.PlanFile, c.Text)
			}
		}
		return
	}
}

// workerFail records a task failure and returns the result.
func workerFail(result *WorkerResult, cfg WorkerConfig, reason string) WorkerResult {
	cfg.Logger.TaskFailed(cfg.Task.ID, reason)
	sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("✗ %s: %s", cfg.Task.ID, reason)})
	result.Failed = &failedTask{ID: cfg.Task.ID, Title: cfg.Task.Title, Reason: reason}
	return *result
}

// workerHandleMergeErr handles merge errors — conflicts mark the task as
// blocked, other errors mark it as failed.
func workerHandleMergeErr(result *WorkerResult, cfg WorkerConfig, err error) WorkerResult {
	if _, ok := err.(*gitmerge.MergeConflictError); ok {
		sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("⚠ %s: merge conflict — task blocked", cfg.Task.ID)})
		result.Warning = fmt.Sprintf("%s: merge conflict", cfg.Task.ID)
		result.Blocked = true
	} else {
		reason := fmt.Sprintf("merge: %v", err)
		cfg.Logger.TaskFailed(cfg.Task.ID, reason)
		sendEvent(cfg.EventSender, InfoMsg{Text: fmt.Sprintf("✗ %s: %s", cfg.Task.ID, reason)})
		result.Failed = &failedTask{ID: cfg.Task.ID, Title: cfg.Task.Title, Reason: reason}
	}
	return *result
}

// workerCleanupWorktree removes a worktree on failure (best-effort).
func workerCleanupWorktree(cfg WorkerConfig, worktreePath string) {
	if worktreePath != "" {
		_ = gitworktree.RemoveWorktree(cfg.RepoDir, worktreePath)
	}
}

// sendEvent sends a tea.Msg through an agent.MessageSender (nil-safe).
func sendEvent(sender agent.MessageSender, msg tea.Msg) {
	if sender != nil {
		sender.Send(msg)
	}
}

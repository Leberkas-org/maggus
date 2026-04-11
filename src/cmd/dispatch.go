package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leberkas-org/maggus/internal/gitbranch"
	"github.com/leberkas-org/maggus/internal/gitutil"
	"github.com/leberkas-org/maggus/internal/gitworktree"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// dispatchWork runs `maggus run --task <id>` by invoking the run subcommand.
func dispatchWork(taskID string) error {
	sub, remaining, err := rootCmd.Find([]string{"run", "--task", taskID})
	if err != nil {
		return err
	}
	// Reset work command flags so previous invocations don't leak.
	resetRunFlags()
	if err := sub.ParseFlags(remaining); err != nil {
		return err
	}
	return sub.RunE(sub, sub.Flags().Args())
}

// dispatchTask spawns a single task as an isolated background worker in its
// own worktree and branch. It creates the worktree, registers the worker in
// the shared workers index, launches a detached `maggus run` process, and
// returns immediately without waiting for the task to complete.
func dispatchTask(dir, taskID, model, agentName string) error {
	worktreePath := filepath.Join(dir, ".maggus", "worktrees", taskID)
	taskBranch := gitbranch.BranchName(taskID)

	// If the worktree already exists (e.g. from an interrupted dispatch),
	// clean it up before creating a fresh one.
	if err := cleanupExistingWorktree(dir, worktreePath); err != nil {
		return fmt.Errorf("cleanup existing worktree: %w", err)
	}

	// Determine the base branch to create the task branch from.
	baseBranch, err := dispatchBaseBranch(dir, taskID)
	if err != nil {
		return fmt.Errorf("determine base branch: %w", err)
	}

	// Create task branch from base and set up the worktree.
	if err := gitbranch.CreateBranchFrom(dir, taskBranch, baseBranch); err != nil {
		return fmt.Errorf("create task branch: %w", err)
	}
	if err := gitworktree.CreateWorktree(dir, worktreePath, taskBranch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	// Register the worker in the shared workers index so the TUI picks it up.
	entry := runlog.WorkerIndexEntry{
		TaskID:    taskID,
		TaskTitle: taskID, // Title will be updated by the dispatched process.
		Status:    "working",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := runlog.UpsertWorkerEntry(dir, entry); err != nil {
		return fmt.Errorf("register worker: %w", err)
	}

	// Write initial per-worker snapshot so the status view has something to show.
	initSnap := runlog.StateSnapshot{
		RunID:        generateDaemonRunID(),
		TaskID:       taskID,
		TaskTitle:    taskID,
		Status:       "Working",
		RunStartedAt: time.Now().UTC().Format(time.RFC3339),
		TaskStartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := runlog.WriteWorkerSnapshot(dir, taskID, initSnap); err != nil {
		return fmt.Errorf("write initial snapshot: %w", err)
	}

	// Launch the worker as a detached background process.
	if err := launchDispatchedWorker(dir, worktreePath, taskID, model, agentName); err != nil {
		// Clean up on launch failure: remove worker from index and worktree.
		_ = runlog.UpdateWorkerStatus(dir, taskID, "failed")
		_ = gitworktree.RemoveWorktree(dir, worktreePath)
		return fmt.Errorf("launch worker: %w", err)
	}

	return nil
}

// dispatchBaseBranch determines what branch to base the task branch on.
// In parallel mode the plan branch (feature/maggus-NNN-plan) is preferred;
// otherwise the current branch is used.
func dispatchBaseBranch(dir, taskID string) (string, error) {
	// Check if the parallel-mode plan branch exists for this task's plan.
	planBranch := gitbranch.PlanBranchNameFromTaskID(taskID)
	wts, err := gitworktree.ListWorktrees(dir)
	if err == nil {
		for _, wt := range wts {
			if wt.Branch == planBranch || wt.Branch == planBranch+"-plan" {
				return wt.Branch, nil
			}
		}
	}

	// Fall back to current branch.
	return currentBranchForDispatch(dir)
}

// currentBranchForDispatch returns the current HEAD branch name.
var currentBranchForDispatch = func(dir string) (string, error) {
	return currentBranchName(dir)
}

// currentBranchName reads the current branch using git.
func currentBranchName(dir string) (string, error) {
	cmd := gitutil.Command("rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// cleanupExistingWorktree removes a worktree directory if it already exists.
func cleanupExistingWorktree(repoDir, worktreePath string) error {
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil
	}
	// Try git worktree remove first (cleans up .git/worktrees/ metadata).
	if err := gitworktree.RemoveWorktree(repoDir, worktreePath); err != nil {
		// If git worktree remove fails (e.g. stale metadata), force-remove dir.
		return os.RemoveAll(worktreePath)
	}
	return nil
}

// launchDispatchedWorker starts a detached `maggus run` process in the worktree.
func launchDispatchedWorker(repoDir, worktreeDir, taskID, model, agentName string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	runID := generateDaemonRunID()
	args := []string{"run", "--task", taskID, "--daemon-run", "--daemon-run-id=" + runID, "--dispatch-repo=" + repoDir}
	if model != "" {
		args = append(args, "--model="+model)
	}
	if agentName != "" {
		args = append(args, "--agent="+agentName)
	}

	// Open log file for the dispatched worker.
	logDir := filepath.Join(repoDir, ".maggus", "runs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("dispatch-%s.log", taskID))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	pid, err := launchDaemon(exe, args, logFile, worktreeDir)
	if err != nil {
		return err
	}

	// We don't track the PID beyond this — the worker is fire-and-forget.
	// The status TUI tracks it via the workers index and per-worker snapshot.
	_ = pid
	return nil
}

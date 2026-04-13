package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/gitmerge"
	"github.com/leberkas-org/maggus/internal/notify"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/runlog"
)

// --- msgCollector collects tea.Msg sent via agent.MessageSender ---

type msgCollector struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (c *msgCollector) Send(msg tea.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
}

func (c *msgCollector) infoTexts() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var texts []string
	for _, m := range c.msgs {
		if im, ok := m.(InfoMsg); ok {
			texts = append(texts, im.Text)
		}
	}
	return texts
}

func (c *msgCollector) commitMsgs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var texts []string
	for _, m := range c.msgs {
		if cm, ok := m.(CommitMsg); ok {
			texts = append(texts, cm.Message)
		}
	}
	return texts
}

// --- configurable fakeAgent for worker tests ---

type workerFakeAgent struct {
	runErr error
}

func (f *workerFakeAgent) Run(_ context.Context, _ string, _ string, _ bool, _ agent.MessageSender) error {
	return f.runErr
}
func (f *workerFakeAgent) RunOnce(_ context.Context, _ string, _ string, _ bool) (string, error) {
	return "", nil
}
func (f *workerFakeAgent) Name() string    { return "fake" }
func (f *workerFakeAgent) Validate() error { return nil }

// newTestLogger creates a runlog.Logger in a temp dir for testing.
func newTestLogger(t *testing.T) *runlog.Logger {
	t.Helper()
	dir := t.TempDir()
	logger, err := runlog.Open(dir, 0)
	if err != nil {
		t.Fatalf("runlog.Open: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

// newTestNotifier creates a silent Notifier for tests.
func newTestNotifier() *notify.Notifier {
	return notify.New(config.NotificationsConfig{})
}

// --- workerMarkCriteria tests ---

func TestWorkerMarkCriteria(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature.md")
	content := "### TASK-001: First\n- [ ] Criterion A\n- [ ] Criterion B\n- [ ] BLOCKED: Something\n\n### TASK-002: Second\n- [ ] Other criterion\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := WorkerConfig{
		Task:     parser.Task{ID: "TASK-001"},
		PlanFile: path,
	}
	workerMarkCriteria(cfg)

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "- [x] Criterion A") {
		t.Error("Criterion A should be checked")
	}
	if !strings.Contains(got, "- [x] Criterion B") {
		t.Error("Criterion B should be checked")
	}
	if !strings.Contains(got, "- [ ] BLOCKED: Something") {
		t.Error("BLOCKED criterion should remain unchecked")
	}
	if !strings.Contains(got, "- [ ] Other criterion") {
		t.Error("TASK-002 criterion should remain unchecked")
	}
}

func TestWorkerMarkCriteria_EmptyPlanFile(t *testing.T) {
	// Should not panic when PlanFile is empty.
	cfg := WorkerConfig{
		Task: parser.Task{ID: "TASK-001"},
	}
	workerMarkCriteria(cfg) // no-op, should not panic
}

func TestWorkerMarkCriteria_WithMutex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feature.md")
	content := "### TASK-001: First\n- [ ] Criterion A\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	mu := &sync.Mutex{}
	cfg := WorkerConfig{
		Task:     parser.Task{ID: "TASK-001"},
		PlanFile: path,
		MergeMu:  mu,
	}
	workerMarkCriteria(cfg)

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "- [x] Criterion A") {
		t.Error("Criterion A should be checked")
	}
}

// --- workerHandleMergeErr tests ---

func TestWorkerHandleMergeErr_Conflict(t *testing.T) {
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Task:        parser.Task{ID: "TASK-001", Title: "Test task"},
		Logger:      logger,
		EventSender: events,
	}

	var result WorkerResult
	got := workerHandleMergeErr(&result, cfg, &gitmerge.MergeConflictError{
		FeatureBranch: "feature/feat-001-plan",
		TaskBranch:    "feature/maggus-001/task-001",
	})

	if !got.Blocked {
		t.Error("expected Blocked to be true for merge conflict")
	}
	if got.Failed != nil {
		t.Error("expected Failed to be nil for merge conflict (it's a warning, not a failure)")
	}
	if !strings.Contains(got.Warning, "merge conflict") {
		t.Errorf("Warning = %q, want it to mention 'merge conflict'", got.Warning)
	}

	infos := events.infoTexts()
	if len(infos) == 0 {
		t.Fatal("expected at least one info message")
	}
	if !strings.Contains(infos[0], "merge conflict") {
		t.Errorf("info text = %q, want it to mention 'merge conflict'", infos[0])
	}
}

func TestWorkerHandleMergeErr_OtherError(t *testing.T) {
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Task:        parser.Task{ID: "TASK-002", Title: "Other task"},
		Logger:      logger,
		EventSender: events,
	}

	var result WorkerResult
	got := workerHandleMergeErr(&result, cfg, errors.New("network timeout"))

	if got.Blocked {
		t.Error("expected Blocked to be false for non-conflict error")
	}
	if got.Failed == nil {
		t.Fatal("expected Failed to be non-nil for non-conflict merge error")
	}
	if got.Failed.ID != "TASK-002" {
		t.Errorf("Failed.ID = %q, want %q", got.Failed.ID, "TASK-002")
	}
	if !strings.Contains(got.Failed.Reason, "network timeout") {
		t.Errorf("Failed.Reason = %q, want it to contain 'network timeout'", got.Failed.Reason)
	}
}

// --- workerFail tests ---

func TestWorkerFail(t *testing.T) {
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Task:        parser.Task{ID: "TASK-003", Title: "Failing task"},
		Logger:      logger,
		EventSender: events,
	}

	var result WorkerResult
	got := workerFail(&result, cfg, "branch creation failed")

	if got.Failed == nil {
		t.Fatal("expected Failed to be non-nil")
	}
	if got.Failed.ID != "TASK-003" {
		t.Errorf("Failed.ID = %q, want %q", got.Failed.ID, "TASK-003")
	}
	if got.Failed.Reason != "branch creation failed" {
		t.Errorf("Failed.Reason = %q, want %q", got.Failed.Reason, "branch creation failed")
	}

	infos := events.infoTexts()
	if len(infos) != 1 {
		t.Fatalf("expected 1 info message, got %d", len(infos))
	}
	if !strings.Contains(infos[0], "TASK-003") || !strings.Contains(infos[0], "branch creation failed") {
		t.Errorf("info = %q, want it to contain task ID and reason", infos[0])
	}
}

// --- sendEvent tests ---

func TestSendEvent_NilSender(t *testing.T) {
	// Should not panic with nil sender.
	sendEvent(nil, InfoMsg{Text: "test"})
}

func TestSendEvent_SendsMessage(t *testing.T) {
	c := &msgCollector{}
	sendEvent(c, InfoMsg{Text: "hello"})

	if len(c.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(c.msgs))
	}
	if im, ok := c.msgs[0].(InfoMsg); !ok || im.Text != "hello" {
		t.Errorf("expected InfoMsg{Text: 'hello'}, got %v", c.msgs[0])
	}
}

// --- RunTaskWorker tests ---

func TestRunTaskWorker_AgentError(t *testing.T) {
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Ctx:         context.Background(),
		Task:        parser.Task{ID: "TASK-010", Title: "Agent fail test"},
		Agent:       &workerFakeAgent{runErr: errors.New("agent crashed")},
		RepoDir:     t.TempDir(),
		Logger:      logger,
		AgentSender: events,
		EventSender: events,
		Notifier:    newTestNotifier(),
	}

	result := RunTaskWorker(cfg)

	if result.Completed {
		t.Error("expected Completed to be false")
	}
	if result.Failed == nil {
		t.Fatal("expected Failed to be non-nil")
	}
	if result.Failed.Reason != "agent crashed" {
		t.Errorf("Failed.Reason = %q, want %q", result.Failed.Reason, "agent crashed")
	}
}

func TestRunTaskWorker_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Ctx:         ctx,
		Task:        parser.Task{ID: "TASK-011", Title: "Cancelled test"},
		Agent:       &workerFakeAgent{runErr: context.Canceled},
		RepoDir:     t.TempDir(),
		Logger:      logger,
		AgentSender: events,
		EventSender: events,
		Notifier:    newTestNotifier(),
	}

	result := RunTaskWorker(cfg)

	if result.StopReason != StopReasonInterrupted {
		t.Errorf("StopReason = %v, want StopReasonInterrupted", result.StopReason)
	}
	if result.Failed != nil {
		t.Error("Failed should be nil for interruption (not a task failure)")
	}
}

func TestRunTaskWorker_NoBranch_AgentSucceeds_NoCommit(t *testing.T) {
	// Test the simplest path: no plan branch, agent succeeds, no COMMIT.md.
	dir := t.TempDir()
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Ctx:         context.Background(),
		Task:        parser.Task{ID: "TASK-012", Title: "No commit test"},
		Agent:       &workerFakeAgent{},
		RepoDir:     dir,
		Logger:      logger,
		AgentSender: events,
		EventSender: events,
		Notifier:    newTestNotifier(),
	}

	result := RunTaskWorker(cfg)

	// CommitIteration will fail because there's no git repo in TempDir.
	// It should produce a failure (commit error), not a panic.
	if result.Failed == nil {
		// If the commit doesn't error out (e.g., COMMIT.md not found returns
		// a non-error Result), the worker should report a warning instead.
		if result.Warning == "" && result.Completed {
			t.Error("expected either a failure or a warning for missing COMMIT.md")
		}
	}
}

// --- WorkerResult struct tests ---

func TestWorkerResult_ZeroValue(t *testing.T) {
	var r WorkerResult
	if r.Completed {
		t.Error("zero-value Completed should be false")
	}
	if r.Blocked {
		t.Error("zero-value Blocked should be false")
	}
	if r.Failed != nil {
		t.Error("zero-value Failed should be nil")
	}
	if r.StopReason != 0 {
		t.Error("zero-value StopReason should be 0")
	}
}

// --- WorkerConfig usability tests ---

func TestWorkerConfig_UsableFromSequentialCallSite(t *testing.T) {
	// Verify the WorkerConfig struct can be populated for sequential mode.
	// WorkDir == RepoDir (no worktree), no MergeMu needed.
	cfg := WorkerConfig{
		Ctx:            context.Background(),
		Task:           parser.Task{ID: "TASK-020", Title: "Sequential test"},
		PlanFile:       "/path/to/feature_020.md",
		MaggusID:       "uuid-020",
		Agent:          &workerFakeAgent{},
		Model:          "claude-sonnet-4-6",
		SessionPersist: false,
		RepoDir:        "/repo",
		WorkDir:        "/repo", // same as RepoDir in sequential mode
		PlanBranch:     "",      // no branching in simple sequential mode
		MergeMu:        nil,
		AgentSender:    &msgCollector{},
		EventSender:    &msgCollector{},
	}
	if cfg.WorkDir != cfg.RepoDir {
		t.Error("WorkDir should equal RepoDir for sequential mode")
	}
	if cfg.MergeMu != nil {
		t.Error("MergeMu should be nil for sequential mode")
	}
}

func TestWorkerConfig_UsableFromParallelCallSite(t *testing.T) {
	// Verify the WorkerConfig struct can be populated for parallel mode.
	// The caller (orchestrator) creates the worktree and passes WorkDir pointing to it.
	mu := &sync.Mutex{}
	cfg := WorkerConfig{
		Ctx:            context.Background(),
		Task:           parser.Task{ID: "TASK-021", Title: "Parallel test"},
		PlanFile:       "/path/to/feature_021.md",
		MaggusID:       "uuid-021",
		Agent:          &workerFakeAgent{},
		Model:          "claude-opus-4-6",
		SessionPersist: true,
		RepoDir:        "/repo",
		WorkDir:        "/repo/.maggus/worktrees/TASK-021", // worktree created by orchestrator
		PlanBranch:     "feature/feat-021-plan",
		MergeMu:        mu,
		AgentSender:    &msgCollector{},
		EventSender:    &msgCollector{},
	}
	if cfg.WorkDir == cfg.RepoDir {
		t.Error("WorkDir should differ from RepoDir in parallel worktree mode")
	}
	if cfg.MergeMu == nil {
		t.Error("MergeMu should be non-nil for parallel mode")
	}
	if cfg.PlanBranch == "" {
		t.Error("PlanBranch should be set for parallel mode")
	}
}

func TestWorkerConfig_UsableFromDispatchCallSite(t *testing.T) {
	// Verify the WorkerConfig struct can be populated for dispatch mode.
	// RepoDir = main repo for git operations; WorkDir = pre-existing worktree path.
	cfg := WorkerConfig{
		Ctx:        context.Background(),
		Task:       parser.Task{ID: "TASK-022", Title: "Dispatch test"},
		PlanFile:   "/repo/.maggus/features/feature_022.md",
		MaggusID:   "uuid-022",
		Agent:      &workerFakeAgent{},
		Model:      "claude-sonnet-4-6",
		RepoDir:    "/repo",                                    // main repo for git operations
		WorkDir:    "/repo/.maggus/worktrees/TASK-022-001",     // pre-existing worktree
		PlanBranch: "feature/feat-022-plan",                    // base branch for merge-back
		MergeMu:    nil,                                        // single worker, no serialization needed
		AgentSender: &msgCollector{},
		EventSender: &msgCollector{},
	}
	if cfg.PlanBranch == "" {
		t.Error("PlanBranch should be set for dispatch mode (for merge-back)")
	}
	if cfg.WorkDir == "" {
		t.Error("WorkDir should be set for dispatch mode")
	}
	if cfg.RepoDir == cfg.WorkDir {
		t.Error("RepoDir (main repo) and WorkDir (worktree) should differ")
	}
}

// --- workerMerge tests ---

func TestWorkerMerge_WithMutex_LocksAndUnlocks(t *testing.T) {
	// We can't test actual git merge without a repo, but we can verify
	// the mutex is properly locked/unlocked by checking for deadlock.
	mu := &sync.Mutex{}

	cfg := WorkerConfig{
		RepoDir:    t.TempDir(),
		PlanBranch: "test-branch",
		MergeMu:    mu,
	}

	// workerMerge will fail (no git repo) but the mutex should be released.
	_ = workerMerge(cfg, "nonexistent-branch")

	// If the mutex wasn't unlocked, this would deadlock.
	mu.Lock()
	mu.Unlock()
}

// --- Integration: verify workerMarkCriteria handles all criterion states ---

func TestWorkerMarkCriteria_AllCriterionStates(t *testing.T) {
	dir := t.TempDir()

	content := "### TASK-001: First\n- [ ] A\n- [ ] B\n- [ ] BLOCKED: X\n- [x] C\n\n### TASK-002: Second\n- [ ] D\n"

	path := filepath.Join(dir, "feature.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := WorkerConfig{
		Task:     parser.Task{ID: "TASK-001"},
		PlanFile: path,
	}
	workerMarkCriteria(cfg)

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "- [x] A") {
		t.Error("A should be checked")
	}
	if !strings.Contains(got, "- [x] B") {
		t.Error("B should be checked")
	}
	if !strings.Contains(got, "- [ ] BLOCKED: X") {
		t.Error("BLOCKED criterion should remain unchecked")
	}
	if !strings.Contains(got, "- [x] C") {
		t.Error("already-checked C should remain checked")
	}
	if !strings.Contains(got, "- [ ] D") {
		t.Error("TASK-002 criterion should remain unchecked")
	}
}

// --- Test sendEvent with CommitMsg ---

func TestSendEvent_CommitMsg(t *testing.T) {
	c := &msgCollector{}
	sendEvent(c, CommitMsg{Message: "feat: add feature"})

	commits := c.commitMsgs()
	if len(commits) != 1 || commits[0] != "feat: add feature" {
		t.Errorf("expected CommitMsg 'feat: add feature', got %v", commits)
	}
}

// --- Verify worker produces correct result fields ---

func TestRunTaskWorker_ResultFieldsOnAgentError(t *testing.T) {
	events := &msgCollector{}
	logger := newTestLogger(t)

	cfg := WorkerConfig{
		Ctx:         context.Background(),
		Task:        parser.Task{ID: "TASK-030", Title: "Field check"},
		Agent:       &workerFakeAgent{runErr: fmt.Errorf("timeout")},
		RepoDir:     t.TempDir(),
		Logger:      logger,
		AgentSender: events,
		EventSender: events,
		Notifier:    newTestNotifier(),
	}

	result := RunTaskWorker(cfg)

	if result.CommitHash != "" {
		t.Errorf("CommitHash should be empty on agent error, got %q", result.CommitHash)
	}
	if result.CommitMsg != "" {
		t.Errorf("CommitMsg should be empty on agent error, got %q", result.CommitMsg)
	}
	if result.Completed {
		t.Error("Completed should be false on agent error")
	}
	if result.Blocked {
		t.Error("Blocked should be false on agent error")
	}
}

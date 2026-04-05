package cmd

import (
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

// ProgressMsg is sent when iteration progress changes.
type ProgressMsg struct {
	Current int
	Total   int
}

// CommitMsg is sent when a commit completes, to display in the recent commits section.
type CommitMsg struct {
	Message string
}

// InfoMsg displays an informational message in the TUI.
type InfoMsg struct {
	Text string
}

// TaskCriterion holds a single acceptance criterion for display in the task detail view.
type TaskCriterion struct {
	Text    string
	Checked bool
	Blocked bool
}

// RemainingTask is a task that was not completed during the run.
type RemainingTask struct {
	ID         string
	Title      string
	SourceFile string // filename (not full path) of the feature/bug file
}

// FailedTask records a task that could not be completed during the run.
type FailedTask struct {
	ID     string
	Title  string
	Reason string
}

// IterationStartMsg resets per-iteration state when a new iteration begins.
type IterationStartMsg struct {
	Current         int
	Total           int
	TaskID          string
	TaskTitle       string
	ItemID          string // stable UUID from <!-- maggus-id: ... -->
	ItemShort       string // e.g. "feature_001"
	ItemTitle       string // parsed H1 title from the feature/bug file
	Kind            string // "bug" or "feature"
	TaskDescription string
	TaskCriteria    []TaskCriterion
	RemainingTasks  []RemainingTask // upcoming workable tasks (excludes current)
	FeatureCurrent  int             // 1-based index of current feature (0 if not feature-centric)
	FeatureTotal    int             // total number of features being processed
	TaskModel       string          // per-task model override (empty = use default)
}

// BannerInfo holds startup information displayed in the TUI's initial view.
type BannerInfo struct {
	Iterations    int
	Branch        string
	RunID         string
	Worktree      string // empty if not using worktree
	Agent         string // agent name (e.g. "claude", "opencode")
	TwoXExpiresIn string // e.g. "17h 54m 44s"; empty when not in 2x mode
	CWD           string // current working directory, shown in header
}

// StopReason describes why the work loop ended.
type StopReason int

const (
	StopReasonComplete        StopReason = iota // all requested tasks finished
	StopReasonUserStop                          // user pressed 's' (stop after task)
	StopReasonInterrupted                       // user pressed Ctrl+C
	StopReasonError                             // a task or commit failed
	StopReasonNoTasks                           // no workable tasks found
	StopReasonPartialComplete                   // loop finished but some tasks failed
)

// SummaryData holds information displayed on the post-completion summary screen.
type SummaryData struct {
	RunID          string
	Branch         string
	Model          string
	StartTime      time.Time
	TasksCompleted int
	TasksTotal     int
	CommitStart    string // short hash of first commit
	CommitEnd      string // short hash of last commit
	RemainingTasks []RemainingTask
	Reason         StopReason // why the run ended
	ErrorDetail    string     // error message when Reason == StopReasonError
	Warnings       []string   // non-fatal warnings (e.g. skipped commits)
	FailedTasks    []FailedTask
	TasksFailed    int
}

// SummaryMsg tells the TUI to transition to the summary view.
type SummaryMsg struct {
	Data SummaryData
}

// PushStatusMsg updates the push status on the summary screen.
type PushStatusMsg struct {
	Status string // e.g. "Pushed to origin/branch" or "Push failed: reason"
	Done   bool
}

// QuitMsg tells the TUI to transition to the "done" state (waiting for keypress to exit).
type QuitMsg struct{}

// SyncCheckMsg tells the TUI to show the sync resolution screen between tasks.
// The work goroutine blocks on ResultCh until the user makes a choice.
type SyncCheckMsg struct {
	Behind       int
	Ahead        int
	RemoteBranch string
	ResultCh     chan<- SyncCheckResult
}

// SyncCheckResult is the user's resolution choice sent back to the work goroutine.
type SyncCheckResult struct {
	Action  SyncAction
	Message string // info message (e.g. "Pulled 3 commits")
	Err     error  // non-nil if the pull action failed fatally
}

// SyncAction represents the user's choice during a between-task sync check.
type SyncAction int

const (
	SyncProceed SyncAction = iota // continue (pull succeeded, skip, or up-to-date)
	SyncAbort                     // user chose to abort
)

// TaskUsage records token usage for a single task/iteration.
type TaskUsage struct {
	Kind                     string
	ItemID                   string
	ItemShort                string
	ItemTitle                string
	TaskShort                string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	CostUSD                  float64
	ModelUsage               map[string]agent.ModelTokens
	StartTime                time.Time
	EndTime                  time.Time
}

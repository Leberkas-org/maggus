package bryan

import "context"

type TaskStatus int

const (
	TaskStatusNew        TaskStatus = 4
	TaskStatusReady      TaskStatus = 5
	TaskStatusInProgress TaskStatus = 6
	TaskStatusReview     TaskStatus = 7
	TaskStatusDone       TaskStatus = 8
	TaskStatusFailed     TaskStatus = 3
)

type FeatureContext struct {
	FeatureID              string
	Title                  string
	Intro                  string
	Goals                  []string
	FunctionalRequirements []string
	NonGoals               []string
	TechnicalConsiderations []string
	SuccessMetrics         []string
	WorktreePath           string
	BranchPrefix           string
}

type TaskAssignment struct {
	TaskID            string
	RepoURL           string
	Description       string
	Context           string
	Priority          int
	Predecessors      []string
	Successors        []string
	FeatureID         string
	BranchName        string
	AcceptanceCriteria []string
}

type LogSender interface {
	Send(entry LogEntry) error
	Close() error
}

type LogEntry struct {
	Timestamp int64
	Level     string
	Event     string
	ItemID    string
	TaskID    string
	Title     string
	Tool      string
	Text      string
}

type UsageReport struct {
	AgentID    string
	PID        int
	Repository string
	Kind       string
	ItemID     string
	Model      string
	Agent      string
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
	CostUSD           float64
}

type Client interface {
	Connect(ctx context.Context, machineID string, repos []string) error
	UpdateTaskStatus(taskID string, status TaskStatus, msg string) error
	GetFeatureContext(featureID string) (*FeatureContext, error)
	RequestNextTask() error
	LogStream(ctx context.Context) (LogSender, error)
	ReportUsage(ctx context.Context, report UsageReport) error
	SyncMemory(repoURL string, content string) error
	Messages() <-chan TaskAssignment
	Close() error
}

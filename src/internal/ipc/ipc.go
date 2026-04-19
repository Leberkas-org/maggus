package ipc

import (
	"context"
	"time"
)

type DaemonSnapshot struct {
	BryanConnected bool             `json:"bryan_connected"`
	Repos          []RepoSnapshot   `json:"repos"`
	Queue          []QueueItem      `json:"queue"`
	Workers        []WorkerSnapshot `json:"workers"`
	ActiveTasks    int              `json:"active_tasks"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type RepoSnapshot struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

type QueueItem struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	RepoURL     string          `json:"repo_url"`
	RepoPath    string          `json:"repo_path"`
	Status      string          `json:"status"`
	Priority    int             `json:"priority"`
	Tasks       int             `json:"tasks"`
	Done        int             `json:"done"`
	PlanFile    string          `json:"plan_file,omitempty"`
	Description string          `json:"description,omitempty"`
	TaskList    []TaskSnapshot  `json:"task_list,omitempty"`
}

type TaskSnapshot struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type WorkerSnapshot struct {
	TaskID      string     `json:"task_id"`
	ItemID      string     `json:"item_id"`
	TaskTitle   string     `json:"task_title"`
	RepoURL     string     `json:"repo_url"`
	Status      string     `json:"status"`
	AgentOutput string     `json:"agent_output"`
	TokenUsage  TokenUsage `json:"token_usage"`
	StartedAt   time.Time  `json:"started_at"`
}

type TokenUsage struct {
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	CacheReadTokens   int     `json:"cache_read_tokens"`
	CacheCreateTokens int     `json:"cache_create_tokens"`
	CostUSD           float64 `json:"cost_usd"`
}

type StateWriter interface {
	WriteState(state DaemonSnapshot) error
}

type StateReader interface {
	ReadState() (DaemonSnapshot, error)
	Watch(ctx context.Context) <-chan DaemonSnapshot
}

type CommandWriter interface {
	StopAll() error
	StopRepo(repoURL string) error
	Approve(itemID string) error
	Skip(itemID string) error
	Reorder(priorities map[string]int) error
}

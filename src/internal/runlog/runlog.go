package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Timestamp   time.Time         `json:"ts"`
	Level       string            `json:"level"`
	Event       string            `json:"event"`
	ItemID      string            `json:"item_id"`
	TaskID      string            `json:"task_id"`
	Title       string            `json:"title,omitempty"`
	Tool        string            `json:"tool,omitempty"`
	Input       json.RawMessage   `json:"input,omitempty"`
	Text        string            `json:"text,omitempty"`
	Commit      string            `json:"commit,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	TokenUsage  *TokenUsageEntry  `json:"task_usage,omitempty"`
}

type TokenUsageEntry struct {
	InputTokens       int                    `json:"input_tokens"`
	OutputTokens      int                    `json:"output_tokens"`
	CacheReadTokens   int                    `json:"cache_read_tokens"`
	CacheCreateTokens int                    `json:"cache_create_tokens"`
	CostUSD           float64                `json:"cost_usd"`
	ModelUsage        map[string]any `json:"model_usage,omitempty"`
}

type Logger struct {
	file *os.File
	mu   sync.Mutex
}

func New(logsDir, itemID string) (*Logger, error) {
	dir := filepath.Join(logsDir, itemID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.log", os.Getpid()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{file: f}, nil
}

func (l *Logger) Log(entry Entry) error {
	entry.Timestamp = time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = l.file.Write(append(data, '\n'))
	return err
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

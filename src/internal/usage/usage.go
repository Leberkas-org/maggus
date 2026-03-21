// Package usage appends per-task token usage records to .maggus/usage_work.jsonl.
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
)

const fileName = ".maggus/usage_work.jsonl"

// Record represents a single task usage entry.
type Record struct {
	RunID                    string                       `json:"run_id"`
	TaskID                   string                       `json:"task_id"`
	TaskTitle                string                       `json:"task_title"`
	PlanFile                 string                       `json:"plan_file"`
	Model                    string                       `json:"model"`
	Agent                    string                       `json:"agent"`
	InputTokens              int                          `json:"input_tokens"`
	OutputTokens             int                          `json:"output_tokens"`
	CacheCreationInputTokens int                          `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                          `json:"cache_read_input_tokens"`
	CostUSD                  float64                      `json:"cost_usd"`
	ModelUsage               map[string]agent.ModelTokens `json:"model_usage"`
	StartTime                time.Time                    `json:"start_time"`
	EndTime                  time.Time                    `json:"end_time"`
	Elapsed                  string                       `json:"elapsed"`
}

// Append writes one or more usage records as JSON Lines to .maggus/usage_work.jsonl.
func Append(dir string, records []Record) error {
	return AppendTo(filepath.Join(dir, fileName), records)
}

// AppendTo writes one or more usage records as JSON Lines to the given file path.
func AppendTo(path string, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open usage file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for i := range records {
		records[i].Elapsed = records[i].EndTime.Sub(records[i].StartTime).Truncate(time.Second).String()
		if err := enc.Encode(records[i]); err != nil {
			return fmt.Errorf("write usage record: %w", err)
		}
	}

	return nil
}

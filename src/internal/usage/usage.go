// Package usage appends per-task token usage records to ~/.maggus/usage/.
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/globalconfig"
)

// Record represents a single task usage entry.
type Record struct {
	RunID                    string                       `json:"run_id"`
	Repository               string                       `json:"repository,omitempty"`
	Kind                     string                       `json:"kind,omitempty"`
	ItemID                   string                       `json:"item_id,omitempty"`
	ItemShort                string                       `json:"item_short,omitempty"`
	ItemTitle                string                       `json:"item_title,omitempty"`
	TaskShort                string                       `json:"task_short,omitempty"`
	TaskTitle                string                       `json:"task_title,omitempty"`
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
}

// Append writes one or more usage records to ~/.maggus/usage/.
// Work records (Kind empty) go to work.jsonl; session records go to sessions.jsonl.
// The ~/.maggus/usage/ directory is created automatically if missing.
func Append(records []Record) error {
	if len(records) == 0 {
		return nil
	}

	globalDir, err := globalconfig.Dir()
	if err != nil {
		return fmt.Errorf("get global config dir: %w", err)
	}

	usageDir := filepath.Join(globalDir, "usage")
	if err := os.MkdirAll(usageDir, 0o755); err != nil {
		return fmt.Errorf("create usage directory: %w", err)
	}

	var workRecords, sessionRecords []Record
	for _, r := range records {
		if r.Kind == "" {
			workRecords = append(workRecords, r)
		} else {
			sessionRecords = append(sessionRecords, r)
		}
	}

	if len(workRecords) > 0 {
		if err := AppendTo(filepath.Join(usageDir, "work.jsonl"), workRecords); err != nil {
			return err
		}
	}
	if len(sessionRecords) > 0 {
		if err := AppendTo(filepath.Join(usageDir, "sessions.jsonl"), sessionRecords); err != nil {
			return err
		}
	}

	return nil
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
		if err := enc.Encode(records[i]); err != nil {
			return fmt.Errorf("write usage record: %w", err)
		}
	}

	return nil
}

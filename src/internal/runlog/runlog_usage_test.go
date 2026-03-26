package runlog_test

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestTaskUsage(t *testing.T) {
	dir := t.TempDir()
	l, err := runlog.Open("run1", dir, 50)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	l.TaskUsage(runlog.TaskUsageData{
		InputTokens:              12000,
		OutputTokens:             800,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     200,
		CostUSD:                  0.042,
		ModelUsage: map[string]runlog.ModelTokensEntry{
			"claude-sonnet-4-6": {
				InputTokens:  12000,
				OutputTokens: 800,
				CostUSD:      0.042,
			},
		},
	})

	logPath := findLogFile(t, dir)
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("unmarshal: %v\nline: %s", err, scanner.Text())
		}
		lines = append(lines, m)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(lines))
	}
	raw := lines[0]

	if raw["event"] != "task_usage" {
		t.Errorf("event = %q, want task_usage", raw["event"])
	}
	if raw["input_tokens"] != float64(12000) {
		t.Errorf("input_tokens = %v, want 12000", raw["input_tokens"])
	}
	if raw["output_tokens"] != float64(800) {
		t.Errorf("output_tokens = %v, want 800", raw["output_tokens"])
	}
	if raw["cache_creation_input_tokens"] != float64(500) {
		t.Errorf("cache_creation_input_tokens = %v, want 500", raw["cache_creation_input_tokens"])
	}
	if raw["cache_read_input_tokens"] != float64(200) {
		t.Errorf("cache_read_input_tokens = %v, want 200", raw["cache_read_input_tokens"])
	}
	costUSD, ok := raw["cost_usd"].(float64)
	if !ok {
		t.Fatalf("cost_usd missing or not a float")
	}
	if costUSD < 0.041 || costUSD > 0.043 {
		t.Errorf("cost_usd = %v, want ~0.042", costUSD)
	}

	modelUsage, ok := raw["model_usage"].(map[string]any)
	if !ok {
		t.Fatalf("model_usage missing or wrong type")
	}
	modelEntry, ok := modelUsage["claude-sonnet-4-6"].(map[string]any)
	if !ok {
		t.Fatalf("model_usage[claude-sonnet-4-6] missing or wrong type")
	}
	if modelEntry["input_tokens"] != float64(12000) {
		t.Errorf("model input_tokens = %v, want 12000", modelEntry["input_tokens"])
	}
	if modelEntry["output_tokens"] != float64(800) {
		t.Errorf("model output_tokens = %v, want 800", modelEntry["output_tokens"])
	}
}

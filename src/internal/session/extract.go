package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/leberkas-org/maggus/internal/agent"
)

// sessionLine represents a single line in a Claude session JSONL file.
// We only decode the fields we need for usage extraction.
type sessionLine struct {
	Type    string         `json:"type"`
	Message sessionMessage `json:"message"`
}

type sessionMessage struct {
	Model string       `json:"model"`
	Usage sessionUsage `json:"usage"`
}

type sessionUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// UsageSummary holds aggregated token usage from a Claude session, grouped by model.
type UsageSummary struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	ModelUsage               map[string]agent.ModelTokens
}

// ExtractUsage reads a Claude session JSONL file, finds all assistant messages,
// and sums their token usage. Usage is grouped by model. Malformed lines are skipped.
func ExtractUsage(path string) (*UsageSummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	summary := &UsageSummary{
		ModelUsage: make(map[string]agent.ModelTokens),
	}

	scanner := bufio.NewScanner(f)
	// Increase buffer size for potentially large session lines.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var line sessionLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			// Malformed line — skip and continue.
			continue
		}

		if line.Type != "assistant" {
			continue
		}

		u := line.Message.Usage
		summary.InputTokens += u.InputTokens
		summary.OutputTokens += u.OutputTokens
		summary.CacheCreationInputTokens += u.CacheCreationInputTokens
		summary.CacheReadInputTokens += u.CacheReadInputTokens

		model := line.Message.Model
		if model == "" {
			continue
		}

		mt := summary.ModelUsage[model]
		mt.InputTokens += u.InputTokens
		mt.OutputTokens += u.OutputTokens
		mt.CacheCreationInputTokens += u.CacheCreationInputTokens
		mt.CacheReadInputTokens += u.CacheReadInputTokens
		summary.ModelUsage[model] = mt
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	return summary, nil
}

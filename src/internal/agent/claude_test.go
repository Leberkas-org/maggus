package agent

import (
	"encoding/json"
	"testing"
)

func TestStreamUsage_JSONDeserialization(t *testing.T) {
	raw := `{
		"input_tokens": 3,
		"output_tokens": 24,
		"cache_creation_input_tokens": 13055,
		"cache_read_input_tokens": 6692
	}`

	var usage streamUsage
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		t.Fatalf("failed to unmarshal streamUsage: %v", err)
	}

	if usage.InputTokens != 3 {
		t.Errorf("InputTokens = %d, want 3", usage.InputTokens)
	}
	if usage.OutputTokens != 24 {
		t.Errorf("OutputTokens = %d, want 24", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 13055 {
		t.Errorf("CacheCreationInputTokens = %d, want 13055", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 6692 {
		t.Errorf("CacheReadInputTokens = %d, want 6692", usage.CacheReadInputTokens)
	}
}

func TestStreamUsage_JSONWithoutCacheFields(t *testing.T) {
	raw := `{"input_tokens": 100, "output_tokens": 50}`

	var usage streamUsage
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		t.Fatalf("failed to unmarshal streamUsage: %v", err)
	}

	if usage.CacheCreationInputTokens != 0 {
		t.Errorf("CacheCreationInputTokens = %d, want 0", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 0 {
		t.Errorf("CacheReadInputTokens = %d, want 0", usage.CacheReadInputTokens)
	}
}

func TestStreamEvent_ResultWithCacheUsage(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"usage": {
			"input_tokens": 3,
			"output_tokens": 24,
			"cache_creation_input_tokens": 13055,
			"cache_read_input_tokens": 6692
		}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if event.Usage == nil {
		t.Fatal("expected Usage to be non-nil")
	}
	if event.Usage.CacheCreationInputTokens != 13055 {
		t.Errorf("Usage.CacheCreationInputTokens = %d, want 13055", event.Usage.CacheCreationInputTokens)
	}
	if event.Usage.CacheReadInputTokens != 6692 {
		t.Errorf("Usage.CacheReadInputTokens = %d, want 6692", event.Usage.CacheReadInputTokens)
	}
}

func TestStreamEvent_ResultWithCostUSD(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"total_cost_usd": 0.0855,
		"usage": {
			"input_tokens": 3,
			"output_tokens": 24,
			"cache_creation_input_tokens": 13055,
			"cache_read_input_tokens": 6692
		}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if event.CostUSD != 0.0855 {
		t.Errorf("CostUSD = %f, want 0.0855", event.CostUSD)
	}
}

func TestStreamEvent_ResultWithoutCostUSD(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50
		}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if event.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", event.CostUSD)
	}
}

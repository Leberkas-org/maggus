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

func TestModelUsageEntry_JSONDeserialization(t *testing.T) {
	raw := `{
		"inputTokens": 3,
		"outputTokens": 24,
		"cacheReadInputTokens": 6692,
		"cacheCreationInputTokens": 13055,
		"costUSD": 0.0855
	}`

	var entry modelUsageEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("failed to unmarshal modelUsageEntry: %v", err)
	}

	if entry.InputTokens != 3 {
		t.Errorf("InputTokens = %d, want 3", entry.InputTokens)
	}
	if entry.OutputTokens != 24 {
		t.Errorf("OutputTokens = %d, want 24", entry.OutputTokens)
	}
	if entry.CacheReadInputTokens != 6692 {
		t.Errorf("CacheReadInputTokens = %d, want 6692", entry.CacheReadInputTokens)
	}
	if entry.CacheCreationInputTokens != 13055 {
		t.Errorf("CacheCreationInputTokens = %d, want 13055", entry.CacheCreationInputTokens)
	}
	if entry.CostUSD != 0.0855 {
		t.Errorf("CostUSD = %f, want 0.0855", entry.CostUSD)
	}
}

func TestStreamEvent_ResultWithModelUsage(t *testing.T) {
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
		},
		"modelUsage": {
			"claude-opus-4-6[1m]": {
				"inputTokens": 3,
				"outputTokens": 24,
				"cacheReadInputTokens": 6692,
				"cacheCreationInputTokens": 13055,
				"costUSD": 0.0855
			}
		}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if len(event.ModelUsage) != 1 {
		t.Fatalf("ModelUsage length = %d, want 1", len(event.ModelUsage))
	}

	entry, ok := event.ModelUsage["claude-opus-4-6[1m]"]
	if !ok {
		t.Fatal("expected ModelUsage to contain key 'claude-opus-4-6[1m]'")
	}
	if entry.InputTokens != 3 {
		t.Errorf("InputTokens = %d, want 3", entry.InputTokens)
	}
	if entry.OutputTokens != 24 {
		t.Errorf("OutputTokens = %d, want 24", entry.OutputTokens)
	}
	if entry.CacheReadInputTokens != 6692 {
		t.Errorf("CacheReadInputTokens = %d, want 6692", entry.CacheReadInputTokens)
	}
	if entry.CacheCreationInputTokens != 13055 {
		t.Errorf("CacheCreationInputTokens = %d, want 13055", entry.CacheCreationInputTokens)
	}
	if entry.CostUSD != 0.0855 {
		t.Errorf("CostUSD = %f, want 0.0855", entry.CostUSD)
	}
}

func TestStreamEvent_ResultWithMultipleModels(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"modelUsage": {
			"claude-opus-4-6[1m]": {
				"inputTokens": 100,
				"outputTokens": 50,
				"cacheReadInputTokens": 5000,
				"cacheCreationInputTokens": 10000,
				"costUSD": 0.05
			},
			"claude-haiku-4-5-20251001": {
				"inputTokens": 200,
				"outputTokens": 100,
				"cacheReadInputTokens": 3000,
				"cacheCreationInputTokens": 0,
				"costUSD": 0.01
			}
		}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if len(event.ModelUsage) != 2 {
		t.Fatalf("ModelUsage length = %d, want 2", len(event.ModelUsage))
	}

	opus := event.ModelUsage["claude-opus-4-6[1m]"]
	if opus.InputTokens != 100 {
		t.Errorf("opus InputTokens = %d, want 100", opus.InputTokens)
	}
	if opus.CostUSD != 0.05 {
		t.Errorf("opus CostUSD = %f, want 0.05", opus.CostUSD)
	}

	haiku := event.ModelUsage["claude-haiku-4-5-20251001"]
	if haiku.InputTokens != 200 {
		t.Errorf("haiku InputTokens = %d, want 200", haiku.InputTokens)
	}
	if haiku.CostUSD != 0.01 {
		t.Errorf("haiku CostUSD = %f, want 0.01", haiku.CostUSD)
	}
}

func TestStreamEvent_ResultWithoutModelUsage(t *testing.T) {
	raw := `{
		"type": "result",
		"subtype": "success",
		"result": "done",
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`

	var event streamEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal streamEvent: %v", err)
	}

	if event.ModelUsage != nil {
		t.Errorf("ModelUsage = %v, want nil", event.ModelUsage)
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

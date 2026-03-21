package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/agent"
)

func TestExtractUsage_BasicSummation(t *testing.T) {
	content := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"a1"}
{"type":"assistant","message":{"model":"claude-opus-4-6","role":"assistant","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":20}},"uuid":"b1"}
{"type":"assistant","message":{"model":"claude-opus-4-6","role":"assistant","usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":5,"cache_read_input_tokens":30}},"uuid":"b2"}
`
	path := writeTemp(t, content)

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertInt(t, "InputTokens", 300, summary.InputTokens)
	assertInt(t, "OutputTokens", 130, summary.OutputTokens)
	assertInt(t, "CacheCreationInputTokens", 15, summary.CacheCreationInputTokens)
	assertInt(t, "CacheReadInputTokens", 50, summary.CacheReadInputTokens)

	if len(summary.ModelUsage) != 1 {
		t.Fatalf("expected 1 model, got %d", len(summary.ModelUsage))
	}

	mt := summary.ModelUsage["claude-opus-4-6"]
	assertInt(t, "model InputTokens", 300, mt.InputTokens)
	assertInt(t, "model OutputTokens", 130, mt.OutputTokens)
}

func TestExtractUsage_MultipleModels(t *testing.T) {
	content := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"a1"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"a2"}
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":150,"output_tokens":60,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"a3"}
`
	path := writeTemp(t, content)

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertInt(t, "total InputTokens", 450, summary.InputTokens)
	assertInt(t, "total OutputTokens", 190, summary.OutputTokens)

	if len(summary.ModelUsage) != 2 {
		t.Fatalf("expected 2 models, got %d", len(summary.ModelUsage))
	}

	opus := summary.ModelUsage["claude-opus-4-6"]
	assertInt(t, "opus InputTokens", 250, opus.InputTokens)
	assertInt(t, "opus OutputTokens", 110, opus.OutputTokens)

	sonnet := summary.ModelUsage["claude-sonnet-4-6"]
	assertInt(t, "sonnet InputTokens", 200, sonnet.InputTokens)
	assertInt(t, "sonnet OutputTokens", 80, sonnet.OutputTokens)
}

func TestExtractUsage_SkipsMalformedLines(t *testing.T) {
	content := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"a1"}
this is not json at all
{"broken json
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":200,"output_tokens":80,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"a2"}
`
	path := writeTemp(t, content)

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertInt(t, "InputTokens", 300, summary.InputTokens)
	assertInt(t, "OutputTokens", 130, summary.OutputTokens)
}

func TestExtractUsage_SkipsNonAssistantTypes(t *testing.T) {
	content := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"a1"}
{"type":"queue-operation","operation":"enqueue","timestamp":"2026-03-13T20:30:00.816Z"}
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"uuid":"b1"}
{"type":"tool_result","message":{"role":"tool","content":"ok"},"uuid":"c1"}
`
	path := writeTemp(t, content)

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertInt(t, "InputTokens", 100, summary.InputTokens)
	assertInt(t, "OutputTokens", 50, summary.OutputTokens)
}

func TestExtractUsage_EmptyFile(t *testing.T) {
	path := writeTemp(t, "")

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertInt(t, "InputTokens", 0, summary.InputTokens)
	assertInt(t, "OutputTokens", 0, summary.OutputTokens)
	if len(summary.ModelUsage) != 0 {
		t.Fatalf("expected empty model usage, got %d entries", len(summary.ModelUsage))
	}
}

func TestExtractUsage_FileNotFound(t *testing.T) {
	_, err := ExtractUsage("/nonexistent/path/session.jsonl")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtractUsage_ModelUsageMatchesAgentModelTokens(t *testing.T) {
	// Verify that the returned ModelUsage values are compatible with agent.ModelTokens
	content := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":20}},"uuid":"a1"}
`
	path := writeTemp(t, content)

	summary, err := ExtractUsage(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mt := summary.ModelUsage["claude-opus-4-6"]
	// Verify the type is agent.ModelTokens
	var _ agent.ModelTokens = mt
	assertInt(t, "CacheCreationInputTokens", 10, mt.CacheCreationInputTokens)
	assertInt(t, "CacheReadInputTokens", 20, mt.CacheReadInputTokens)
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func assertInt(t *testing.T, name string, want, got int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: want %d, got %d", name, want, got)
	}
}

package agent

import (
	"testing"
	"time"
)

func TestErrInterrupted_Message(t *testing.T) {
	want := "interrupted by user"
	if got := ErrInterrupted.Error(); got != want {
		t.Errorf("ErrInterrupted.Error() = %q, want %q", got, want)
	}
}

func TestStatusMsg(t *testing.T) {
	msg := StatusMsg{Status: "Thinking..."}
	if msg.Status != "Thinking..." {
		t.Errorf("StatusMsg.Status = %q, want %q", msg.Status, "Thinking...")
	}
}

func TestOutputMsg(t *testing.T) {
	msg := OutputMsg{Text: "Hello, world!"}
	if msg.Text != "Hello, world!" {
		t.Errorf("OutputMsg.Text = %q, want %q", msg.Text, "Hello, world!")
	}
}

func TestToolMsg(t *testing.T) {
	ts := time.Date(2026, 3, 19, 10, 30, 0, 0, time.UTC)
	params := map[string]string{"file": "main.go", "line": "42"}
	msg := ToolMsg{
		Description: "Reading file",
		Type:        "Read",
		Params:      params,
		Timestamp:   ts,
	}

	if msg.Description != "Reading file" {
		t.Errorf("ToolMsg.Description = %q, want %q", msg.Description, "Reading file")
	}
	if msg.Type != "Read" {
		t.Errorf("ToolMsg.Type = %q, want %q", msg.Type, "Read")
	}
	if msg.Params["file"] != "main.go" {
		t.Errorf("ToolMsg.Params[\"file\"] = %q, want %q", msg.Params["file"], "main.go")
	}
	if msg.Params["line"] != "42" {
		t.Errorf("ToolMsg.Params[\"line\"] = %q, want %q", msg.Params["line"], "42")
	}
	if !msg.Timestamp.Equal(ts) {
		t.Errorf("ToolMsg.Timestamp = %v, want %v", msg.Timestamp, ts)
	}
}

func TestToolMsg_TimestampPrecision(t *testing.T) {
	// Verify nanosecond precision is preserved.
	ts := time.Date(2026, 1, 15, 8, 30, 45, 123456789, time.UTC)
	msg := ToolMsg{Timestamp: ts}

	if msg.Timestamp.Nanosecond() != 123456789 {
		t.Errorf("nanosecond precision lost: got %d, want %d", msg.Timestamp.Nanosecond(), 123456789)
	}
	if !msg.Timestamp.Equal(ts) {
		t.Errorf("ToolMsg.Timestamp = %v, want %v", msg.Timestamp, ts)
	}
}

func TestSkillMsg(t *testing.T) {
	msg := SkillMsg{Name: "brainstorming"}
	if msg.Name != "brainstorming" {
		t.Errorf("SkillMsg.Name = %q, want %q", msg.Name, "brainstorming")
	}
}

func TestMCPMsg(t *testing.T) {
	msg := MCPMsg{Name: "fetch"}
	if msg.Name != "fetch" {
		t.Errorf("MCPMsg.Name = %q, want %q", msg.Name, "fetch")
	}
}

func TestUsageMsg(t *testing.T) {
	msg := UsageMsg{InputTokens: 1500, OutputTokens: 300}
	if msg.InputTokens != 1500 {
		t.Errorf("UsageMsg.InputTokens = %d, want %d", msg.InputTokens, 1500)
	}
	if msg.OutputTokens != 300 {
		t.Errorf("UsageMsg.OutputTokens = %d, want %d", msg.OutputTokens, 300)
	}
}

func TestUsageMsg_CacheFields(t *testing.T) {
	msg := UsageMsg{
		InputTokens:              3,
		OutputTokens:             24,
		CacheCreationInputTokens: 13055,
		CacheReadInputTokens:     6692,
	}
	if msg.CacheCreationInputTokens != 13055 {
		t.Errorf("UsageMsg.CacheCreationInputTokens = %d, want %d", msg.CacheCreationInputTokens, 13055)
	}
	if msg.CacheReadInputTokens != 6692 {
		t.Errorf("UsageMsg.CacheReadInputTokens = %d, want %d", msg.CacheReadInputTokens, 6692)
	}
}

func TestUsageMsg_CacheFieldsZero(t *testing.T) {
	msg := UsageMsg{InputTokens: 100, OutputTokens: 50}
	if msg.CacheCreationInputTokens != 0 {
		t.Errorf("UsageMsg.CacheCreationInputTokens = %d, want 0", msg.CacheCreationInputTokens)
	}
	if msg.CacheReadInputTokens != 0 {
		t.Errorf("UsageMsg.CacheReadInputTokens = %d, want 0", msg.CacheReadInputTokens)
	}
}

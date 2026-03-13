package agent

import (
	"strings"
	"testing"
)

func TestNew_DefaultToClaude(t *testing.T) {
	a, err := New("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "claude" {
		t.Errorf("expected agent name %q, got %q", "claude", a.Name())
	}
}

func TestNew_ExplicitClaude(t *testing.T) {
	a, err := New("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "claude" {
		t.Errorf("expected agent name %q, got %q", "claude", a.Name())
	}
}

func TestNew_ExplicitOpenCode(t *testing.T) {
	a, err := New("opencode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "opencode" {
		t.Errorf("expected agent name %q, got %q", "opencode", a.Name())
	}
}

func TestNew_UnknownAgent(t *testing.T) {
	_, err := New("unknown-agent")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error should mention 'unknown agent', got: %v", err)
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error should list available agent 'claude', got: %v", err)
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("error should list available agent 'opencode', got: %v", err)
	}
}

package runner

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunClaudeReturnsErrorWhenClaudeNotFound(t *testing.T) {
	// Use a PATH that won't find claude
	t.Setenv("PATH", "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewTUIModel("", "dev", "test-fp", func() {})
	p := tea.NewProgram(m)

	err := RunClaude(ctx, "test prompt", "", p)
	if err == nil {
		t.Fatal("expected error when claude is not on PATH")
	}
}

func TestErrInterruptedMessage(t *testing.T) {
	if ErrInterrupted.Error() != "interrupted by user" {
		t.Fatalf("unexpected error message: %s", ErrInterrupted.Error())
	}
}

func TestDescribeToolUse(t *testing.T) {
	tests := []struct {
		tool   string
		input  toolInput
		expect string
	}{
		{"Bash", toolInput{Description: "run tests"}, "Bash: run tests"},
		{"Bash", toolInput{Command: "go test ./..."}, "Bash: go test ./..."},
		{"Read", toolInput{FilePath: "/foo/bar.go"}, "Read: /foo/bar.go"},
		{"Edit", toolInput{FilePath: "/foo/bar.go"}, "Edit: /foo/bar.go"},
		{"Write", toolInput{FilePath: "/foo/bar.go"}, "Write: /foo/bar.go"},
		{"Glob", toolInput{Pattern: "**/*.go"}, "Glob: **/*.go"},
		{"Grep", toolInput{Pattern: "TODO"}, "Grep: TODO"},
		{"Skill", toolInput{Skill: "commit"}, "Skill: commit"},
		{"mcp__myserver__mytool", toolInput{}, "MCP myserver: mytool"},
		{"UnknownTool", toolInput{}, "UnknownTool"},
	}
	for _, tt := range tests {
		got := describeToolUse(tt.tool, tt.input)
		if got != tt.expect {
			t.Errorf("describeToolUse(%q, ...) = %q, want %q", tt.tool, got, tt.expect)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short string: got %q", got)
	}
	if got := truncate("hello world!", 8); got != "hello..." {
		t.Errorf("truncate long string: got %q", got)
	}
}

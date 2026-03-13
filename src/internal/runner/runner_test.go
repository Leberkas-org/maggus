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

	m := NewTUIModel("", "dev", "test-fp", func() {}, BannerInfo{})
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

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input  int
		expect string
	}{
		{0, "0"},
		{234, "234"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1.5k"},
		{12345, "12.3k"},
		{100000, "100k"},
		{1234567, "1234.6k"},
	}
	for _, tt := range tests {
		got := FormatTokens(tt.input)
		if got != tt.expect {
			t.Errorf("FormatTokens(%d) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestUsageAccumulation(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})

	// Initially no usage data
	if m.hasUsageData {
		t.Error("expected hasUsageData to be false initially")
	}
	if m.totalInputTokens != 0 || m.totalOutputTokens != 0 {
		t.Error("expected zero tokens initially")
	}

	// Start first iteration
	updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 2, TaskID: "TASK-001", TaskTitle: "First task"})
	m = updated.(tuiModel)

	// Receive usage data
	updated, _ = m.Update(UsageMsg{InputTokens: 1000, OutputTokens: 500})
	m = updated.(tuiModel)

	if !m.hasUsageData {
		t.Error("expected hasUsageData to be true after receiving usage")
	}
	if m.iterInputTokens != 1000 || m.iterOutputTokens != 500 {
		t.Errorf("iter tokens: got %d/%d, want 1000/500", m.iterInputTokens, m.iterOutputTokens)
	}
	if m.totalInputTokens != 1000 || m.totalOutputTokens != 500 {
		t.Errorf("total tokens: got %d/%d, want 1000/500", m.totalInputTokens, m.totalOutputTokens)
	}

	// Start second iteration — should save first task's usage
	updated, _ = m.Update(IterationStartMsg{Current: 2, Total: 2, TaskID: "TASK-002", TaskTitle: "Second task"})
	m = updated.(tuiModel)

	if len(m.taskUsages) != 1 {
		t.Fatalf("expected 1 task usage entry, got %d", len(m.taskUsages))
	}
	if m.taskUsages[0].TaskID != "TASK-001" {
		t.Errorf("expected task ID TASK-001, got %s", m.taskUsages[0].TaskID)
	}
	if m.taskUsages[0].InputTokens != 1000 || m.taskUsages[0].OutputTokens != 500 {
		t.Errorf("task usage: got %d/%d, want 1000/500", m.taskUsages[0].InputTokens, m.taskUsages[0].OutputTokens)
	}
	if m.iterInputTokens != 0 || m.iterOutputTokens != 0 {
		t.Error("expected iter tokens reset to 0 after new iteration")
	}

	// Receive usage for second iteration
	updated, _ = m.Update(UsageMsg{InputTokens: 2000, OutputTokens: 800})
	m = updated.(tuiModel)

	// Cumulative should reflect both iterations
	if m.totalInputTokens != 3000 || m.totalOutputTokens != 1300 {
		t.Errorf("cumulative tokens: got %d/%d, want 3000/1300", m.totalInputTokens, m.totalOutputTokens)
	}

	// Summary should save last iteration's usage
	updated, _ = m.Update(SummaryMsg{Data: SummaryData{TasksCompleted: 2, TasksTotal: 2}})
	m = updated.(tuiModel)

	if len(m.taskUsages) != 2 {
		t.Fatalf("expected 2 task usage entries after summary, got %d", len(m.taskUsages))
	}
	if m.taskUsages[1].TaskID != "TASK-002" {
		t.Errorf("expected task ID TASK-002, got %s", m.taskUsages[1].TaskID)
	}
}

func TestUsageNAWhenNoData(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})

	// Start an iteration with no usage data sent
	updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test"})
	m = updated.(tuiModel)

	// The view should contain "N/A" when no usage data received
	view := m.renderView()
	if !contains(view, "N/A") {
		t.Error("expected 'N/A' in view when no usage data received")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

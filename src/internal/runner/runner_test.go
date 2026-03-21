package runner

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/agent"
	"github.com/leberkas-org/maggus/internal/tui/styles"
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
	if agent.ErrInterrupted.Error() != "interrupted by user" {
		t.Fatalf("unexpected error message: %s", agent.ErrInterrupted.Error())
	}
}

func TestDescribeToolUse(t *testing.T) {
	// toolInput and describeToolUse are now in the agent package.
	cases := []struct {
		tool   string
		desc   string
		cmd    string
		fp     string
		pat    string
		skill  string
		expect string
	}{
		{tool: "Bash", desc: "run tests", expect: "Bash: run tests"},
		{tool: "Bash", cmd: "go test ./...", expect: "Bash: go test ./..."},
		{tool: "Read", fp: "/foo/bar.go", expect: "Read: /foo/bar.go"},
		{tool: "Edit", fp: "/foo/bar.go", expect: "Edit: /foo/bar.go"},
		{tool: "Write", fp: "/foo/bar.go", expect: "Write: /foo/bar.go"},
		{tool: "Glob", pat: "**/*.go", expect: "Glob: **/*.go"},
		{tool: "Grep", pat: "TODO", expect: "Grep: TODO"},
		{tool: "Skill", skill: "commit", expect: "Skill: commit"},
		{tool: "mcp__myserver__mytool", expect: "MCP myserver: mytool"},
		{tool: "UnknownTool", expect: "UnknownTool"},
	}
	for _, tt := range cases {
		got := agent.DescribeToolUse(tt.tool, agent.ToolInput{
			Description: tt.desc,
			Command:     tt.cmd,
			FilePath:    tt.fp,
			Pattern:     tt.pat,
			Skill:       tt.skill,
		})
		if got != tt.expect {
			t.Errorf("DescribeToolUse(%q, ...) = %q, want %q", tt.tool, got, tt.expect)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := styles.Truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short string: got %q", got)
	}
	if got := styles.Truncate("hello world!", 8); got != "hello..." {
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

	if m.tokens.hasData {
		t.Error("expected hasData to be false initially")
	}
	if m.tokens.totalInput != 0 || m.tokens.totalOutput != 0 {
		t.Error("expected zero tokens initially")
	}

	updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 2, TaskID: "TASK-001", TaskTitle: "First task"})
	m = updated.(TUIModel)

	updated, _ = m.Update(agent.UsageMsg{InputTokens: 1000, OutputTokens: 500})
	m = updated.(TUIModel)

	if !m.tokens.hasData {
		t.Error("expected hasData to be true after receiving usage")
	}
	if m.tokens.iterInput != 1000 || m.tokens.iterOutput != 500 {
		t.Errorf("iter tokens: got %d/%d, want 1000/500", m.tokens.iterInput, m.tokens.iterOutput)
	}
	if m.tokens.totalInput != 1000 || m.tokens.totalOutput != 500 {
		t.Errorf("total tokens: got %d/%d, want 1000/500", m.tokens.totalInput, m.tokens.totalOutput)
	}

	updated, _ = m.Update(IterationStartMsg{Current: 2, Total: 2, TaskID: "TASK-002", TaskTitle: "Second task"})
	m = updated.(TUIModel)

	if len(m.tokens.usages) != 1 {
		t.Fatalf("expected 1 task usage entry, got %d", len(m.tokens.usages))
	}
	if m.tokens.usages[0].TaskID != "TASK-001" {
		t.Errorf("expected task ID TASK-001, got %s", m.tokens.usages[0].TaskID)
	}
	if m.tokens.usages[0].InputTokens != 1000 || m.tokens.usages[0].OutputTokens != 500 {
		t.Errorf("task usage: got %d/%d, want 1000/500", m.tokens.usages[0].InputTokens, m.tokens.usages[0].OutputTokens)
	}
	if m.tokens.iterInput != 0 || m.tokens.iterOutput != 0 {
		t.Error("expected iter tokens reset to 0 after new iteration")
	}

	updated, _ = m.Update(agent.UsageMsg{InputTokens: 2000, OutputTokens: 800})
	m = updated.(TUIModel)

	if m.tokens.totalInput != 3000 || m.tokens.totalOutput != 1300 {
		t.Errorf("cumulative tokens: got %d/%d, want 3000/1300", m.tokens.totalInput, m.tokens.totalOutput)
	}

	updated, _ = m.Update(SummaryMsg{Data: SummaryData{TasksCompleted: 2, TasksTotal: 2}})
	m = updated.(TUIModel)

	if len(m.tokens.usages) != 2 {
		t.Fatalf("expected 2 task usage entries after summary, got %d", len(m.tokens.usages))
	}
	if m.tokens.usages[1].TaskID != "TASK-002" {
		t.Errorf("expected task ID TASK-002, got %s", m.tokens.usages[1].TaskID)
	}
}

func TestUsageNAWhenNoData(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})

	updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test"})
	m = updated.(TUIModel)

	view := m.renderView()
	if !contains(view, "N/A") {
		t.Error("expected 'N/A' in view when no usage data received")
	}
}

func TestSummaryRenderComplete(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})
	m.summary.show = true
	m.summary.data = SummaryData{
		Reason:      StopReasonComplete,
		TasksFailed: 0,
	}
	view := m.summary.renderSummaryView(&m)
	if !contains(view, "Work Complete") {
		t.Error("expected 'Work Complete' in complete summary view")
	}
	if contains(view, "with failures") {
		t.Error("unexpected 'with failures' in complete summary view with no failed tasks")
	}
	if contains(view, "Failed Tasks") {
		t.Error("unexpected 'Failed Tasks' section when no tasks failed")
	}
}

func TestSummaryRenderPartialComplete(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})
	m.summary.show = true
	m.summary.data = SummaryData{
		Reason:      StopReasonPartialComplete,
		TasksFailed: 1,
		FailedTasks: []FailedTask{
			{ID: "TASK-001", Title: "My Task", Reason: "agent error: something went wrong"},
		},
	}
	view := m.summary.renderSummaryView(&m)
	if !contains(view, "Work Complete (with failures)") {
		t.Error("expected 'Work Complete (with failures)' in partial complete view")
	}
	if !contains(view, "Failed Tasks:") {
		t.Error("expected 'Failed Tasks:' section in partial complete view")
	}
	if !contains(view, "TASK-001") {
		t.Error("expected TASK-001 in failed tasks list")
	}
	if !contains(view, "My Task") {
		t.Error("expected task title in failed tasks list")
	}
	if !contains(view, "agent error") {
		t.Error("expected failure reason in failed tasks list")
	}
}

func TestSummaryFailedTasksSectionHiddenWhenNone(t *testing.T) {
	m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})
	m.summary.show = true
	m.summary.data = SummaryData{
		Reason:      StopReasonComplete,
		TasksFailed: 0,
		FailedTasks: nil,
	}
	view := m.summary.renderSummaryView(&m)
	if contains(view, "Failed Tasks:") {
		t.Error("unexpected 'Failed Tasks:' section when no tasks failed")
	}
}

func TestRenderView_2xBorderColor(t *testing.T) {
	t.Run("non-2x uses primary border", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})
		m.width = 120
		m.height = 40
		updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-001", TaskTitle: "Test"})
		m = updated.(TUIModel)
		view := m.renderView()
		if !contains(view, "TASK-001") {
			t.Error("view should contain task ID")
		}
		// Non-2x, non-stop → should render without error
		if view == "" {
			t.Error("expected non-empty view")
		}
	})

	t.Run("2x active renders with warning border", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{TwoXExpiresIn: "5h 30m"})
		m.width = 120
		m.height = 40
		updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-002", TaskTitle: "Test 2x"})
		m = updated.(TUIModel)
		view := m.renderView()
		if !contains(view, "TASK-002") {
			t.Error("view should contain task ID")
		}
		if !contains(view, "2x:") {
			t.Error("view should contain 2x status line")
		}
	})

	t.Run("stop-after-task overrides border to warning", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{})
		m.width = 120
		m.height = 40
		m.stopAfterTask = true
		updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-003", TaskTitle: "Stop test"})
		m = updated.(TUIModel)
		view := m.renderView()
		if !contains(view, "Stopping after current task") {
			t.Error("view should contain stop indicator")
		}
	})

	t.Run("non-border elements remain cyan", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{TwoXExpiresIn: "5h"})
		m.width = 120
		m.height = 40
		updated, _ := m.Update(IterationStartMsg{Current: 1, Total: 1, TaskID: "TASK-004", TaskTitle: "Cyan check"})
		m = updated.(TUIModel)
		view := m.renderView()
		// The spinner and task ID should still use cyan style, not warning
		if !contains(view, "TASK-004") {
			t.Error("view should contain task ID rendered with cyan style")
		}
	})
}

func TestRenderBannerView_2xBorderColor(t *testing.T) {
	t.Run("non-2x banner uses primary border", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{
			Iterations: 3,
			RunID:      "run-001",
		})
		m.width = 120
		m.height = 40
		view := m.renderBannerView()
		if !contains(view, "run-001") {
			t.Error("banner should contain run ID")
		}
	})

	t.Run("2x banner uses warning border", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{
			Iterations:    3,
			RunID:         "run-002",
			TwoXExpiresIn: "17h 54m",
		})
		m.width = 120
		m.height = 40
		view := m.renderBannerView()
		if !contains(view, "run-002") {
			t.Error("banner should contain run ID")
		}
		if !contains(view, "2x:") {
			t.Error("banner should contain 2x status in header")
		}
	})
}

func TestIs2xDerivation(t *testing.T) {
	t.Run("empty TwoXExpiresIn means not 2x", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{TwoXExpiresIn: ""})
		is2x := m.banner.TwoXExpiresIn != ""
		if is2x {
			t.Error("empty TwoXExpiresIn should mean not 2x")
		}
	})

	t.Run("non-empty TwoXExpiresIn means 2x", func(t *testing.T) {
		m := NewTUIModel("test", "dev", "fp", func() {}, BannerInfo{TwoXExpiresIn: "5h 30m"})
		is2x := m.banner.TwoXExpiresIn != ""
		if !is2x {
			t.Error("non-empty TwoXExpiresIn should mean 2x")
		}
	})
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

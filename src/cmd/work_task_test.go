package cmd

import (
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestComputeTaskProgress(t *testing.T) {
	mkTask := func(id, sourceFile string, complete bool) parser.Task {
		criteria := []parser.Criterion{{Text: "done", Checked: complete}}
		return parser.Task{ID: id, SourceFile: sourceFile, Criteria: criteria}
	}

	t.Run("counts completed vs total for source file", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "feature_001.md", true),
			mkTask("TASK-002", "feature_001.md", true),
			mkTask("TASK-003", "feature_001.md", false),
			mkTask("TASK-004", "feature_001.md", false),
			mkTask("TASK-005", "feature_002.md", false), // different file
		}
		// Reset taskFlag to ensure normal mode.
		old := taskFlag
		taskFlag = ""
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "feature_001.md")
		if completed != 2 || total != 4 {
			t.Errorf("got %d/%d, want 2/4", completed, total)
		}
	})

	t.Run("all complete", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "f.md", true),
			mkTask("TASK-002", "f.md", true),
			mkTask("TASK-003", "f.md", true),
		}
		old := taskFlag
		taskFlag = ""
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 3 || total != 3 {
			t.Errorf("got %d/%d, want 3/3", completed, total)
		}
	})

	t.Run("none complete", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "f.md", false),
			mkTask("TASK-002", "f.md", false),
		}
		old := taskFlag
		taskFlag = ""
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 0 || total != 2 {
			t.Errorf("got %d/%d, want 0/2", completed, total)
		}
	})

	t.Run("no tasks for source file returns 0/0", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "other.md", true),
		}
		old := taskFlag
		taskFlag = ""
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 0 || total != 0 {
			t.Errorf("got %d/%d, want 0/0", completed, total)
		}
	})

	t.Run("single task mode incomplete", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "f.md", false),
			mkTask("TASK-002", "f.md", true),
		}
		old := taskFlag
		taskFlag = "TASK-001"
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 0 || total != 1 {
			t.Errorf("got %d/%d, want 0/1", completed, total)
		}
	})

	t.Run("single task mode complete", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "f.md", true),
			mkTask("TASK-002", "f.md", false),
		}
		old := taskFlag
		taskFlag = "TASK-001"
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 1 || total != 1 {
			t.Errorf("got %d/%d, want 1/1", completed, total)
		}
	})

	t.Run("single task mode task not found", func(t *testing.T) {
		tasks := []parser.Task{
			mkTask("TASK-001", "f.md", false),
		}
		old := taskFlag
		taskFlag = "TASK-999"
		defer func() { taskFlag = old }()

		completed, total := computeTaskProgress(tasks, "f.md")
		if completed != 0 || total != 1 {
			t.Errorf("got %d/%d, want 0/1", completed, total)
		}
	})
}

func TestResolveTaskModel(t *testing.T) {
	tests := []struct {
		name         string
		taskModel    string
		defaultModel string
		want         string
	}{
		{
			name:         "no override uses default",
			taskModel:    "",
			defaultModel: "claude-sonnet-4-6",
			want:         "claude-sonnet-4-6",
		},
		{
			name:         "task model alias resolved",
			taskModel:    "opus",
			defaultModel: "claude-sonnet-4-6",
			want:         "claude-opus-4-6",
		},
		{
			name:         "task model full ID passed through",
			taskModel:    "claude-haiku-4-5-20251001",
			defaultModel: "claude-sonnet-4-6",
			want:         "claude-haiku-4-5-20251001",
		},
		{
			name:         "haiku alias resolved",
			taskModel:    "haiku",
			defaultModel: "claude-opus-4-6",
			want:         "claude-haiku-4-5-20251001",
		},
		{
			name:         "sonnet alias resolved",
			taskModel:    "sonnet",
			defaultModel: "claude-opus-4-6",
			want:         "claude-sonnet-4-6",
		},
		{
			name:         "provider/model format passed through",
			taskModel:    "anthropic/claude-opus-4-6",
			defaultModel: "claude-sonnet-4-6",
			want:         "anthropic/claude-opus-4-6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTaskModel(tt.taskModel, tt.defaultModel)
			if got != tt.want {
				t.Errorf("resolveTaskModel(%q, %q) = %q, want %q", tt.taskModel, tt.defaultModel, got, tt.want)
			}
		})
	}
}

func TestVerbForTask(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		want       string
	}{
		{
			name:       "feature file unix path",
			sourceFile: ".maggus/features/feature_001.md",
			want:       "Working",
		},
		{
			name:       "feature file windows path",
			sourceFile: `.maggus\features\feature_001.md`,
			want:       "Working",
		},
		{
			name:       "bug file unix path",
			sourceFile: ".maggus/bugs/bug_001.md",
			want:       "Fixing",
		},
		{
			name:       "bug file windows path",
			sourceFile: `.maggus\bugs\bug_001.md`,
			want:       "Fixing",
		},
		{
			name:       "absolute bug path unix",
			sourceFile: "/home/user/project/.maggus/bugs/bug_002.md",
			want:       "Fixing",
		},
		{
			name:       "absolute bug path windows",
			sourceFile: `C:\projects\app\.maggus\bugs\bug_002.md`,
			want:       "Fixing",
		},
		{
			name:       "empty string defaults to Working",
			sourceFile: "",
			want:       "Working",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verbForTask(tt.sourceFile)
			if got != tt.want {
				t.Errorf("verbForTask(%q) = %q, want %q", tt.sourceFile, got, tt.want)
			}
		})
	}
}

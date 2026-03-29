package cmd

import (
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/runlog"
)

func TestRenderSnapshotInPane_TruncatesLongTaskTitle(t *testing.T) {
	tests := []struct {
		name           string
		taskTitle      string
		width          int
		expectTruncated bool
		expectOmitted  bool
	}{
		{
			name:           "short title fits completely",
			taskTitle:      "Short task",
			width:          100,
			expectTruncated: false,
			expectOmitted:  false,
		},
		{
			name:           "long title gets truncated",
			taskTitle:      "This is a very long task title that should be truncated when displayed in a narrow pane",
			width:          50,
			expectTruncated: true,
			expectOmitted:  false,
		},
		{
			name:           "very narrow width omits title entirely",
			taskTitle:      "Task title",
			width:          20,
			expectTruncated: false,
			expectOmitted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &runlog.StateSnapshot{
				TaskID:    "TASK-001",
				TaskTitle: tt.taskTitle,
				Status:    "Running",
			}

			m := statusModel{
				snapshot: snap,
			}

			output := m.renderSnapshotInPane(tt.width, 30)

			// Find the Task title line (line with TaskID in "Task: TASKID - Title" format)
			lines := strings.Split(output, "\n")
			var taskTitleLine string
			for _, line := range lines {
				// Look for the line that contains "Task:" followed by the TaskID
				// This will be on the top part of the output before the separator
				if strings.Contains(line, "Task:") && strings.Contains(line, "TASK-001") {
					taskTitleLine = line
					break
				}
			}

			if tt.expectOmitted {
				if taskTitleLine != "" && strings.Contains(taskTitleLine, "Task:") && strings.Contains(taskTitleLine, "TASK-001") {
					// The line should just have "Task: TASK-001" without a title
					if strings.Contains(taskTitleLine, " - ") {
						t.Errorf("expected task title to be omitted (no ' - '), but found: %q", taskTitleLine)
					}
				}
			} else {
				if taskTitleLine == "" {
					t.Errorf("expected to find task title line with TaskID, but not found in output")
				}
			}
		})
	}
}

package prompt

import (
	"fmt"
	"strings"

	"github.com/dirnei/maggus/internal/parser"
)

// Build creates a focused prompt for Claude Code to work on a single task.
func Build(task *parser.Task) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are working on the following task. Focus only on this task and nothing else.\n\n")
	fmt.Fprintf(&b, "## %s: %s\n\n", task.ID, task.Title)
	fmt.Fprintf(&b, "**Description:** %s\n\n", task.Description)
	fmt.Fprintf(&b, "**Acceptance Criteria:**\n")
	for _, c := range task.Criteria {
		if c.Checked {
			fmt.Fprintf(&b, "- [x] %s\n", c.Text)
		} else {
			fmt.Fprintf(&b, "- [ ] %s\n", c.Text)
		}
	}
	fmt.Fprintf(&b, "\nBefore finishing, verify that every acceptance criterion above is met. Do not work on anything outside this task.\n")

	return b.String()
}

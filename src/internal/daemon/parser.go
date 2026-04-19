package daemon

import (
	"fmt"
	"regexp"
	"strings"
)

type ParsedPlan struct {
	Title   string
	Context string
	Tasks   []ParsedTask
}

type ParsedTask struct {
	Number  int
	Title   string
	Content string
}

var taskHeadingRe = regexp.MustCompile(`(?m)^## Task (\d+):\s*(.+)$`)

func Parse(content string) (*ParsedPlan, error) {
	plan := &ParsedPlan{}

	// Extract title from first # heading
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
		plan.Title = strings.TrimPrefix(strings.TrimSpace(lines[0]), "# ")
	}

	// Find all task headings
	locs := taskHeadingRe.FindAllStringIndex(content, -1)
	matches := taskHeadingRe.FindAllStringSubmatch(content, -1)

	if len(locs) == 0 {
		return nil, fmt.Errorf("no tasks found (expected ## Task N: headings)")
	}

	// Context = everything before the first task heading
	plan.Context = strings.TrimSpace(content[:locs[0][0]])

	// Strip agentic worker blockquote from context
	plan.Context = stripBlockquote(plan.Context)

	// Split tasks
	for i, match := range matches {
		taskNum := i + 1
		start := locs[i][0]
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}

		plan.Tasks = append(plan.Tasks, ParsedTask{
			Number:  taskNum,
			Title:   match[2],
			Content: strings.TrimSpace(content[start:end]),
		})
	}

	return plan, nil
}

func stripBlockquote(text string) string {
	var result []string
	inBlockquote := false

	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "> **For agentic") {
			inBlockquote = true
			continue
		}
		if inBlockquote {
			if trimmed == "" || !strings.HasPrefix(trimmed, ">") {
				inBlockquote = false
			} else {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

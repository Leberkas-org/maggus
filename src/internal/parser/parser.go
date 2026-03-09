package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var taskHeadingRe = regexp.MustCompile(`^###\s+TASK-(\d+):\s+(.+)$`)

type Criterion struct {
	Text    string
	Checked bool
	Blocked bool // marked as [x] ⚠️ BLOCKED: ...
}

type Task struct {
	ID          string
	Title       string
	Description string
	Criteria    []Criterion
	SourceFile  string
}

func (t *Task) IsComplete() bool {
	if len(t.Criteria) == 0 {
		return false
	}
	for _, c := range t.Criteria {
		if !c.Checked {
			return false
		}
	}
	return true
}

// IsBlocked returns true if any criterion is marked as blocked.
func (t *Task) IsBlocked() bool {
	for _, c := range t.Criteria {
		if c.Blocked {
			return true
		}
	}
	return false
}

// IsWorkable returns true if the task is incomplete and not blocked.
func (t *Task) IsWorkable() bool {
	return !t.IsComplete() && !t.IsBlocked()
}

// ParseFile parses a single plan markdown file and returns all tasks found in it.
func ParseFile(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var tasks []Task
	var current *Task
	inDescription := false
	inCriteria := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for task heading
		if m := taskHeadingRe.FindStringSubmatch(line); m != nil {
			// Save previous task
			if current != nil {
				current.Description = strings.TrimSpace(current.Description)
				tasks = append(tasks, *current)
			}
			current = &Task{
				ID:         "TASK-" + m[1],
				Title:      strings.TrimSpace(m[2]),
				SourceFile: path,
			}
			inDescription = false
			inCriteria = false
			continue
		}

		if current == nil {
			continue
		}

		// Detect section markers
		if strings.HasPrefix(line, "**Description:**") {
			inDescription = true
			inCriteria = false
			// Grab inline text after the marker
			text := strings.TrimPrefix(line, "**Description:**")
			text = strings.TrimSpace(text)
			if text != "" {
				current.Description = text
			}
			continue
		}

		if strings.HasPrefix(line, "**Acceptance Criteria:**") {
			inCriteria = true
			inDescription = false
			continue
		}

		// A new section (bold marker or heading) ends the current section
		if strings.HasPrefix(line, "**") || strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") {
			inDescription = false
			inCriteria = false
			// If it's a non-task heading, finalize current task
			if strings.HasPrefix(line, "## ") {
				if current != nil {
					current.Description = strings.TrimSpace(current.Description)
					tasks = append(tasks, *current)
					current = nil
				}
			}
			continue
		}

		if inDescription {
			current.Description += line + "\n"
		}

		if inCriteria {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- [x] ") {
				text := strings.TrimPrefix(trimmed, "- [x] ")
				blocked := strings.Contains(text, "BLOCKED:")
				current.Criteria = append(current.Criteria, Criterion{
					Text:    text,
					Checked: true,
					Blocked: blocked,
				})
			} else if strings.HasPrefix(trimmed, "- [ ] ") {
				current.Criteria = append(current.Criteria, Criterion{
					Text:    strings.TrimPrefix(trimmed, "- [ ] "),
					Checked: false,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Don't forget the last task
	if current != nil {
		current.Description = strings.TrimSpace(current.Description)
		tasks = append(tasks, *current)
	}

	return tasks, nil
}

// FindNextIncomplete returns the first workable task (incomplete and not blocked), or nil if none.
func FindNextIncomplete(tasks []Task) *Task {
	for i := range tasks {
		if tasks[i].IsWorkable() {
			return &tasks[i]
		}
	}
	return nil
}

// ParsePlans finds all .maggus/plan_*.md files in the given directory and parses them.
// Tasks are returned in order: files sorted by name, tasks in document order within each file.
func ParsePlans(dir string) ([]Task, error) {
	pattern := filepath.Join(dir, ".maggus", "plan_*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	sort.Strings(files)

	var allTasks []Task
	for _, f := range files {
		tasks, err := ParseFile(f)
		if err != nil {
			return nil, err
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

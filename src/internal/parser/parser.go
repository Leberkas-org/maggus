package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var taskHeadingRe = regexp.MustCompile(`^###\s+(?:(IGNORED)\s+)?(TASK-[\w-]+?):\s+(.+)$`)

// TaskHeadingRe is the exported task heading regex for use by other packages (e.g. ignore/unignore commands).
var TaskHeadingRe = taskHeadingRe

type Criterion struct {
	Text    string
	Checked bool
	Blocked bool // unchecked criterion containing "BLOCKED:" — a checked BLOCKED: means resolved
}

type Task struct {
	ID          string
	Title       string
	Description string
	Criteria    []Criterion
	SourceFile  string
	Ignored     bool
}

type Plan struct {
	File    string
	Ignored bool
	Tasks   []Task
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

// IsWorkable returns true if the task is incomplete, not blocked, and not ignored.
func (t *Task) IsWorkable() bool {
	return !t.IsComplete() && !t.IsBlocked() && !t.Ignored
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
				ID:         m[2],
				Title:      strings.TrimSpace(m[3]),
				SourceFile: path,
				Ignored:    m[1] == "IGNORED",
			}
			inDescription = false

			continue
		}

		if current == nil {
			continue
		}

		// Detect section markers
		if strings.HasPrefix(line, "**Description:**") {
			inDescription = true

			// Grab inline text after the marker
			text := strings.TrimPrefix(line, "**Description:**")
			text = strings.TrimSpace(text)
			if text != "" {
				current.Description = text
			}
			continue
		}

		if strings.HasPrefix(line, "**Acceptance Criteria:**") {

			inDescription = false
			continue
		}

		// Checkbox lines are always treated as criteria (with or without section header)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [x] ") {

			inDescription = false
			current.Criteria = append(current.Criteria, Criterion{
				Text:    strings.TrimPrefix(trimmed, "- [x] "),
				Checked: true,
				Blocked: false, // checked items are resolved; never count as blocked
			})
			continue
		}
		if strings.HasPrefix(trimmed, "- [ ] ") {

			inDescription = false
			text := strings.TrimPrefix(trimmed, "- [ ] ")
			current.Criteria = append(current.Criteria, Criterion{
				Text:    text,
				Checked: false,
				Blocked: strings.HasPrefix(text, "BLOCKED:") || strings.HasPrefix(text, "⚠️ BLOCKED:"),
			})
			continue
		}

		// A new section (bold marker or heading) ends the current section
		if strings.HasPrefix(line, "**") || strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") {
			inDescription = false

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

// GlobPlanFiles returns all plan_*.md file paths in .maggus/, sorted numerically.
// If includeCompleted is false, files ending in _completed.md are excluded.
func GlobPlanFiles(dir string, includeCompleted bool) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "plan_*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	SortPlanFiles(files)

	if includeCompleted {
		return files, nil
	}

	filtered := files[:0]
	for _, f := range files {
		if !strings.HasSuffix(f, "_completed.md") {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

// IsIgnoredFile returns true if the given path is an ignored plan file (ends with _ignored.md).
func IsIgnoredFile(path string) bool {
	return strings.HasSuffix(path, "_ignored.md")
}

// ParsePlans finds all .maggus/plan_*.md files in the given directory and parses them.
// Files ending in _completed.md are excluded.
// Tasks are returned in order: files sorted by name, tasks in document order within each file.
// Tasks from _ignored plan files have Ignored set to true.
func ParsePlans(dir string) ([]Task, error) {
	files, err := GlobPlanFiles(dir, false)
	if err != nil {
		return nil, err
	}

	var allTasks []Task
	for _, f := range files {
		tasks, err := ParseFile(f)
		if err != nil {
			return nil, err
		}
		ignored := IsIgnoredFile(f)
		if ignored {
			for i := range tasks {
				tasks[i].Ignored = true
			}
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

// ParsePlansGrouped finds all .maggus/plan_*.md files and returns them as Plan structs.
// Files ending in _completed.md are excluded.
// Plans from _ignored files have Ignored set to true, and all their tasks inherit this flag.
func ParsePlansGrouped(dir string) ([]Plan, error) {
	files, err := GlobPlanFiles(dir, false)
	if err != nil {
		return nil, err
	}

	var plans []Plan
	for _, f := range files {
		tasks, err := ParseFile(f)
		if err != nil {
			return nil, err
		}
		ignored := IsIgnoredFile(f)
		if ignored {
			for i := range tasks {
				tasks[i].Ignored = true
			}
		}
		plans = append(plans, Plan{
			File:    f,
			Ignored: ignored,
			Tasks:   tasks,
		})
	}

	return plans, nil
}

// planNumberRe extracts the numeric part from plan filenames like "plan_10.md" or "plan_3_completed.md".
var planNumberRe = regexp.MustCompile(`plan_(\d+)`)

// SortPlanFiles sorts plan file paths by their numeric plan number (e.g. plan_8 before plan_10).
func SortPlanFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return extractPlanNumber(files[i]) < extractPlanNumber(files[j])
	})
}

// extractPlanNumber returns the numeric portion of a plan filename for sorting.
// Returns math.MaxInt if the number cannot be parsed, pushing unrecognised files to the end.
func extractPlanNumber(path string) int {
	base := filepath.Base(path)
	m := planNumberRe.FindStringSubmatch(base)
	if m == nil {
		return 1<<31 - 1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 1<<31 - 1
	}
	return n
}

// MarkCompletedPlans renames plan files where all tasks are complete (and none are blocked)
// by appending _completed before the .md extension (e.g. plan_1.md → plan_1_completed.md).
func MarkCompletedPlans(dir string) error {
	files, err := GlobPlanFiles(dir, false)
	if err != nil {
		return err
	}

	for _, f := range files {

		tasks, err := ParseFile(f)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			continue
		}

		allComplete := true
		for _, t := range tasks {
			if !t.IsComplete() || t.IsBlocked() {
				allComplete = false
				break
			}
		}

		if allComplete {
			newName := strings.TrimSuffix(f, ".md") + "_completed.md"
			if err := os.Rename(f, newName); err != nil {
				return fmt.Errorf("rename %s: %w", f, err)
			}
		}
	}

	return nil
}

// DeleteTask removes a task section (### TASK-ID: ... up to the next ### or EOF)
// from the given plan file. Returns an error if the task is not found.
func DeleteTask(filePath string, taskID string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	lines := strings.Split(string(data), "\n")
	start := -1
	end := len(lines)

	for i, line := range lines {
		if m := taskHeadingRe.FindStringSubmatch(line); m != nil {
			if m[2] == taskID {
				// Found the task — also consume blank lines before the heading
				start = i
				for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
					start--
				}
			} else if start >= 0 {
				// Next task heading — end of the section to delete
				end = i
				break
			}
		}
	}

	if start < 0 {
		return fmt.Errorf("task %s not found in %s", taskID, filePath)
	}

	result := append(lines[:start], lines[end:]...)
	return os.WriteFile(filePath, []byte(strings.Join(result, "\n")), 0644)
}

// UnblockCriterion reads the plan file, removes the "BLOCKED: " prefix from the
// matching criterion line, and writes the file back. Returns an error if the
// exact line cannot be found.
func UnblockCriterion(filePath string, c Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	oldLine := "- [ ] " + c.Text
	// Remove "BLOCKED: " or "⚠️ BLOCKED: " prefix from criterion text
	newText := c.Text
	if strings.HasPrefix(newText, "⚠️ BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "⚠️ BLOCKED: ")
	} else if strings.HasPrefix(newText, "BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "BLOCKED: ")
	}
	newLine := "- [ ] " + newText

	content := string(data)
	if !strings.Contains(content, oldLine) {
		return fmt.Errorf("criterion line not found in %s: %s", filepath.Base(filePath), c.Text)
	}

	content = strings.Replace(content, oldLine, newLine, 1)
	return os.WriteFile(filePath, []byte(content), 0o644)
}

// ResolveCriterion removes the BLOCKED: prefix from a criterion and marks it
// as checked (- [x]). This indicates the user has resolved the blocker themselves.
func ResolveCriterion(filePath string, c Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	oldLine := "- [ ] " + c.Text
	// Remove "BLOCKED: " or "⚠️ BLOCKED: " prefix from criterion text
	newText := c.Text
	if strings.HasPrefix(newText, "⚠️ BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "⚠️ BLOCKED: ")
	} else if strings.HasPrefix(newText, "BLOCKED: ") {
		newText = strings.TrimPrefix(newText, "BLOCKED: ")
	}
	newLine := "- [x] " + newText

	content := string(data)
	if !strings.Contains(content, oldLine) {
		return fmt.Errorf("criterion line not found in %s: %s", filepath.Base(filePath), c.Text)
	}

	content = strings.Replace(content, oldLine, newLine, 1)
	return os.WriteFile(filePath, []byte(content), 0o644)
}

// DeleteCriterion reads the plan file, removes the entire criterion line,
// and writes the file back. Returns an error if the exact line cannot be found.
func DeleteCriterion(filePath string, c Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	targetLine := "- [ ] " + c.Text
	lines := strings.Split(string(data), "\n")
	found := false
	var result []string
	for _, line := range lines {
		if !found && strings.TrimSpace(line) == targetLine {
			found = true
			continue // skip this line
		}
		result = append(result, line)
	}

	if !found {
		return fmt.Errorf("criterion line not found in %s: %s", filepath.Base(filePath), c.Text)
	}

	return os.WriteFile(filePath, []byte(strings.Join(result, "\n")), 0o644)
}

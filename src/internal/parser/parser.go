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

var taskHeadingRe = regexp.MustCompile(`^###\s+(?:(IGNORED)\s+)?((?:TASK|BUG)-[\w-]+?):\s+(.+)$`)

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

type Feature struct {
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

// ParseFile parses a single feature markdown file and returns all tasks found in it.
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

// GlobFeatureFiles returns all feature_*.md file paths in .maggus/features/, sorted numerically.
// If includeCompleted is false, files ending in _completed.md are excluded.
func GlobFeatureFiles(dir string, includeCompleted bool) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "features", "feature_*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	SortFeatureFiles(files)

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

// IsIgnoredFile returns true if the given path is an ignored feature or bug file (ends with _ignored.md).
func IsIgnoredFile(path string) bool {
	return strings.HasSuffix(path, "_ignored.md")
}

// ParseFeatures finds all .maggus/features/feature_*.md files in the given directory and parses them.
// Files ending in _completed.md are excluded.
// Tasks are returned in order: files sorted by name, tasks in document order within each file.
// Tasks from _ignored feature files have Ignored set to true.
func ParseFeatures(dir string) ([]Task, error) {
	files, err := GlobFeatureFiles(dir, false)
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

// ParseFeaturesGrouped finds all .maggus/features/feature_*.md files and returns them as Feature structs.
// Files ending in _completed.md are excluded.
// Features from _ignored files have Ignored set to true, and all their tasks inherit this flag.
func ParseFeaturesGrouped(dir string) ([]Feature, error) {
	files, err := GlobFeatureFiles(dir, false)
	if err != nil {
		return nil, err
	}

	var features []Feature
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
		features = append(features, Feature{
			File:    f,
			Ignored: ignored,
			Tasks:   tasks,
		})
	}

	return features, nil
}

// featureNumberRe extracts the numeric part from feature filenames like "feature_010.md" or "feature_003_completed.md".
var featureNumberRe = regexp.MustCompile(`feature_(\d+)`)

// SortFeatureFiles sorts feature file paths by their numeric feature number (e.g. feature_008 before feature_010).
func SortFeatureFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return extractFeatureNumber(files[i]) < extractFeatureNumber(files[j])
	})
}

// extractFeatureNumber returns the numeric portion of a feature filename for sorting.
// Returns math.MaxInt if the number cannot be parsed, pushing unrecognised files to the end.
func extractFeatureNumber(path string) int {
	base := filepath.Base(path)
	m := featureNumberRe.FindStringSubmatch(base)
	if m == nil {
		return 1<<31 - 1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 1<<31 - 1
	}
	return n
}

// MarkCompletedFeatures renames feature files where all tasks are complete (and none are blocked)
// by appending _completed before the .md extension (e.g. feature_001.md → feature_001_completed.md).
func MarkCompletedFeatures(dir string) error {
	files, err := GlobFeatureFiles(dir, false)
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
// from the given feature file. Returns an error if the task is not found.
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

// UnblockCriterion reads the feature file, removes the "BLOCKED: " prefix from the
// matching criterion line, and writes the file back. Returns an error if the
// exact line cannot be found.
func UnblockCriterion(filePath string, c Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read feature file: %w", err)
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
		return fmt.Errorf("read feature file: %w", err)
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

// DeleteCriterion reads the feature file, removes the entire criterion line,
// and writes the file back. Returns an error if the exact line cannot be found.
func DeleteCriterion(filePath string, c Criterion) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read feature file: %w", err)
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

// bugNumberRe extracts the numeric part from bug filenames like "bug_001.md" or "bug_003_completed.md".
var bugNumberRe = regexp.MustCompile(`bug_(\d+)`)

// legacyBugTaskRe matches legacy TASK-NNN headings (### TASK-NNN: or ### IGNORED TASK-NNN:) in bug files.
var legacyBugTaskRe = regexp.MustCompile(`^(###\s+(?:IGNORED\s+)?)TASK-(\d+):\s`)

// GlobBugFiles returns all bug_*.md file paths in .maggus/bugs/, sorted numerically.
// If includeCompleted is false, files ending in _completed.md are excluded.
func GlobBugFiles(dir string, includeCompleted bool) ([]string, error) {
	pattern := filepath.Join(dir, ".maggus", "bugs", "bug_*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	SortBugFiles(files)

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

// SortBugFiles sorts bug file paths by their numeric bug number (e.g. bug_001 before bug_010).
func SortBugFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return extractBugNumber(files[i]) < extractBugNumber(files[j])
	})
}

// extractBugNumber returns the numeric portion of a bug filename for sorting.
func extractBugNumber(path string) int {
	base := filepath.Base(path)
	m := bugNumberRe.FindStringSubmatch(base)
	if m == nil {
		return 1<<31 - 1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 1<<31 - 1
	}
	return n
}

// MigrateLegacyBugIDs rewrites legacy TASK-NNN headings in a bug file to BUG-NNN-XXX format.
// NNN is derived from the bug file number (e.g., bug_1.md → 001).
// Returns true if the file was modified.
func MigrateLegacyBugIDs(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	base := filepath.Base(path)
	m := bugNumberRe.FindStringSubmatch(base)
	if m == nil {
		return false, nil
	}
	bugNum, err := strconv.Atoi(m[1])
	if err != nil {
		return false, nil
	}
	bugPrefix := fmt.Sprintf("BUG-%03d", bugNum)

	lines := strings.Split(string(data), "\n")
	modified := false
	taskCounter := 0

	for i, line := range lines {
		if legacyBugTaskRe.MatchString(line) {
			taskCounter++
			newID := fmt.Sprintf("%s-%03d", bugPrefix, taskCounter)
			lines[i] = legacyBugTaskRe.ReplaceAllString(line, "${1}"+newID+": ")
			modified = true
		}
	}

	if modified {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", path, err)
		}
	}

	return modified, nil
}

// ParseBugs finds all .maggus/bugs/bug_*.md files, auto-migrates legacy IDs, and parses them.
// Files ending in _completed.md are excluded.
func ParseBugs(dir string) ([]Task, error) {
	files, err := GlobBugFiles(dir, false)
	if err != nil {
		return nil, err
	}

	var allTasks []Task
	for _, f := range files {
		if _, err := MigrateLegacyBugIDs(f); err != nil {
			return nil, err
		}
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

// ParseBugsGrouped finds all .maggus/bugs/bug_*.md files and returns them as Feature structs.
// Files ending in _completed.md are excluded. Auto-migrates legacy IDs before parsing.
func ParseBugsGrouped(dir string) ([]Feature, error) {
	files, err := GlobBugFiles(dir, false)
	if err != nil {
		return nil, err
	}

	var bugs []Feature
	for _, f := range files {
		if _, err := MigrateLegacyBugIDs(f); err != nil {
			return nil, err
		}
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
		bugs = append(bugs, Feature{
			File:    f,
			Ignored: ignored,
			Tasks:   tasks,
		})
	}

	return bugs, nil
}

// MarkCompletedBugs renames bug files where all tasks are complete (and none are blocked)
// by appending _completed before the .md extension (e.g. bug_001.md → bug_001_completed.md).
func MarkCompletedBugs(dir string) error {
	files, err := GlobBugFiles(dir, false)
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

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

const progressBarWidth = 10

type featureInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
	ignored   bool // filename contains _ignored
	isBug     bool // true for bug files (from .maggus/bugs/)
}

func (f *featureInfo) doneCount() int {
	n := 0
	for _, t := range f.tasks {
		if t.IsComplete() {
			n++
		}
	}
	return n
}

func (f *featureInfo) blockedCount() int {
	n := 0
	for _, t := range f.tasks {
		if !t.IsComplete() && t.IsBlocked() {
			n++
		}
	}
	return n
}

func buildProgressBar(done, total int) string {
	return styles.ProgressBar(done, total, progressBarWidth)
}

func buildProgressBarPlain(done, total int) string {
	return styles.ProgressBarPlain(done, total, progressBarWidth)
}

// buildSelectableTasksForFeature returns the flat list of tasks for a single feature.
// When showAll is false, completed tasks are excluded.
func buildSelectableTasksForFeature(feature featureInfo, showAll bool) []parser.Task {
	var selectable []parser.Task
	for _, t := range feature.tasks {
		if !showAll && t.IsComplete() {
			continue
		}
		selectable = append(selectable, t)
	}
	return selectable
}

func parseFeatures(dir string) ([]featureInfo, error) {
	files, err := parser.GlobFeatureFiles(dir, true)
	if err != nil {
		return nil, fmt.Errorf("glob features: %w", err)
	}

	var features []featureInfo
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		ignored := parser.IsIgnoredFile(f)
		if ignored {
			for i := range tasks {
				tasks[i].Ignored = true
			}
		}
		features = append(features, featureInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
			ignored:   ignored,
		})
	}
	return features, nil
}

func parseBugs(dir string) ([]featureInfo, error) {
	files, err := parser.GlobBugFiles(dir, true)
	if err != nil {
		return nil, fmt.Errorf("glob bugs: %w", err)
	}

	var bugs []featureInfo
	for _, f := range files {
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		ignored := parser.IsIgnoredFile(f)
		if ignored {
			for i := range tasks {
				tasks[i].Ignored = true
			}
		}
		bugs = append(bugs, featureInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
			ignored:   ignored,
			isBug:     true,
		})
	}
	return bugs, nil
}

func findNextTask(features []featureInfo) (string, string) {
	// Bugs first, then features
	for _, f := range features {
		if f.completed || !f.isBug {
			continue
		}
		next := parser.FindNextIncomplete(f.tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	for _, f := range features {
		if f.completed || f.isBug {
			continue
		}
		next := parser.FindNextIncomplete(f.tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

// renderStatusPlain builds the plain-text status output (no ANSI, no TUI).
func renderStatusPlain(w *strings.Builder, features []featureInfo, showAll bool, nextTaskID, nextTaskFile, agentName string) {
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, f := range features {
		totalTasks += len(f.tasks)
		totalDone += f.doneCount()
		totalBlocked += f.blockedCount()
		if f.isBug {
			totalBugs++
			if !f.completed {
				activeBugs++
			}
		} else {
			if !f.completed {
				activeFeatures++
			}
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked
	featureCount := len(features) - totalBugs

	headerParts := fmt.Sprintf("%d features (%d active)", featureCount, activeFeatures)
	if totalBugs > 0 {
		headerParts += fmt.Sprintf(", %d bugs (%d active)", totalBugs, activeBugs)
	}
	fmt.Fprintf(w, "Maggus Status — %s, %d tasks total\n\n", headerParts, totalTasks)
	fmt.Fprintf(w, " Summary: %d/%d tasks complete · %d pending · %d blocked\n", totalDone, totalTasks, totalPending, totalBlocked)
	fmt.Fprintf(w, " Agent: %s\n", agentName)

	for _, f := range features {
		if f.completed && !showAll {
			continue
		}
		fmt.Fprintln(w)
		if f.completed {
			fmt.Fprintf(w, " Tasks — %s (archived)\n", f.filename)
		} else if f.ignored {
			fmt.Fprintf(w, " Tasks — [~] %s (ignored)\n", f.filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", f.filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range f.tasks {
			var icon, prefix string

			if t.Ignored {
				icon = "[~]"
				prefix = "  "
			} else if t.IsComplete() {
				icon = "[x]"
				prefix = "  "
			} else if t.IsBlocked() {
				icon = "[!]"
				prefix = "  "
			} else if t.ID == nextTaskID && t.SourceFile == nextTaskFile {
				icon = "o"
				prefix = "-> "
			} else {
				icon = "o"
				prefix = "  "
			}

			fmt.Fprintf(w, " %s%s  %s: %s\n", prefix, icon, t.ID, t.Title)

			if t.IsBlocked() && !f.completed {
				for _, c := range t.Criteria {
					if !c.Blocked {
						continue
					}
					reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
					reason = strings.TrimPrefix(reason, "BLOCKED: ")
					fmt.Fprintf(w, "         BLOCKED: %s\n", reason)
				}
			}
		}
	}

	// Features table
	fmt.Fprintln(w)
	fmt.Fprintln(w, " Features")
	fmt.Fprintln(w, " ──────────────────────────────────────────")

	maxCountWidth := 0
	for _, f := range features {
		if f.completed && !showAll {
			continue
		}
		cw := len(fmt.Sprintf("%d/%d", f.doneCount(), len(f.tasks)))
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, f := range features {
		if f.completed && !showAll {
			continue
		}

		done := f.doneCount()
		total := len(f.tasks)
		bar := buildProgressBarPlain(done, total)

		var prefix, suffix string

		if f.completed {
			prefix = " [x] "
			suffix = "done"
		} else if f.ignored {
			prefix = " [~] "
			suffix = "ignored"
		} else if f.blockedCount() > 0 {
			prefix = "   "
			suffix = "blocked"
		} else if total > 0 && done == total {
			prefix = "   "
			suffix = "done"
		} else if done == 0 {
			prefix = "   "
			suffix = "new"
		} else {
			prefix = "   "
			suffix = "in progress"
		}

		countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
		fmt.Fprintf(w, "%s%-32s [%s]  %s   %s\n", prefix, f.filename, bar, countStr, suffix)
	}
}

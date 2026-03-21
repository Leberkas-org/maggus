package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

const progressBarWidth = 10

type planInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
	ignored   bool // filename contains _ignored
}

func (p *planInfo) doneCount() int {
	n := 0
	for _, t := range p.tasks {
		if t.IsComplete() {
			n++
		}
	}
	return n
}

func (p *planInfo) blockedCount() int {
	n := 0
	for _, t := range p.tasks {
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

// buildSelectableTasksForPlan returns the flat list of tasks for a single plan.
// When showAll is false, completed tasks are excluded.
func buildSelectableTasksForPlan(plan planInfo, showAll bool) []parser.Task {
	var selectable []parser.Task
	for _, t := range plan.tasks {
		if !showAll && t.IsComplete() {
			continue
		}
		selectable = append(selectable, t)
	}
	return selectable
}

func parsePlans(dir string) ([]planInfo, error) {
	files, err := parser.GlobPlanFiles(dir, true)
	if err != nil {
		return nil, fmt.Errorf("glob plans: %w", err)
	}

	var plans []planInfo
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
		plans = append(plans, planInfo{
			filename:  filepath.Base(f),
			tasks:     tasks,
			completed: strings.HasSuffix(f, "_completed.md"),
			ignored:   ignored,
		})
	}
	return plans, nil
}

func findNextTask(plans []planInfo) (string, string) {
	for _, p := range plans {
		if p.completed {
			continue
		}
		next := parser.FindNextIncomplete(p.tasks)
		if next != nil {
			return next.ID, next.SourceFile
		}
	}
	return "", ""
}

// renderStatusPlain builds the plain-text status output (no ANSI, no TUI).
func renderStatusPlain(w *strings.Builder, plans []planInfo, showAll bool, nextTaskID, nextTaskFile, agentName string) {
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activePlans := 0
	for _, p := range plans {
		totalTasks += len(p.tasks)
		totalDone += p.doneCount()
		totalBlocked += p.blockedCount()
		if !p.completed {
			activePlans++
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked

	fmt.Fprintf(w, "Maggus Status — %d plans (%d active), %d tasks total\n\n", len(plans), activePlans, totalTasks)
	fmt.Fprintf(w, " Summary: %d/%d tasks complete · %d pending · %d blocked\n", totalDone, totalTasks, totalPending, totalBlocked)
	fmt.Fprintf(w, " Agent: %s\n", agentName)

	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		fmt.Fprintln(w)
		if p.completed {
			fmt.Fprintf(w, " Tasks — %s (archived)\n", p.filename)
		} else if p.ignored {
			fmt.Fprintf(w, " Tasks — [~] %s (ignored)\n", p.filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", p.filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range p.tasks {
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

			if t.IsBlocked() && !p.completed {
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

	// Plans table
	fmt.Fprintln(w)
	fmt.Fprintln(w, " Plans")
	fmt.Fprintln(w, " ──────────────────────────────────────────")

	maxCountWidth := 0
	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}
		cw := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range plans {
		if p.completed && !showAll {
			continue
		}

		done := p.doneCount()
		total := len(p.tasks)
		bar := buildProgressBarPlain(done, total)

		var prefix, suffix string

		if p.completed {
			prefix = " [x] "
			suffix = "done"
		} else if p.ignored {
			prefix = " [~] "
			suffix = "ignored"
		} else if p.blockedCount() > 0 {
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
		fmt.Fprintf(w, "%s%-32s [%s]  %s   %s\n", prefix, p.filename, bar, countStr, suffix)
	}
}

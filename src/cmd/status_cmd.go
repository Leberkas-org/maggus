package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/parser"
)

// renderStatusPlain builds the plain-text status output (no ANSI, no TUI).
func renderStatusPlain(w *strings.Builder, plans []parser.Plan, showAll bool, nextTaskID, nextTaskFile, agentName string, approvals approval.Approvals, approvalRequired bool) {
	totalTasks := 0
	totalDone := 0
	totalBlocked := 0
	activeFeatures := 0
	totalBugs := 0
	activeBugs := 0
	for _, p := range plans {
		totalTasks += len(p.Tasks)
		totalDone += p.DoneCount()
		totalBlocked += p.BlockedCount()
		if p.IsBug {
			totalBugs++
			if !p.Completed {
				activeBugs++
			}
		} else {
			if !p.Completed {
				activeFeatures++
			}
		}
	}
	totalPending := totalTasks - totalDone - totalBlocked
	featureCount := len(plans) - totalBugs

	headerParts := fmt.Sprintf("%d features (%d active)", featureCount, activeFeatures)
	if totalBugs > 0 {
		headerParts += fmt.Sprintf(", %d bugs (%d active)", totalBugs, activeBugs)
	}
	fmt.Fprintf(w, "Maggus Status — %s, %d tasks total\n\n", headerParts, totalTasks)
	fmt.Fprintf(w, " Summary: %d/%d tasks complete · %d pending · %d blocked\n", totalDone, totalTasks, totalPending, totalBlocked)
	fmt.Fprintf(w, " Agent: %s\n", agentName)

	for _, p := range plans {
		if p.Completed && !showAll {
			continue
		}
		filename := filepath.Base(p.File)
		approved := isPlanApproved(p, approvals, approvalRequired)
		fmt.Fprintln(w)
		if p.Completed {
			fmt.Fprintf(w, " Tasks — %s (archived)\n", filename)
		} else if !approved {
			fmt.Fprintf(w, " Tasks — [✗] %s (unapproved)\n", filename)
		} else {
			fmt.Fprintf(w, " Tasks — %s\n", filename)
		}
		fmt.Fprintln(w, " ──────────────────────────────────────────")

		for _, t := range p.Tasks {
			var icon, prefix string

			if t.IsComplete() {
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

			if t.IsBlocked() && !p.Completed {
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
	for _, p := range plans {
		if p.Completed && !showAll {
			continue
		}
		cw := len(fmt.Sprintf("%d/%d", p.DoneCount(), len(p.Tasks)))
		if cw > maxCountWidth {
			maxCountWidth = cw
		}
	}
	countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

	for _, p := range plans {
		if p.Completed && !showAll {
			continue
		}

		filename := filepath.Base(p.File)
		approved := isPlanApproved(p, approvals, approvalRequired)
		done := p.DoneCount()
		total := len(p.Tasks)
		bar := buildProgressBarPlain(done, total)

		var prefix, suffix string

		if p.Completed {
			prefix = " [x] "
			suffix = "done"
		} else if !approved {
			prefix = " [✗] "
			suffix = "unapproved"
		} else if p.BlockedCount() > 0 {
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
		fmt.Fprintf(w, "%s%-32s [%s]  %s   %s\n", prefix, filename, bar, countStr, suffix)
	}
}

func runStatus() error {
	sm, err := buildStatusModel()
	if err != nil {
		return err
	}
	app := appModel{
		active: screenStatus,
		status: sm,
	}
	prog := tea.NewProgram(app, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}


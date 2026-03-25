package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/config"
	"github.com/leberkas-org/maggus/internal/filewatcher"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/spf13/cobra"
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of feature progress",
	Long:  `Reads all feature files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}
		showLog, err := cmd.Flags().GetBool("show-log")
		if err != nil {
			return err
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		cfg, err := config.Load(dir)
		if err != nil {
			return err
		}
		agentName := cfg.Agent
		approvalRequired := cfg.IsApprovalRequired()

		features, approvals, err := loadPlansWithApprovals(dir, true)
		if err != nil {
			return err
		}
		pruneStaleApprovals(dir, features)

		if len(features) == 0 {
			if plain {
				fmt.Fprintln(cmd.OutOrStdout(), "No features found.")
				return nil
			}
			// TUI mode: show empty status view
			features = []parser.Plan{}
		}

		nextTaskID, nextTaskFile := findNextTask(features)

		if plain {
			var sb strings.Builder
			renderStatusPlain(&sb, features, all, nextTaskID, nextTaskFile, agentName, approvals, approvalRequired)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		}

		// TUI mode: interactive status with detail view
		watcherCh := make(chan bool, 1)
		w, _ := filewatcher.New(dir, func(msg any) {
			hasNew := false
			if u, ok := msg.(filewatcher.UpdateMsg); ok {
				hasNew = u.HasNewFile
			}
			select {
			case watcherCh <- hasNew:
			default: // don't block if channel already has a pending update
			}
		}, 300*time.Millisecond)

		m := newStatusModel(features, all, nextTaskID, nextTaskFile, agentName, dir, showLog, approvalRequired)
		m.presence = sharedPresence
		m.watcherCh = watcherCh
		m.watcher = w
		prog := tea.NewProgram(m, tea.WithAltScreen())
		result, err := prog.Run()
		if w != nil {
			w.Close()
		}
		if err != nil {
			return err
		}
		if final, ok := result.(statusModel); ok && final.RunTaskID != "" {
			return dispatchWork(final.RunTaskID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed features in task sections and Features table")
	statusCmd.Flags().Bool("show-log", false, "Open the live log panel immediately on startup")
}


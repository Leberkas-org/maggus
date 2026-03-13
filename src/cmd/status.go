package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/parser"
	"github.com/leberkas-org/maggus/internal/tui/styles"

	"github.com/spf13/cobra"
)

const progressBarWidth = 10

// Lipgloss styles for the status command.
var (
	statusGreenStyle  = lipgloss.NewStyle().Foreground(styles.Success)
	statusCyanStyle   = lipgloss.NewStyle().Foreground(styles.Primary)
	statusYellowStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	statusRedStyle    = lipgloss.NewStyle().Foreground(styles.Error)
	statusBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	statusDimStyle    = lipgloss.NewStyle().Faint(true)
	statusDimGreen    = lipgloss.NewStyle().Faint(true).Foreground(styles.Success)
)

type planInfo struct {
	filename  string
	tasks     []parser.Task
	completed bool // filename contains _completed
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of plan progress",
	Long:  `Reads all plan files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		// Helper: applies a lipgloss style only when not in plain mode.
		render := func(s lipgloss.Style, text string) string {
			if plain {
				return text
			}
			return s.Render(text)
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		files, err := parser.GlobPlanFiles(dir, true)
		if err != nil {
			return fmt.Errorf("glob plans: %w", err)
		}

		if len(files) == 0 {
			fmt.Println("No plans found.")
			return nil
		}

		// Parse each plan file
		var plans []planInfo
		for _, f := range files {
			tasks, err := parser.ParseFile(f)
			if err != nil {
				return fmt.Errorf("parse %s: %w", f, err)
			}
			plans = append(plans, planInfo{
				filename:  filepath.Base(f),
				tasks:     tasks,
				completed: strings.HasSuffix(f, "_completed.md"),
			})
		}

		// Compute totals across all plans
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

		// Header
		fmt.Printf("Maggus Status — %d plans (%d active), %d tasks total\n\n",
			len(plans), activePlans, totalTasks)

		// Summary
		fmt.Printf(" Summary: %d/%d tasks complete · %d pending · %d blocked\n",
			totalDone, totalTasks, totalPending, totalBlocked)

		// Find the global next workable task (first across all active plans)
		var nextTaskID, nextTaskFile string
		for _, p := range plans {
			if p.completed {
				continue
			}
			next := parser.FindNextIncomplete(p.tasks)
			if next != nil {
				nextTaskID = next.ID
				nextTaskFile = next.SourceFile
				break
			}
		}

		// Detailed task list section
		for _, p := range plans {
			if p.completed && !all {
				continue
			}
			fmt.Println()
			if p.completed {
				fmt.Println(render(statusDimGreen, fmt.Sprintf(" Tasks — %s (archived)", p.filename)))
			} else {
				fmt.Printf(" Tasks — %s\n", p.filename)
			}
			fmt.Println(" ──────────────────────────────────────────")

			for _, t := range p.tasks {
				var icon, prefix string
				var style lipgloss.Style

				if t.IsComplete() {
					if plain {
						icon = "[x]"
					} else {
						icon = "✓"
					}
					if p.completed {
						style = statusDimGreen
					} else {
						style = statusGreenStyle
					}
					prefix = "  "
				} else if t.IsBlocked() {
					if plain {
						icon = "[!]"
					} else {
						icon = "⚠"
					}
					style = statusRedStyle
					prefix = "  "
				} else if t.ID == nextTaskID && t.SourceFile == nextTaskFile {
					icon = "o"
					style = statusCyanStyle
					if plain {
						prefix = "-> "
					} else {
						prefix = "→ "
					}
				} else {
					icon = "o"
					style = lipgloss.NewStyle()
					prefix = "  "
				}

				if p.completed {
					style = statusDimStyle
				}

				line := fmt.Sprintf(" %s%s  %s: %s", prefix, icon, t.ID, t.Title)
				fmt.Println(render(style, line))

				if t.IsBlocked() && !p.completed {
					for _, c := range t.Criteria {
						if !c.Blocked {
							continue
						}
						reason := strings.TrimPrefix(c.Text, "⚠️ BLOCKED: ")
						reason = strings.TrimPrefix(reason, "BLOCKED: ")
						blockedLine := fmt.Sprintf("         BLOCKED: %s", reason)
						fmt.Println(render(statusRedStyle, blockedLine))
					}
				}
			}
		}

		// Plans table at the bottom
		fmt.Println()
		fmt.Println(" Plans")
		fmt.Println(" ──────────────────────────────────────────")

		// Find max width of "done/total" strings for aligned columns.
		maxCountWidth := 0
		for _, p := range plans {
			if p.completed && !all {
				continue
			}
			w := len(fmt.Sprintf("%d/%d", p.doneCount(), len(p.tasks)))
			if w > maxCountWidth {
				maxCountWidth = w
			}
		}
		countFmt := fmt.Sprintf("%%-%ds", maxCountWidth)

		for _, p := range plans {
			if p.completed && !all {
				continue
			}

			done := p.doneCount()
			total := len(p.tasks)

			var bar string
			if plain {
				bar = buildProgressBarPlain(done, total)
			} else {
				bar = buildProgressBar(done, total)
			}

			var prefix, suffix string
			var style lipgloss.Style

			if p.completed {
				if plain {
					prefix = " [x] "
				} else {
					prefix = " ✓ "
				}
				style = statusDimGreen
				suffix = "done"
			} else if p.blockedCount() > 0 {
				prefix = "   "
				style = statusRedStyle
				suffix = "blocked"
			} else if total > 0 && done == total {
				prefix = "   "
				style = statusGreenStyle
				suffix = "done"
			} else if done == 0 {
				prefix = "   "
				style = statusBlueStyle
				suffix = "new"
			} else {
				prefix = "   "
				style = statusYellowStyle
				suffix = "in progress"
			}

			countStr := fmt.Sprintf(countFmt, fmt.Sprintf("%d/%d", done, total))
			line := fmt.Sprintf("%s%-32s [%s]  %s   %s", prefix, p.filename, bar, countStr, suffix)
			fmt.Println(render(style, line))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	statusCmd.Flags().Bool("all", false, "Show completed plans in task sections and Plans table")
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dirnei/maggus/internal/parser"
	"github.com/spf13/cobra"
)

const (
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"

	progressBarWidth = 10
	progressBarFull  = '█'
	progressBarEmpty = '░'
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
	if total == 0 {
		return strings.Repeat(string(progressBarEmpty), progressBarWidth)
	}
	filled := (done * progressBarWidth) / total
	return strings.Repeat(string(progressBarFull), filled) +
		strings.Repeat(string(progressBarEmpty), progressBarWidth-filled)
}

func buildProgressBarPlain(done, total int) string {
	if total == 0 {
		return strings.Repeat(".", progressBarWidth)
	}
	filled := (done * progressBarWidth) / total
	return strings.Repeat("#", filled) +
		strings.Repeat(".", progressBarWidth-filled)
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

		// Helper: returns color codes only when not in plain mode
		color := func(code string) string {
			if plain {
				return ""
			}
			return code
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		maggusDir := filepath.Join(dir, ".maggus")
		if _, err := os.Stat(maggusDir); os.IsNotExist(err) {
			fmt.Println("No plans found.")
			return nil
		}

		// Find all plan files including completed ones
		pattern := filepath.Join(maggusDir, "plan_*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob plans: %w", err)
		}
		sort.Strings(files)

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
		fmt.Println(" Plans")
		fmt.Println(" ──────────────────────────────────────────")

		for _, p := range plans {
			done := p.doneCount()
			total := len(p.tasks)

			var bar string
			if plain {
				bar = buildProgressBarPlain(done, total)
			} else {
				bar = buildProgressBar(done, total)
			}

			var prefix, clr, suffix string

			if p.completed {
				if plain {
					prefix = " [x] "
				} else {
					prefix = " ✓ "
				}
				clr = color(colorDim + colorGreen)
				suffix = "done"
			} else if p.blockedCount() > 0 {
				prefix = "   "
				clr = color(colorRed)
				suffix = "blocked"
			} else if total > 0 && done == total {
				prefix = "   "
				clr = color(colorGreen)
				suffix = "done"
			} else {
				prefix = "   "
				clr = color(colorYellow)
				suffix = "in progress"
			}

			fmt.Printf("%s%s%-32s [%s]  %d/%d   %s%s\n",
				clr, prefix, p.filename, bar, done, total, suffix, color(colorReset))
		}

		fmt.Println()
		fmt.Printf(" Summary: %d/%d tasks complete · %d pending · %d blocked\n",
			totalDone, totalTasks, totalPending, totalBlocked)

		// Find the global next workable task (first across all active plans)
		var nextTaskID string
		for _, p := range plans {
			if p.completed {
				continue
			}
			next := parser.FindNextIncomplete(p.tasks)
			if next != nil {
				nextTaskID = next.ID
				break
			}
		}

		// Detailed task list section
		for _, p := range plans {
			fmt.Println()
			if p.completed {
				fmt.Printf("%s%s Tasks — %s (archived)%s\n", color(colorDim), color(colorGreen), p.filename, color(colorReset))
			} else {
				fmt.Printf(" Tasks — %s\n", p.filename)
			}
			fmt.Println(" ──────────────────────────────────────────")

			for _, t := range p.tasks {
				var icon, clr, prefix string

				if t.IsComplete() {
					if plain {
						icon = "[x]"
					} else {
						icon = "✓"
					}
					if p.completed {
						clr = color(colorDim + colorGreen)
					} else {
						clr = color(colorGreen)
					}
					prefix = "  "
				} else if t.IsBlocked() {
					if plain {
						icon = "[!]"
					} else {
						icon = "⚠"
					}
					clr = color(colorRed)
					prefix = "  "
				} else if t.ID == nextTaskID {
					icon = "o"
					clr = color(colorCyan)
					if plain {
						prefix = "-> "
					} else {
						prefix = "→ "
					}
				} else {
					icon = "o"
					clr = ""
					prefix = "  "
				}

				if p.completed {
					clr = color(colorDim)
				}

				fmt.Printf(" %s%s%s  %s: %s%s\n", clr, prefix, icon, t.ID, t.Title, color(colorReset))
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
}

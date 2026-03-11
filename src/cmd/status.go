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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact summary of plan progress",
	Long:  `Reads all plan files in .maggus/ and displays a compact progress summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			bar := buildProgressBar(done, total)

			var prefix, color, suffix string

			if p.completed {
				prefix = " ✓ "
				color = colorDim + colorGreen
				suffix = "done"
			} else if p.blockedCount() > 0 {
				prefix = "   "
				color = colorRed
				suffix = "blocked"
			} else if total > 0 && done == total {
				prefix = "   "
				color = colorGreen
				suffix = "done"
			} else {
				prefix = "   "
				color = colorYellow
				suffix = "in progress"
			}

			fmt.Printf("%s%s%-32s [%s]  %d/%d   %s%s\n",
				color, prefix, p.filename, bar, done, total, suffix, colorReset)
		}

		fmt.Println()
		fmt.Printf(" Summary: %d/%d tasks complete · %d pending · %d blocked\n",
			totalDone, totalTasks, totalPending, totalBlocked)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

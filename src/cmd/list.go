package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dirnei/maggus/internal/parser"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [N]",
	Short: "Preview the next N upcoming workable tasks",
	Long:  `Reads all plan files in .maggus/ and lists the next N workable (incomplete, not blocked) tasks.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plain, err := cmd.Flags().GetBool("plain")
		if err != nil {
			return err
		}

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}

		// Positional arg overrides --count
		if len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid count %q: must be a positive integer", args[0])
			}
			count = n
		}

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
			fmt.Println("No pending tasks found. All done!")
			return nil
		}

		pattern := filepath.Join(maggusDir, "plan_*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob plans: %w", err)
		}
		sort.Strings(files)

		// Collect workable tasks in order, skipping completed plans
		var workable []parser.Task
		for _, f := range files {
			if strings.HasSuffix(f, "_completed.md") {
				continue
			}
			tasks, err := parser.ParseFile(f)
			if err != nil {
				return fmt.Errorf("parse %s: %w", f, err)
			}
			for _, t := range tasks {
				if t.IsWorkable() {
					workable = append(workable, t)
				}
			}
		}

		if len(workable) == 0 {
			fmt.Println("No pending tasks found. All done!")
			return nil
		}

		// Cap to count
		if count < len(workable) {
			workable = workable[:count]
		}

		fmt.Printf("Next %d task(s):\n", len(workable))
		fmt.Println()

		for i, t := range workable {
			clr := ""
			if i == 0 {
				clr = color(colorCyan)
			}

			fmt.Printf(" %s#%-2d %s: %s%s\n", clr, i+1, t.ID, t.Title, color(colorReset))

			// First line of description, truncated to 80 chars
			desc := firstLine(t.Description)
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			if desc != "" {
				fmt.Printf("     %s%s%s\n", color(colorDim), desc, color(colorReset))
			}
			fmt.Println()
		}

		return nil
	},
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
}

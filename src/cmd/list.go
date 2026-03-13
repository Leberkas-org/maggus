package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/leberkas-org/maggus/internal/parser"
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

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}

		// Positional arg overrides --count (ignored when --all is set)
		if !all && len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid count %q: must be a positive integer", args[0])
			}
			count = n
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		return runList(cmd, dir, plain, all, count)
	},
}

func runList(cmd *cobra.Command, dir string, plain, all bool, count int) error {
	out := cmd.OutOrStdout()

	color := func(code string) string {
		if plain {
			return ""
		}
		return code
	}

	files, err := parser.GlobPlanFiles(dir, false)
	if err != nil {
		return fmt.Errorf("glob plans: %w", err)
	}

	// Collect workable tasks in order
	var workable []parser.Task
	for _, f := range files {
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
		fmt.Fprintln(out, "No pending tasks found. All done!")
		return nil
	}

	// Cap to count unless --all is set
	if !all && count < len(workable) {
		workable = workable[:count]
	}

	if all {
		fmt.Fprintln(out, "All upcoming tasks:")
	} else {
		fmt.Fprintf(out, "Next %d task(s):\n", len(workable))
	}
	fmt.Fprintln(out)

	for i, t := range workable {
		clr := ""
		if i == 0 {
			clr = color(colorCyan)
		}
		planFile := color(colorDim) + filepath.Base(t.SourceFile) + color(colorReset)
		fmt.Fprintf(out, " %s#%-2d %s: %s%s  %s\n", clr, i+1, t.ID, t.Title, color(colorReset), planFile)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntP("count", "c", 5, "Number of tasks to show")
	listCmd.Flags().Bool("plain", false, "Strip colors and use ASCII characters for scripting/piping")
	listCmd.Flags().Bool("all", false, "Show all upcoming workable tasks with no count cap")
}

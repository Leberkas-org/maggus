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

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Interactive wizard to manage blocked tasks",
	Long: `Walks through each blocked task in your plan files and lets you
unblock, resolve, or skip each blocked criterion interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runBlocked(cmd, dir)
	},
}

func runBlocked(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()

	maggusDir := filepath.Join(dir, ".maggus")
	if _, err := os.Stat(maggusDir); os.IsNotExist(err) {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	pattern := filepath.Join(maggusDir, "plan_*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob plans: %w", err)
	}
	sort.Strings(files)

	var blocked []parser.Task
	for _, f := range files {
		if strings.HasSuffix(f, "_completed.md") {
			continue
		}
		tasks, err := parser.ParseFile(f)
		if err != nil {
			return fmt.Errorf("parse %s: %w", f, err)
		}
		for _, t := range tasks {
			if t.IsBlocked() {
				blocked = append(blocked, t)
			}
		}
	}

	if len(blocked) == 0 {
		fmt.Fprintln(out, "No blocked tasks found.")
		return nil
	}

	fmt.Fprintf(out, "Found %d blocked task(s).\n", len(blocked))
	return nil
}

func init() {
	rootCmd.AddCommand(blockedCmd)
}

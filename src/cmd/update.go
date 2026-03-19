package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/leberkas-org/maggus/internal/updater"
	"github.com/spf13/cobra"
)

// checkLatestVersion is a package-level var so tests can replace it.
var checkLatestVersion = func(currentVersion string) updater.UpdateInfo {
	return updater.CheckLatestVersion(currentVersion)
}

// applyUpdate is a package-level var so tests can replace it.
var applyUpdate = func(downloadURL string) error {
	return updater.Apply(downloadURL)
}

// promptConfirm is a package-level var so tests can replace it.
var promptConfirm = func(question string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long: `Checks GitHub Releases for a newer version of maggus and offers to install it.

Examples:
  maggus update`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd)
	},
}

func runUpdate(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	currentVersion := Version

	fmt.Fprintln(out, "Checking for updates...")

	info := checkLatestVersion(currentVersion)

	if !info.IsNewer {
		fmt.Fprintf(out, "Already up to date (v%s)\n", strings.TrimPrefix(currentVersion, "v"))
		return nil
	}

	// Show current vs latest — "dev" is displayed as-is, tagged versions get "v" prefix normalization
	currentDisplay := currentVersion
	if currentVersion != "dev" {
		currentDisplay = "v" + strings.TrimPrefix(currentVersion, "v")
	}
	fmt.Fprintf(out, "Update available: %s → %s\n",
		currentDisplay,
		info.TagName)

	// Show changelog summary if available
	if info.Body != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Changelog:")
		fmt.Fprintln(out, info.Body)
		fmt.Fprintln(out)
	}

	// Ask for confirmation
	if !promptConfirm("Install update?") {
		fmt.Fprintln(out, "Update cancelled.")
		return nil
	}

	// Apply the update
	if info.DownloadURL == "" {
		return fmt.Errorf("no download available for your platform")
	}

	fmt.Fprintln(out, "Downloading and installing...")

	if err := applyUpdate(info.DownloadURL); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Fprintf(out, "Successfully updated to %s! Please restart maggus.\n", info.TagName)
	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

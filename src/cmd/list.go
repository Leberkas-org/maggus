package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/leberkas-org/maggus/internal/stores"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all active features and bugs as tab-separated lines",
	Long:         `Print each active (non-completed) feature and bug as a tab-separated line: filename, id, title, approved.`,
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		featureStore := stores.NewFileFeatureStore(dir)
		bugStore := stores.NewFileBugStore(dir)
		return runList(cmd, dir, featureStore, bugStore)
	},
}

func runList(cmd *cobra.Command, dir string, featureStore stores.FeatureStore, bugStore stores.BugStore) error {
	plans, err := loadAllPlans(featureStore, bugStore)
	if err != nil {
		return err
	}

	if len(plans) == 0 {
		return nil
	}

	a, err := approval.Load(dir)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	for _, p := range plans {
		approvedStr := "unapproved"
		if a[p.ApprovalKey()] {
			approvedStr = "approved"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", filepath.Base(p.File), p.ID, p.Title, approvedStr)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
}

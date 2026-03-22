package cmd

// dispatchWork runs `maggus work --task <id>` by invoking the work subcommand.
func dispatchWork(taskID string) error {
	sub, remaining, err := rootCmd.Find([]string{"work", "--task", taskID})
	if err != nil {
		return err
	}
	// Reset work command flags so previous invocations don't leak.
	resetWorkFlags()
	if err := sub.ParseFlags(remaining); err != nil {
		return err
	}
	return sub.RunE(sub, sub.Flags().Args())
}

package cmd

// dispatchWork runs `maggus run --task <id>` by invoking the run subcommand.
func dispatchWork(taskID string) error {
	sub, remaining, err := rootCmd.Find([]string{"run", "--task", taskID})
	if err != nil {
		return err
	}
	// Reset work command flags so previous invocations don't leak.
	resetRunFlags()
	if err := sub.ParseFlags(remaining); err != nil {
		return err
	}
	return sub.RunE(sub, sub.Flags().Args())
}

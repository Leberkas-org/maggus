package cmd

// dispatchWork runs `maggus work --count 1` by invoking the work subcommand.
func dispatchWork() error {
	sub, remaining, err := rootCmd.Find([]string{"work", "--count", "1"})
	if err != nil {
		return err
	}
	if err := sub.ParseFlags(remaining); err != nil {
		return err
	}
	return sub.RunE(sub, sub.Flags().Args())
}

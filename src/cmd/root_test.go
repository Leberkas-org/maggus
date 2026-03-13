package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_HelpOutput(t *testing.T) {
	// Verify that root command still produces help output when RunE is set.
	// This ensures non-interactive usage (pipes, CI, --help) works correctly.
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("expected help output to contain 'Usage:', got:\n%s", output)
	}
	if !strings.Contains(output, "maggus") {
		t.Errorf("expected help output to contain 'maggus', got:\n%s", output)
	}

	// Reset args to avoid polluting other tests
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
}

func TestRootCmd_RunE_IsSet(t *testing.T) {
	// Verify that RunE is set on the root command (required for menu/fallback behavior).
	if rootCmd.RunE == nil {
		t.Error("expected rootCmd.RunE to be set")
	}
}

package cmd

import (
	"bytes"
	"os"
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

func TestShouldSkipResolver(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"start skips resolver", []string{"maggus", "start"}, true},
		{"stop skips resolver", []string{"maggus", "stop"}, true},
		{"start --all skips resolver", []string{"maggus", "start", "--all"}, true},
		{"stop --all skips resolver", []string{"maggus", "stop", "--all"}, true},
		{"work does not skip", []string{"maggus", "work"}, false},
		{"list does not skip", []string{"maggus", "list"}, false},
		{"status does not skip", []string{"maggus", "status"}, false},
		{"no args does not skip", []string{"maggus"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			defer func() { os.Args = origArgs }()
			os.Args = tt.args
			if got := shouldSkipResolver(); got != tt.want {
				t.Errorf("shouldSkipResolver() = %v, want %v", got, tt.want)
			}
		})
	}
}

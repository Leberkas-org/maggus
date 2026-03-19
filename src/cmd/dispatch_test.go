package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestDispatchWork_FindsWorkSubcommand(t *testing.T) {
	// Save the original RunE so we can restore it after the test.
	origRunE := workCmd.RunE
	defer func() { workCmd.RunE = origRunE }()

	var called bool
	workCmd.RunE = func(cmd *cobra.Command, args []string) error {
		called = true
		return nil
	}

	if err := dispatchWork("TASK-042"); err != nil {
		t.Fatalf("dispatchWork returned unexpected error: %v", err)
	}
	if !called {
		t.Error("expected work subcommand RunE to be called")
	}
}

func TestDispatchWork_TaskFlagIsSet(t *testing.T) {
	origRunE := workCmd.RunE
	origTaskFlag := taskFlag
	defer func() {
		workCmd.RunE = origRunE
		taskFlag = origTaskFlag
	}()

	var capturedTaskFlag string
	workCmd.RunE = func(cmd *cobra.Command, args []string) error {
		capturedTaskFlag = taskFlag
		return nil
	}

	if err := dispatchWork("TASK-123"); err != nil {
		t.Fatalf("dispatchWork returned unexpected error: %v", err)
	}
	if capturedTaskFlag != "TASK-123" {
		t.Errorf("expected taskFlag to be %q, got %q", "TASK-123", capturedTaskFlag)
	}
}

func TestDispatchWork_PropagatesRunError(t *testing.T) {
	origRunE := workCmd.RunE
	defer func() { workCmd.RunE = origRunE }()

	sentinel := errors.New("work failed")
	workCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return sentinel
	}

	err := dispatchWork("TASK-001")
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

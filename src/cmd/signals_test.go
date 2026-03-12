package cmd

import (
	"os"
	"runtime"
	"testing"
)

func TestShutdownSignalsNotEmpty(t *testing.T) {
	if len(shutdownSignals) == 0 {
		t.Fatal("shutdownSignals must contain at least one signal")
	}
}

func TestShutdownSignalsContainsInterrupt(t *testing.T) {
	found := false
	for _, sig := range shutdownSignals {
		if sig == os.Interrupt {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("shutdownSignals must contain os.Interrupt")
	}
}

func TestShutdownSignalsPlatformSpecific(t *testing.T) {
	if runtime.GOOS == "windows" {
		if len(shutdownSignals) != 1 {
			t.Fatalf("Windows should have exactly 1 signal, got %d", len(shutdownSignals))
		}
	} else {
		if len(shutdownSignals) < 2 {
			t.Fatalf("Unix should have at least 2 signals (SIGINT+SIGTERM), got %d", len(shutdownSignals))
		}
	}
}

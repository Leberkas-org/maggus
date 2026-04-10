package cmd

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/globalconfig"
)

func TestAutoStartDaemon_LoadFails_Noop(t *testing.T) {
	loadErr := errors.New("cannot load global config")
	startCalled := false

	err := autoStartDaemonUsing(
		t.TempDir(),
		func() (globalconfig.GlobalConfig, error) {
			return globalconfig.GlobalConfig{}, loadErr
		},
		func(string) error {
			startCalled = true
			return nil
		},
	)

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if startCalled {
		t.Error("startDaemon should not be called when globalconfig.Load fails")
	}
}

func TestAutoStartDaemon_RepoNotFound_Noop(t *testing.T) {
	dir := t.TempDir()
	startCalled := false
	cfg := globalconfig.GlobalConfig{
		Repositories: []globalconfig.Repository{
			{Path: "/some/other/repo"},
		},
	}

	err := autoStartDaemonUsing(
		dir,
		func() (globalconfig.GlobalConfig, error) { return cfg, nil },
		func(string) error {
			startCalled = true
			return nil
		},
	)

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if startCalled {
		t.Error("startDaemon should not be called when repo is not registered in global config")
	}
}

func TestAutoStartDaemon_AutoStartDisabled_Noop(t *testing.T) {
	dir := t.TempDir()
	absDir, _ := filepath.Abs(dir)
	startCalled := false
	cfg := globalconfig.GlobalConfig{
		Repositories: []globalconfig.Repository{
			{Path: absDir, AutoStartDisabled: true},
		},
	}

	err := autoStartDaemonUsing(
		dir,
		func() (globalconfig.GlobalConfig, error) { return cfg, nil },
		func(string) error {
			startCalled = true
			return nil
		},
	)

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if startCalled {
		t.Error("startDaemon should not be called when auto-start is disabled for the repo")
	}
}

func TestAutoStartDaemon_AutoStartEnabled_CallsStart(t *testing.T) {
	dir := t.TempDir()
	absDir, _ := filepath.Abs(dir)
	startCalled := false
	cfg := globalconfig.GlobalConfig{
		Repositories: []globalconfig.Repository{
			{Path: absDir},
		},
	}

	_ = autoStartDaemonUsing(
		dir,
		func() (globalconfig.GlobalConfig, error) { return cfg, nil },
		func(string) error {
			startCalled = true
			return nil
		},
	)

	if !startCalled {
		t.Error("startDaemon should be called when repo is registered and auto-start is enabled")
	}
}

package logging

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leberkas-org/maggus/internal/config"
)

func SetupDaemon() (*slog.Logger, func(), error) {
	globalDir, err := config.GlobalDir()
	if err != nil {
		return slog.Default(), func() {}, err
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return slog.Default(), func() {}, err
	}

	logPath := filepath.Join(globalDir, "daemon.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return slog.Default(), func() {}, err
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	cleanup := func() { f.Close() }
	return logger, cleanup, nil
}

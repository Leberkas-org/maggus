//go:build !windows

package discord

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"
)

// ipcDialTimeout is the maximum time to wait for a connection to Discord.
const ipcDialTimeout = 5 * time.Second

// connectIPC connects to Discord's Unix domain socket.
// Returns nil, nil if Discord is not running (connection refused / socket not found).
func connectIPC() (net.Conn, error) {
	socket := findSocket()
	if socket == "" {
		return nil, nil
	}

	conn, err := net.DialTimeout("unix", socket, ipcDialTimeout)
	if err != nil {
		return nil, nil // Discord not running — silent
	}
	return conn, nil
}

// findSocket looks for discord-ipc-0 in standard XDG/temp locations.
func findSocket() string {
	candidates := []string{
		os.Getenv("XDG_RUNTIME_DIR"),
		os.Getenv("TMPDIR"),
		"/tmp",
	}

	// Also check common snap/flatpak paths.
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates,
			filepath.Join(runtimeDir, "app", "com.discordapp.Discord"),
			filepath.Join(runtimeDir, "snap.discord"),
		)
	}

	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "discord-ipc-0")
		if info, err := os.Stat(path); err == nil && info.Mode().Type() == os.ModeSocket {
			return path
		}
	}

	// Fallback: try connecting even if stat doesn't show a socket (some systems behave differently).
	for _, dir := range []string{os.Getenv("XDG_RUNTIME_DIR"), "/tmp"} {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "discord-ipc-0")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "/tmp/discord-ipc-0" // last resort fallback
}

// readMessageWithTimeout reads an IPC message with a bounded timeout.
// On Unix, net.Conn supports SetReadDeadline natively, so this uses
// the standard deadline mechanism.
func readMessageWithTimeout(conn net.Conn, timeout time.Duration) (uint32, json.RawMessage, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	op, data, err := readMessage(conn)
	conn.SetReadDeadline(time.Time{})
	return op, data, err
}

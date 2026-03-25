package discord

import (
	"net"
	"time"
)

// ipcDialTimeout is the maximum time to wait for a connection to Discord.
const ipcDialTimeout = 5 * time.Second

// connectIPC connects to Discord's named pipe on Windows.
// Returns nil, nil if Discord is not running (connection refused).
func connectIPC() (net.Conn, error) {
	conn, err := dial(`\\.\pipe\discord-ipc-0`)
	if err != nil {
		return nil, nil // Discord not running — silent
	}
	return conn, nil
}

// dial attempts to connect to the named pipe with a timeout.
func dial(path string) (net.Conn, error) {
	return net.DialTimeout("pipe", path, ipcDialTimeout)
}

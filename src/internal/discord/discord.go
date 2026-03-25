// Package discord implements Discord Rich Presence integration for Maggus.
//
// It uses Discord's local IPC protocol directly (no external library dependency).
// On Windows it connects via named pipe (\\.\pipe\discord-ipc-0), on Unix via
// a domain socket (/tmp/discord-ipc-0 or XDG_RUNTIME_DIR equivalent).
package discord

import (
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// ApplicationID is the Discord Application ID registered at
// https://discord.com/developers/applications for the Maggus Rich Presence integration.
// This is a public value (not a secret) and is safe to hardcode.
//
// TODO: Replace with the actual Application ID once the Discord Application is created.
// See docs/discord-setup.md for setup instructions.
const ApplicationID = "1029849597473996810"

// AssetKeyLargeImage is the Rich Presence asset key for the Maggus logo.
const AssetKeyLargeImage = "maggus_logo"

// PresenceState holds the data needed to update the Discord Rich Presence.
type PresenceState struct {
	TaskID          string
	TaskTitle       string
	FeatureTitle    string
	StartTime       time.Time
	Verb            string // e.g. "Working", "Fixing", "Planning", "Consulting"
	ProgressCurrent int    // completed tasks (0 means no progress)
	ProgressTotal   int    // total tasks (0 means no progress bar)
}

// Presence manages a connection to Discord's local IPC for Rich Presence updates.
type Presence struct {
	mu            sync.Mutex
	conn          net.Conn
	connected     bool
	disconnected  bool // true after a mid-session disconnect — prevents retry spam
	loggedDisconn bool // true after the disconnect has been logged once
}

// Connect establishes a connection to Discord's local IPC.
// Returns nil if Discord is not running — this is not an error.
func (p *Presence) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, err := connectIPC()
	if err != nil {
		return err
	}
	if conn == nil {
		// Discord not running — silently return.
		return nil
	}

	// Send handshake.
	hs := handshakePayload{V: 1, ClientID: ApplicationID}
	if err := writeMessage(conn, opHandshake, hs); err != nil {
		conn.Close()
		return nil // Treat handshake failure as "Discord not available".
	}

	// Read handshake response (with timeout).
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = readMessage(conn)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return nil // Treat response failure as "Discord not available".
	}

	p.conn = conn
	p.connected = true
	return nil
}

// Update sets the Discord Rich Presence activity.
// Does nothing if not connected or if Discord has disconnected mid-session.
func (p *Presence) Update(state PresenceState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.connected || p.disconnected {
		return nil
	}

	payload := buildSetActivityPayload(os.Getpid(), state)
	if err := writeMessage(p.conn, opFrame, payload); err != nil {
		p.handleDisconnect()
		return nil
	}

	// Read the response to keep the protocol in sync.
	p.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := readMessage(p.conn)
	p.conn.SetReadDeadline(time.Time{})
	if err != nil {
		p.handleDisconnect()
	}

	return nil
}

// Close clears the presence and disconnects from Discord.
// Safe to call even if not connected.
func (p *Presence) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.connected || p.conn == nil {
		return nil
	}

	// Best-effort: clear activity and send opClose before closing.
	// Errors are ignored — we always close the connection regardless.
	if !p.disconnected {
		payload := buildClearActivityPayload(os.Getpid())
		if err := writeMessage(p.conn, opFrame, payload); err == nil {
			// Read the response to keep the protocol in sync (same pattern as Update).
			p.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, _, _ = readMessage(p.conn)
			p.conn.SetReadDeadline(time.Time{})
		}

		// Signal graceful disconnect via opClose.
		_ = writeMessage(p.conn, opClose, map[string]string{})
	}

	err := p.conn.Close()
	p.conn = nil
	p.connected = false
	return err
}

// handleDisconnect marks the connection as disconnected and logs once.
// Must be called with p.mu held.
func (p *Presence) handleDisconnect() {
	p.disconnected = true
	if !p.loggedDisconn {
		log.Println("discord: connection lost, presence updates disabled for this session")
		p.loggedDisconn = true
	}
}

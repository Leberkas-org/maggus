package discord

import (
	"net"
	"os"
	"time"
)

// connectIPC connects to Discord's named pipe on Windows.
// Returns nil, nil if Discord is not running (pipe not found).
func connectIPC() (net.Conn, error) {
	f, err := os.OpenFile(`\\.\pipe\discord-ipc-0`, os.O_RDWR, 0)
	if err != nil {
		return nil, nil // Discord not running — silent
	}
	return &pipeConn{f: f}, nil
}

// pipeConn wraps an *os.File to satisfy net.Conn for Windows named pipes.
type pipeConn struct {
	f *os.File
}

func (c *pipeConn) Read(b []byte) (int, error)  { return c.f.Read(b) }
func (c *pipeConn) Write(b []byte) (int, error) { return c.f.Write(b) }
func (c *pipeConn) Close() error                { return c.f.Close() }

func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (c *pipeConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *pipeConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *pipeConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// pipeAddr implements net.Addr for the named pipe.
type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return `\\.\pipe\discord-ipc-0` }

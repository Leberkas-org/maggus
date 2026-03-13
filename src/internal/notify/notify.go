package notify

import (
	"fmt"
	"io"
	"os"

	"github.com/leberkas-org/maggus/internal/config"
)

// Notifier plays sound notifications based on config settings.
type Notifier struct {
	cfg config.NotificationsConfig
	w   io.Writer
}

// New creates a Notifier that writes BEL characters to os.Stdout.
func New(cfg config.NotificationsConfig) *Notifier {
	return &Notifier{cfg: cfg, w: os.Stdout}
}

// NewWithWriter creates a Notifier that writes BEL characters to w (for testing).
func NewWithWriter(cfg config.NotificationsConfig, w io.Writer) *Notifier {
	return &Notifier{cfg: cfg, w: w}
}

func (n *Notifier) bel() {
	fmt.Fprint(n.w, "\a")
}

// PlayTaskComplete plays a sound after a task completes successfully.
func (n *Notifier) PlayTaskComplete() {
	if n.cfg.IsTaskCompleteEnabled() {
		n.bel()
	}
}

// PlayRunComplete plays a sound when the entire run finishes.
func (n *Notifier) PlayRunComplete() {
	if n.cfg.IsRunCompleteEnabled() {
		n.bel()
	}
}

// PlayError plays a sound when an error occurs.
func (n *Notifier) PlayError() {
	if n.cfg.IsErrorEnabled() {
		n.bel()
	}
}

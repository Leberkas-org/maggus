package notify

import (
	"bytes"
	"testing"

	"github.com/leberkas-org/maggus/internal/config"
)

func boolPtr(v bool) *bool { return &v }

func TestPlayTaskComplete_SoundEnabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true}, &buf)
	n.PlayTaskComplete()
	if buf.String() != "\a" {
		t.Errorf("expected BEL, got %q", buf.String())
	}
}

func TestPlayTaskComplete_SoundDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: false}, &buf)
	n.PlayTaskComplete()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPlayTaskComplete_ExplicitlyDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true, OnTaskComplete: boolPtr(false)}, &buf)
	n.PlayTaskComplete()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPlayRunComplete_SoundEnabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true}, &buf)
	n.PlayRunComplete()
	if buf.String() != "\a" {
		t.Errorf("expected BEL, got %q", buf.String())
	}
}

func TestPlayRunComplete_SoundDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: false}, &buf)
	n.PlayRunComplete()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPlayRunComplete_ExplicitlyDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true, OnRunComplete: boolPtr(false)}, &buf)
	n.PlayRunComplete()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPlayError_SoundEnabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true}, &buf)
	n.PlayError()
	if buf.String() != "\a" {
		t.Errorf("expected BEL, got %q", buf.String())
	}
}

func TestPlayError_SoundDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: false}, &buf)
	n.PlayError()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestPlayError_ExplicitlyDisabled(t *testing.T) {
	var buf bytes.Buffer
	n := NewWithWriter(config.NotificationsConfig{Sound: true, OnError: boolPtr(false)}, &buf)
	n.PlayError()
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestDefaultsWhenSoundEnabled(t *testing.T) {
	// When sound is true and individual toggles are nil, all default to true.
	var buf bytes.Buffer
	cfg := config.NotificationsConfig{Sound: true}
	n := NewWithWriter(cfg, &buf)

	n.PlayTaskComplete()
	n.PlayRunComplete()
	n.PlayError()

	if buf.String() != "\a\a\a" {
		t.Errorf("expected 3 BELs, got %q", buf.String())
	}
}

func TestMasterToggleOverridesIndividual(t *testing.T) {
	// Even if individual toggles are true, sound:false means no sound.
	var buf bytes.Buffer
	cfg := config.NotificationsConfig{
		Sound:          false,
		OnTaskComplete: boolPtr(true),
		OnRunComplete:  boolPtr(true),
		OnError:        boolPtr(true),
	}
	n := NewWithWriter(cfg, &buf)

	n.PlayTaskComplete()
	n.PlayRunComplete()
	n.PlayError()

	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

package runner

import (
	"testing"
	"time"
)

func TestFormatHHMMSS(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero duration", 0, "00:00:00"},
		{"seconds only", 45 * time.Second, "00:00:45"},
		{"minutes and seconds", 2*time.Minute + 15*time.Second, "00:02:15"},
		{"hours minutes seconds", 1*time.Hour + 30*time.Minute + 5*time.Second, "01:30:05"},
		{"90 minutes rolls over to hours", 90 * time.Minute, "01:30:00"},
		{"large duration 24h+", 25*time.Hour + 30*time.Minute + 10*time.Second, "25:30:10"},
		{"sub-second truncated", 2*time.Minute + 15*time.Second + 500*time.Millisecond, "00:02:15"},
		{"negative treated as zero", -5 * time.Second, "00:00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHHMMSS(tt.duration)
			if got != tt.expected {
				t.Errorf("formatHHMMSS(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}

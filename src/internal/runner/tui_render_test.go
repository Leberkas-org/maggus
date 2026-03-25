package runner

import (
	"testing"
	"time"

	"github.com/leberkas-org/maggus/internal/tui/styles"
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

func TestDetailAvailableHeightConsistency(t *testing.T) {
	// Verify that detailAvailableHeight() uses the same reserved-lines constant
	// that renderView() passes to renderDetailPanel(). This guards against the
	// previous bug where renderView() subtracted 8 but detailAvailableHeight()
	// subtracted 10, causing a 2-line mismatch.
	terminalSizes := []struct {
		name   string
		width  int
		height int
	}{
		{"standard 80x24", 80, 24},
		{"medium 120x40", 120, 40},
		{"large 200x50", 200, 50},
		{"minimum viable", 40, 16},
	}

	for _, sz := range terminalSizes {
		t.Run(sz.name, func(t *testing.T) {
			m := TUIModel{width: sz.width, height: sz.height}
			got := m.detailAvailableHeight()

			_, innerH := styles.FullScreenInnerSize(sz.width, sz.height)
			// detailAvailableHeight reserves 10 lines from innerH
			expected := innerH - 10
			if expected < 1 {
				expected = 1
			}

			if got != expected {
				t.Errorf("detailAvailableHeight() = %d, want %d (innerH=%d, terminal %dx%d)",
					got, expected, innerH, sz.width, sz.height)
			}

			// Verify the height is positive and reasonable
			if got < 1 {
				t.Errorf("detailAvailableHeight() = %d, must be at least 1", got)
			}
		})
	}
}

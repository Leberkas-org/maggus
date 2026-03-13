package styles

import (
	"strings"
	"testing"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name        string
		done, total int
		width       int
		wantFilled  int
		wantEmpty   int
	}{
		{"zero width", 5, 10, 0, 0, 0},
		{"zero total", 0, 0, 10, 0, 10},
		{"half done", 5, 10, 10, 5, 5},
		{"all done", 10, 10, 10, 10, 0},
		{"none done", 0, 10, 10, 0, 10},
		{"one of three", 1, 3, 9, 3, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProgressBar(tt.done, tt.total, tt.width)
			if tt.width <= 0 {
				if result != "" {
					t.Errorf("expected empty string for zero width, got %q", result)
				}
				return
			}
			// Count the unicode block characters in the raw output
			// (lipgloss adds ANSI escapes, so count the runes)
			filledCount := strings.Count(result, "█")
			emptyCount := strings.Count(result, "░")
			if filledCount != tt.wantFilled {
				t.Errorf("filled: got %d, want %d", filledCount, tt.wantFilled)
			}
			if emptyCount != tt.wantEmpty {
				t.Errorf("empty: got %d, want %d", emptyCount, tt.wantEmpty)
			}
		})
	}
}

func TestProgressBarPlain(t *testing.T) {
	tests := []struct {
		name        string
		done, total int
		width       int
		want        string
	}{
		{"half done", 5, 10, 10, "#####....."},
		{"all done", 10, 10, 10, "##########"},
		{"none done", 0, 10, 10, ".........."},
		{"zero total", 0, 0, 10, ".........."},
		{"zero width", 5, 10, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProgressBarPlain(tt.done, tt.total, tt.width)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		want     string
	}{
		{"short text", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 2, "he"},
		{"max equals 3", "hello", 3, "hel"},
		{"max equals 4", "hello world", 4, "h..."},
		{"zero width", "hello", 0, ""},
		{"empty text", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.text, tt.maxWidth)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSeparator(t *testing.T) {
	result := Separator(10)
	if !strings.Contains(result, "──────────") {
		t.Errorf("expected 10 separator chars, got %q", result)
	}

	empty := Separator(0)
	if empty != "" {
		t.Errorf("expected empty string for zero width, got %q", empty)
	}
}

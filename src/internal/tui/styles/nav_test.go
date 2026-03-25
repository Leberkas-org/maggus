package styles

import "testing"

func TestCursorUp(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		count  int
		want   int
	}{
		{"normal move up", 3, 5, 2},
		{"normal move up from 1", 1, 5, 0},
		{"wraparound at 0", 0, 5, 4},
		{"wraparound count=1", 0, 1, 0},
		{"empty list count=0", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CursorUp(tt.cursor, tt.count)
			if got != tt.want {
				t.Errorf("CursorUp(%d, %d) = %d, want %d", tt.cursor, tt.count, got, tt.want)
			}
		})
	}
}

func TestCursorDown(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		count  int
		want   int
	}{
		{"normal move down", 2, 5, 3},
		{"normal move down from 0", 0, 5, 1},
		{"wraparound at last", 4, 5, 0},
		{"wraparound count=1", 0, 1, 0},
		{"empty list count=0", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CursorDown(tt.cursor, tt.count)
			if got != tt.want {
				t.Errorf("CursorDown(%d, %d) = %d, want %d", tt.cursor, tt.count, got, tt.want)
			}
		})
	}
}

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		count  int
		want   int
	}{
		{"within bounds", 2, 5, 2},
		{"at last", 4, 5, 4},
		{"above count (clamp after deletion)", 5, 5, 4},
		{"far above count", 10, 3, 2},
		{"negative cursor", -1, 5, 0},
		{"empty list count=0", 0, 0, 0},
		{"empty list with positive cursor", 3, 0, 0},
		{"count=1 cursor=0", 0, 1, 0},
		{"count=1 cursor=1 (clamp)", 1, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampCursor(tt.cursor, tt.count)
			if got != tt.want {
				t.Errorf("ClampCursor(%d, %d) = %d, want %d", tt.cursor, tt.count, got, tt.want)
			}
		})
	}
}

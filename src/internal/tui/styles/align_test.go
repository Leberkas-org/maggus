package styles

import (
	"testing"
)

func TestRightAlign(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		width int
		want  string
	}{
		{
			name:  "normal case pad > 0",
			left:  "hello",
			right: "ts",
			width: 12,
			want:  "hello     ts",
		},
		{
			name:  "tight fit pad == 0 returns left unchanged",
			left:  "hello",
			right: "ts",
			width: 7,
			want:  "hello",
		},
		{
			name:  "overflow pad < 0 returns left unchanged",
			left:  "hello world",
			right: "ts",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty left",
			left:  "",
			right: "ts",
			width: 10,
			want:  "        ts",
		},
		{
			name:  "empty right",
			left:  "hello",
			right: "",
			width: 10,
			want:  "hello     ",
		},
		{
			name:  "both empty",
			left:  "",
			right: "",
			width: 5,
			want:  "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RightAlign(tt.left, tt.right, tt.width)
			if got != tt.want {
				t.Errorf("RightAlign(%q, %q, %d) = %q, want %q", tt.left, tt.right, tt.width, got, tt.want)
			}
		})
	}
}

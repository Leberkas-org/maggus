package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
		want string
	}{
		{
			name: "uppercase Q lowercases to q",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}},
			want: "q",
		},
		{
			name: "lowercase q passes through unchanged",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			want: "q",
		},
		{
			name: "ctrl+c passes through unchanged",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
			want: "ctrl+c",
		},
		{
			name: "uppercase G lowercases to g",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}},
			want: "g",
		},
		{
			name: "enter passes through unchanged",
			msg:  tea.KeyMsg{Type: tea.KeyEnter},
			want: "enter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeKey(tt.msg)
			if got != tt.want {
				t.Errorf("normalizeKey(%q) = %q, want %q", tt.msg.String(), got, tt.want)
			}
		})
	}
}

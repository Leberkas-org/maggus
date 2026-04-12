package cmd

import tea "github.com/charmbracelet/bubbletea"

// normalizeKey returns the normalized string representation of a key message.
// Single ASCII uppercase letters (A-Z) are lowercased; everything else passes through unchanged.
// This makes single-letter shortcuts work regardless of Caps Lock state.
func normalizeKey(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= 'A' && r <= 'Z' {
			return string(r + 32)
		}
	}
	return msg.String()
}

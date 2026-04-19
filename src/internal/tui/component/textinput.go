package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

type TextInput struct {
	Label       string
	Value       string
	Cursor      int
	Placeholder string
	Width       int
}

func NewTextInput(label, placeholder string) *TextInput {
	return &TextInput{
		Label:       label,
		Placeholder: placeholder,
	}
}

func (t *TextInput) InsertRune(r rune) {
	before := t.Value[:t.Cursor]
	after := t.Value[t.Cursor:]
	t.Value = before + string(r) + after
	t.Cursor++
}

func (t *TextInput) Backspace() {
	if t.Cursor > 0 {
		before := t.Value[:t.Cursor-1]
		after := t.Value[t.Cursor:]
		t.Value = before + after
		t.Cursor--
	}
}

func (t *TextInput) Delete() {
	if t.Cursor < len(t.Value) {
		before := t.Value[:t.Cursor]
		after := t.Value[t.Cursor+1:]
		t.Value = before + after
	}
}

func (t *TextInput) MoveLeft() {
	if t.Cursor > 0 {
		t.Cursor--
	}
}

func (t *TextInput) MoveRight() {
	if t.Cursor < len(t.Value) {
		t.Cursor++
	}
}

func (t *TextInput) Home() { t.Cursor = 0 }
func (t *TextInput) End()  { t.Cursor = len(t.Value) }

func (t *TextInput) View() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(styles.Muted).
		Width(max(t.Width-4, 20)).
		Padding(0, 1)

	var display string
	if t.Value == "" {
		display = lipgloss.NewStyle().Foreground(styles.Muted).Render(t.Placeholder)
	} else {
		before := t.Value[:t.Cursor]
		cursorChar := " "
		after := ""
		if t.Cursor < len(t.Value) {
			cursorChar = string(t.Value[t.Cursor])
			after = t.Value[t.Cursor+1:]
		}
		cursor := lipgloss.NewStyle().
			Background(styles.Primary).
			Foreground(lipgloss.Color("0")).
			Render(cursorChar)
		display = before + cursor + after
	}

	return labelStyle.Render(t.Label) + "\n" + inputStyle.Render(display)
}

func (t *TextInput) Reset() {
	t.Value = ""
	t.Cursor = 0
}

// Dialog renders a complete input dialog with title, input, and hint
type Dialog struct {
	Title  string
	Inputs []*TextInput
	Active int
	Hint   string
	Width  int
	Height int
}

func NewDialog(title string, inputs []*TextInput, hint string) *Dialog {
	return &Dialog{
		Title:  title,
		Inputs: inputs,
		Hint:   hint,
	}
}

func (d *Dialog) ActiveInput() *TextInput {
	if d.Active < len(d.Inputs) {
		return d.Inputs[d.Active]
	}
	return nil
}

func (d *Dialog) NextInput() {
	if d.Active < len(d.Inputs)-1 {
		d.Active++
	}
}

func (d *Dialog) View() string {
	boxW := max(min(d.Width-4, 60), 30)
	innerW := max(boxW-6, 10) // border(2) + padding(4)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)

	var lines []string
	lines = append(lines, titleStyle.Render(d.Title))
	lines = append(lines, "")

	for i, input := range d.Inputs {
		input.Width = innerW
		lines = append(lines, input.View())
		if i < len(d.Inputs)-1 {
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	hintStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	lines = append(lines, hintStyle.Render(d.Hint))

	content := strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2).
		Width(boxW)

	return box.Render(content)
}

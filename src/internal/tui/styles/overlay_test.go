package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOverlayCenterSmallerThanTerminal(t *testing.T) {
	// 10x5 terminal, 4x3 fg → startX=3, startY=1
	bg := strings.Join([]string{
		"AAAAAAAAAA",
		"BBBBBBBBBB",
		"CCCCCCCCCC",
		"DDDDDDDDDD",
		"EEEEEEEEEE",
	}, "\n")
	fg := strings.Join([]string{
		"1111",
		"2222",
		"3333",
	}, "\n")

	got := OverlayCenter(bg, fg, 10, 5)
	lines := strings.Split(got, "\n")

	// Row 0: unchanged
	if lines[0] != "AAAAAAAAAA" {
		t.Errorf("row 0: got %q, want %q", lines[0], "AAAAAAAAAA")
	}
	// Row 1: BBB + 1111 + BBB
	if lines[1] != "BBB1111BBB" {
		t.Errorf("row 1: got %q, want %q", lines[1], "BBB1111BBB")
	}
	// Row 2: CCC + 2222 + CCC
	if lines[2] != "CCC2222CCC" {
		t.Errorf("row 2: got %q, want %q", lines[2], "CCC2222CCC")
	}
	// Row 3: DDD + 3333 + DDD
	if lines[3] != "DDD3333DDD" {
		t.Errorf("row 3: got %q, want %q", lines[3], "DDD3333DDD")
	}
	// Row 4: unchanged
	if lines[4] != "EEEEEEEEEE" {
		t.Errorf("row 4: got %q, want %q", lines[4], "EEEEEEEEEE")
	}
}

func TestOverlayCenterExactFit(t *testing.T) {
	// fg exactly fills terminal: 6x3 terminal, 6x3 fg → startX=0, startY=0
	bg := strings.Join([]string{
		"AAAAAA",
		"BBBBBB",
		"CCCCCC",
	}, "\n")
	fg := strings.Join([]string{
		"111111",
		"222222",
		"333333",
	}, "\n")

	got := OverlayCenter(bg, fg, 6, 3)
	lines := strings.Split(got, "\n")

	// All rows replaced by fg (startX=0 means no left/right flanks).
	want := []string{"111111", "222222", "333333"}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("row %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestOverlayCenterLargerThanTerminal(t *testing.T) {
	// fg wider than terminal → fallback to lipgloss.Place
	bg := strings.Join([]string{
		"AAA",
		"BBB",
	}, "\n")
	fg := strings.Join([]string{
		"1111111111", // 10 cols in a 3-wide terminal
		"2222222222",
	}, "\n")

	got := OverlayCenter(bg, fg, 3, 2)

	// Fallback should return fg placed (not bg-composited).
	// The result should contain fg content since lipgloss.Place wraps it.
	if !strings.Contains(got, "1111111111") {
		t.Errorf("fallback: expected fg content in result, got %q", got)
	}
	// bg should NOT appear in fallback output.
	if strings.Contains(got, "AAA") {
		t.Errorf("fallback: bg content should not appear in result, got %q", got)
	}
}

func TestOverlayCenterTallerThanTerminal(t *testing.T) {
	// fg taller than terminal → fallback
	bg := "AAA"
	fg := strings.Join([]string{
		"111",
		"222",
		"333",
	}, "\n")

	got := OverlayCenter(bg, fg, 3, 2)
	if !strings.Contains(got, "111") {
		t.Errorf("fallback (tall): expected fg content, got %q", got)
	}
}

func TestOverlayCenterBgShorterThanExpected(t *testing.T) {
	// bg lines are shorter than the overlay region. The function should
	// pad with spaces so the fg is still placed at the correct column.
	bg := strings.Join([]string{
		"AB",
		"CD",
		"EF",
		"GH",
		"IJ",
	}, "\n")
	// 10x5 terminal, 4x1 fg → startX=3, startY=2
	fg := "XXXX"

	got := OverlayCenter(bg, fg, 10, 5)
	lines := strings.Split(got, "\n")

	// Row 2 bg is "EF" (width 2), startX=3 → pad 1 space → "EF " + "XXXX"
	if lines[2] != "EF XXXX" {
		t.Errorf("short bg row: got %q, want %q", lines[2], "EF XXXX")
	}
	// Other rows unchanged.
	if lines[0] != "AB" {
		t.Errorf("row 0: got %q, want %q", lines[0], "AB")
	}
}

func TestOverlayCenterBgFewerLinesThanTerminal(t *testing.T) {
	// bg has fewer lines than termH — padding should be added so fg
	// can be placed at the correct vertical position.
	bg := "single line"
	fg := "XX"

	// 11x5 terminal, 2x1 fg → startX=4, startY=2 — but bg has only 1 line.
	got := OverlayCenter(bg, fg, 11, 5)
	lines := strings.Split(got, "\n")

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	// Row 0: original bg line.
	if lines[0] != "single line" {
		t.Errorf("row 0: got %q, want %q", lines[0], "single line")
	}
	// Row 2: padded left + fg (no right since bg line was empty).
	want := "    XX"
	if lines[2] != want {
		t.Errorf("row 2: got %q, want %q", lines[2], want)
	}
}

func TestOverlayCenterWithANSI(t *testing.T) {
	// Verify ANSI escape sequences in bg are not corrupted.
	red := "\x1b[31m"
	reset := "\x1b[0m"

	bg := strings.Join([]string{
		red + "AAAAAAAAAA" + reset,
		red + "BBBBBBBBBB" + reset,
		red + "CCCCCCCCCC" + reset,
	}, "\n")
	fg := "XX"

	// 10x3 terminal, 2x1 fg → startX=4, startY=1
	got := OverlayCenter(bg, fg, 10, 3)
	lines := strings.Split(got, "\n")

	// Row 0: unchanged (still has ANSI).
	if ansi.StringWidth(lines[0]) != 10 {
		t.Errorf("row 0 visual width: got %d, want 10", ansi.StringWidth(lines[0]))
	}
	// Row 1: composited — left(4) + XX + right(4). Visual width should be 10.
	if ansi.StringWidth(lines[1]) != 10 {
		t.Errorf("row 1 visual width: got %d, want 10", ansi.StringWidth(lines[1]))
	}
	// fg content must be present.
	if !strings.Contains(lines[1], "XX") {
		t.Errorf("row 1 should contain fg content 'XX', got %q", lines[1])
	}
	// Row 2: unchanged.
	if ansi.StringWidth(lines[2]) != 10 {
		t.Errorf("row 2 visual width: got %d, want 10", ansi.StringWidth(lines[2]))
	}
}

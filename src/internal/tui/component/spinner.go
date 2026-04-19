package component

import "github.com/leberkas-org/maggus/internal/tui/styles"

type Spinner struct {
	frame int
}

func NewSpinner() *Spinner {
	return &Spinner{}
}

func (s *Spinner) Tick() {
	s.frame = (s.frame + 1) % len(styles.SpinnerFrames)
}

func (s *Spinner) View() string {
	return string(styles.SpinnerFrames[s.frame])
}

package claude2x

import (
	"fmt"
	"sync"
	"time"
)

// Status represents the current Claude rate limit window status.
// Nerfed hours: 13:00–19:00 UTC on weekdays. All other times are normal.
type Status struct {
	IsNerfed                   bool   `json:"isNerfed"`
	TwoXWindowExpiresIn        string `json:"expiresIn"`
	TwoXWindowExpiresInSeconds int    `json:"expiresInSeconds"`
}

var (
	mu           sync.RWMutex
	testOverride *Status
)

// FetchStatus computes the current Claude rate window status from the current UTC time.
func FetchStatus() Status {
	mu.RLock()
	if testOverride != nil {
		s := *testOverride
		mu.RUnlock()
		return s
	}
	mu.RUnlock()
	return computeFromTime(time.Now().UTC())
}

func computeFromTime(now time.Time) Status {
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return Status{}
	}

	h, m, s := now.Clock()
	totalSecs := h*3600 + m*60 + s

	const (
		nerfStart = 13 * 3600 // 13:00 UTC
		nerfEnd   = 19 * 3600 // 19:00 UTC
	)

	if totalSecs >= nerfStart && totalSecs < nerfEnd {
		remaining := nerfEnd - totalSecs
		return Status{
			IsNerfed:                   true,
			TwoXWindowExpiresInSeconds: remaining,
			TwoXWindowExpiresIn:        formatRemaining(remaining),
		}
	}

	return Status{}
}

func formatRemaining(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

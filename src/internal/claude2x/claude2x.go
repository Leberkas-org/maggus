package claude2x

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const apiURL = "https://isclaude2x.com/json"

// Status represents the response from the isclaude2x API.
type Status struct {
	Is2x                       bool   `json:"is2x"`
	TwoXWindowExpiresIn        string `json:"2xWindowExpiresIn"`
	TwoXWindowExpiresInSeconds int    `json:"2xWindowExpiresInSeconds"`
}

var (
	once        sync.Once
	cached      Status
	fetchedAt   time.Time
	urlOverride string
)

// FetchStatus performs a GET request to the isclaude2x API at most once per process lifetime.
// Subsequent calls compute the current status from the cached result and elapsed time.
// On any error (network, parse, non-200 status), it returns a zero-value Status with Is2x: false.
func FetchStatus() Status {
	once.Do(func() {
		url := apiURL
		if urlOverride != "" {
			url = urlOverride
		}
		cached = fetchFromURL(url)
		fetchedAt = time.Now()
	})
	return computeStatus()
}

func computeStatus() Status {
	if !cached.Is2x {
		return Status{}
	}
	remaining := cached.TwoXWindowExpiresInSeconds - int(time.Since(fetchedAt).Seconds())
	if remaining <= 0 {
		return Status{Is2x: false}
	}
	return Status{
		Is2x:                       true,
		TwoXWindowExpiresInSeconds: remaining,
		TwoXWindowExpiresIn:        formatRemaining(remaining),
	}
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

func resetCache() {
	once = sync.Once{}
	cached = Status{}
	fetchedAt = time.Time{}
	urlOverride = ""
}

func fetchFromURL(url string) Status {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Status{}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Status{}
	}

	var s Status
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return Status{}
	}
	return s
}

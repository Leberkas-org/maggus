package claude2x

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const apiURL = "https://isclaude2x.com/json"

// Status represents the response from the isclaude2x API.
type Status struct {
	Is2x                       bool   `json:"is2x"`
	TwoXWindowExpiresIn        string `json:"2xWindowExpiresIn"`
	TwoXWindowExpiresInSeconds int    `json:"2xWindowExpiresInSeconds"`
}

// FetchStatus performs a GET request to the isclaude2x API and returns the parsed status.
// On any error (network, parse, non-200 status), it returns a zero-value Status with Is2x: false.
func FetchStatus() Status {
	return fetchFromURL(apiURL)
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

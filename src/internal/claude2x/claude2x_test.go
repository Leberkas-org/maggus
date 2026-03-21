package claude2x

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchStatus_Success2x(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     true,
			"2xWindowExpiresIn":        "17h 54m 44s",
			"2xWindowExpiresInSeconds": 64484,
		})
	}))
	defer srv.Close()

	urlOverride = srv.URL
	s := FetchStatus()
	if !s.Is2x {
		t.Error("expected Is2x to be true")
	}
	if s.TwoXWindowExpiresInSeconds < 64480 {
		t.Errorf("expected TwoXWindowExpiresInSeconds ~64484, got %d", s.TwoXWindowExpiresInSeconds)
	}
}

func TestFetchStatus_SuccessNot2x(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     false,
			"2xWindowExpiresIn":        nil,
			"2xWindowExpiresInSeconds": 0,
		})
	}))
	defer srv.Close()

	urlOverride = srv.URL
	s := FetchStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false")
	}
}

func TestFetchStatus_Non200(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	urlOverride = srv.URL
	s := FetchStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false on non-200")
	}
}

func TestFetchStatus_MalformedJSON(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	urlOverride = srv.URL
	s := FetchStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false on malformed JSON")
	}
}

func TestFetchStatus_NetworkError(t *testing.T) {
	resetCache()
	urlOverride = "http://127.0.0.1:1" // nothing listening
	s := FetchStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false on network error")
	}
}

func TestFetchStatus_Timeout(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		json.NewEncoder(w).Encode(map[string]any{"is2x": true})
	}))
	defer srv.Close()

	urlOverride = srv.URL
	start := time.Now()
	s := FetchStatus()
	elapsed := time.Since(start)

	if s.Is2x {
		t.Error("expected Is2x to be false on timeout")
	}
	if elapsed > 4*time.Second {
		t.Errorf("expected timeout within ~3s, took %v", elapsed)
	}
}

func TestFetchStatus_ExtraFieldsIgnored(t *testing.T) {
	resetCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     true,
			"2xWindowExpiresIn":        "1h 2m 3s",
			"2xWindowExpiresInSeconds": 3723,
			"promoActive":              true,
			"isPeak":                   false,
			"currentTimeET":            "2026-03-19T19:15:42",
		})
	}))
	defer srv.Close()

	urlOverride = srv.URL
	s := FetchStatus()
	if !s.Is2x {
		t.Error("expected Is2x to be true")
	}
	if s.TwoXWindowExpiresInSeconds < 3720 {
		t.Errorf("expected ~3723, got %d", s.TwoXWindowExpiresInSeconds)
	}
}

func TestFetchStatus_CacheHit(t *testing.T) {
	resetCache()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     true,
			"2xWindowExpiresInSeconds": 3600,
			"2xWindowExpiresIn":        "1h 0m 0s",
		})
	}))
	defer srv.Close()

	urlOverride = srv.URL
	FetchStatus()
	FetchStatus()
	FetchStatus()

	if calls != 1 {
		t.Errorf("expected exactly 1 API call, got %d", calls)
	}
}

func TestComputeStatus_StillActive(t *testing.T) {
	resetCache()
	cached = Status{
		Is2x:                       true,
		TwoXWindowExpiresInSeconds: 3600,
		TwoXWindowExpiresIn:        "1h 0m 0s",
	}
	fetchedAt = time.Now().Add(-30 * time.Minute)

	s := computeStatus()
	if !s.Is2x {
		t.Error("expected Is2x to be true")
	}
	// ~1800 remaining (3600 - 1800)
	if s.TwoXWindowExpiresInSeconds < 1790 || s.TwoXWindowExpiresInSeconds > 1810 {
		t.Errorf("expected remaining ~1800, got %d", s.TwoXWindowExpiresInSeconds)
	}
	if s.TwoXWindowExpiresIn != "30m 0s" && s.TwoXWindowExpiresIn != "29m 59s" && s.TwoXWindowExpiresIn != "30m 1s" {
		t.Errorf("expected formatted ~30m 0s, got %q", s.TwoXWindowExpiresIn)
	}
}

func TestComputeStatus_Expired(t *testing.T) {
	resetCache()
	cached = Status{
		Is2x:                       true,
		TwoXWindowExpiresInSeconds: 10,
		TwoXWindowExpiresIn:        "10s",
	}
	fetchedAt = time.Now().Add(-11 * time.Second)

	s := computeStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false after expiry")
	}
}

func TestComputeStatus_WasNot2x(t *testing.T) {
	resetCache()
	cached = Status{Is2x: false}
	fetchedAt = time.Now()

	s := computeStatus()
	if s.Is2x {
		t.Error("expected Is2x to be false")
	}
	if s.TwoXWindowExpiresInSeconds != 0 {
		t.Errorf("expected 0 seconds, got %d", s.TwoXWindowExpiresInSeconds)
	}
}

func TestFormatRemaining_HoursMinSec(t *testing.T) {
	got := formatRemaining(64484)
	if got != "17h 54m 44s" {
		t.Errorf("expected '17h 54m 44s', got %q", got)
	}
}

func TestFormatRemaining_HoursMinSec2(t *testing.T) {
	got := formatRemaining(3723)
	if got != "1h 2m 3s" {
		t.Errorf("expected '1h 2m 3s', got %q", got)
	}
}

func TestFormatRemaining_MinSec(t *testing.T) {
	got := formatRemaining(125)
	if got != "2m 5s" {
		t.Errorf("expected '2m 5s', got %q", got)
	}
}

func TestFormatRemaining_SecOnly(t *testing.T) {
	got := formatRemaining(45)
	if got != "45s" {
		t.Errorf("expected '45s', got %q", got)
	}
}

func TestFormatRemaining_Zero(t *testing.T) {
	got := formatRemaining(0)
	if got != "0s" {
		t.Errorf("expected '0s', got %q", got)
	}
}

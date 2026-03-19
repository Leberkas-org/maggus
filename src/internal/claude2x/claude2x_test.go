package claude2x

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchStatus_Success2x(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     true,
			"2xWindowExpiresIn":        "17h 54m 44s",
			"2xWindowExpiresInSeconds": 64484,
		})
	}))
	defer srv.Close()

	s := fetchFromURL(srv.URL)
	if !s.Is2x {
		t.Error("expected Is2x to be true")
	}
	if s.TwoXWindowExpiresIn != "17h 54m 44s" {
		t.Errorf("expected TwoXWindowExpiresIn '17h 54m 44s', got %q", s.TwoXWindowExpiresIn)
	}
	if s.TwoXWindowExpiresInSeconds != 64484 {
		t.Errorf("expected TwoXWindowExpiresInSeconds 64484, got %d", s.TwoXWindowExpiresInSeconds)
	}
}

func TestFetchStatus_SuccessNot2x(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"is2x":                     false,
			"2xWindowExpiresIn":        nil,
			"2xWindowExpiresInSeconds": 0,
		})
	}))
	defer srv.Close()

	s := fetchFromURL(srv.URL)
	if s.Is2x {
		t.Error("expected Is2x to be false")
	}
}

func TestFetchStatus_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := fetchFromURL(srv.URL)
	if s.Is2x {
		t.Error("expected Is2x to be false on non-200")
	}
}

func TestFetchStatus_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := fetchFromURL(srv.URL)
	if s.Is2x {
		t.Error("expected Is2x to be false on malformed JSON")
	}
}

func TestFetchStatus_NetworkError(t *testing.T) {
	s := fetchFromURL("http://127.0.0.1:1") // nothing listening
	if s.Is2x {
		t.Error("expected Is2x to be false on network error")
	}
}

func TestFetchStatus_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		json.NewEncoder(w).Encode(map[string]any{"is2x": true})
	}))
	defer srv.Close()

	start := time.Now()
	s := fetchFromURL(srv.URL)
	elapsed := time.Since(start)

	if s.Is2x {
		t.Error("expected Is2x to be false on timeout")
	}
	if elapsed > 4*time.Second {
		t.Errorf("expected timeout within ~3s, took %v", elapsed)
	}
}

func TestFetchStatus_ExtraFieldsIgnored(t *testing.T) {
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

	s := fetchFromURL(srv.URL)
	if !s.Is2x {
		t.Error("expected Is2x to be true")
	}
	if s.TwoXWindowExpiresIn != "1h 2m 3s" {
		t.Errorf("expected '1h 2m 3s', got %q", s.TwoXWindowExpiresIn)
	}
}

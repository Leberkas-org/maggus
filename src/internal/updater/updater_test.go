package updater

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestServer creates a test server and configures the package to use it.
// Returns a cleanup function that restores the original URL and client.
func setupTestServer(handler http.HandlerFunc) (*httptest.Server, func()) {
	ts := httptest.NewServer(handler)
	origURL := releaseURL
	origClient := httpClient
	releaseURL = ts.URL
	httpClient = ts.Client()
	return ts, func() {
		ts.Close()
		releaseURL = origURL
		httpClient = origClient
	}
}

func TestCheckLatestVersion_NewerAvailable(t *testing.T) {
	body := `{
		"tag_name": "v2.0.0",
		"assets": [
			{"name": "maggus_2.0.0_linux_amd64.tar.gz", "browser_download_url": "https://example.com/maggus_2.0.0_linux_amd64.tar.gz"},
			{"name": "maggus_2.0.0_windows_amd64.zip", "browser_download_url": "https://example.com/maggus_2.0.0_windows_amd64.zip"},
			{"name": "maggus_2.0.0_darwin_arm64.tar.gz", "browser_download_url": "https://example.com/maggus_2.0.0_darwin_arm64.tar.gz"}
		]
	}`
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if !info.IsNewer {
		t.Error("expected IsNewer=true for 2.0.0 vs 1.0.0")
	}
	if info.TagName != "v2.0.0" {
		t.Errorf("expected TagName=v2.0.0, got %s", info.TagName)
	}
	// DownloadURL depends on runtime.GOOS/GOARCH, just check it's non-empty or a known value
	if info.DownloadURL == "" {
		t.Log("DownloadURL empty — expected if test OS/arch not in asset list")
	}
}

func TestCheckLatestVersion_AlreadyUpToDate(t *testing.T) {
	body := `{
		"tag_name": "v1.0.0",
		"assets": [
			{"name": "maggus_1.0.0_linux_amd64.tar.gz", "browser_download_url": "https://example.com/linux"}
		]
	}`
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false when versions are equal")
	}
	if info.TagName != "v1.0.0" {
		t.Errorf("expected TagName=v1.0.0, got %s", info.TagName)
	}
}

func TestCheckLatestVersion_OlderRelease(t *testing.T) {
	body := `{
		"tag_name": "v0.9.0",
		"assets": []
	}`
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false when release is older")
	}
}

func TestCheckLatestVersion_DevVersion(t *testing.T) {
	// Dev version with a mocked release should return IsNewer=true.
	body := `{
		"tag_name": "v1.5.0",
		"assets": [
			{"name": "maggus_1.5.0_linux_amd64.tar.gz", "browser_download_url": "https://example.com/linux"},
			{"name": "maggus_1.5.0_windows_amd64.zip", "browser_download_url": "https://example.com/windows"}
		]
	}`
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
	defer cleanup()

	info := CheckLatestVersion("dev")
	if !info.IsNewer {
		t.Error("expected IsNewer=true for dev version with valid release")
	}
	if info.TagName != "v1.5.0" {
		t.Errorf("expected TagName=v1.5.0, got %s", info.TagName)
	}
}

func TestCheckLatestVersion_NetworkError(t *testing.T) {
	origURL := releaseURL
	releaseURL = "http://127.0.0.1:1" // connection refused
	defer func() { releaseURL = origURL }()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false on network error")
	}
	if info.TagName != "" {
		t.Error("expected empty TagName on network error")
	}
}

func TestCheckLatestVersion_MalformedResponse(t *testing.T) {
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false on malformed JSON")
	}
}

func TestCheckLatestVersion_EmptyTagName(t *testing.T) {
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "", "assets": []}`))
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false with empty tag_name")
	}
}

func TestCheckLatestVersion_HTTPError(t *testing.T) {
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer cleanup()

	info := CheckLatestVersion("1.0.0")
	if info.IsNewer {
		t.Error("expected IsNewer=false on HTTP error")
	}
}

func TestSemverNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v2.0.0", "v1.0.0", true},
		{"v1.1.0", "v1.0.0", true},
		{"v1.0.1", "v1.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.0", "v1.0.0", false},
		{"1.2.3", "1.2.2", true},
		{"v1.0.0-rc1", "v0.9.0", true},
		{"invalid", "v1.0.0", false},
		{"v1.0.0", "invalid", false},
	}
	for _, tt := range tests {
		got := semverNewer(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("semverNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestFindAssetURL(t *testing.T) {
	assets := []githubAsset{
		{Name: "maggus_1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
		{Name: "maggus_1.0.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
		{Name: "maggus_1.0.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	tests := []struct {
		goos, goarch string
		want         string
	}{
		{"linux", "amd64", "https://example.com/linux_amd64"},
		{"darwin", "arm64", "https://example.com/darwin_arm64"},
		{"windows", "amd64", "https://example.com/windows_amd64"},
		{"linux", "arm64", ""},   // not in assets
		{"freebsd", "amd64", ""}, // unsupported OS
	}
	for _, tt := range tests {
		got := findAssetURL(assets, tt.goos, tt.goarch)
		if got != tt.want {
			t.Errorf("findAssetURL(%s, %s) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input               string
		major, minor, patch int
		ok                  bool
	}{
		{"v1.2.3", 1, 2, 3, true},
		{"1.2.3", 1, 2, 3, true},
		{"v0.0.0", 0, 0, 0, true},
		{"v1.2.3-rc1", 1, 2, 3, true},
		{"invalid", 0, 0, 0, false},
		{"v1.2", 0, 0, 0, false},
		{"v1.2.x", 0, 0, 0, false},
	}
	for _, tt := range tests {
		major, minor, patch, ok := parseSemver(tt.input)
		if ok != tt.ok || major != tt.major || minor != tt.minor || patch != tt.patch {
			t.Errorf("parseSemver(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
				tt.input, major, minor, patch, ok, tt.major, tt.minor, tt.patch, tt.ok)
		}
	}
}

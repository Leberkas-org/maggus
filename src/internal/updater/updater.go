package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// UpdateInfo holds the result of a version check against GitHub Releases.
type UpdateInfo struct {
	TagName     string
	DownloadURL string
	IsNewer     bool
	Body        string
}

// githubRelease represents the relevant fields from the GitHub Releases API.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset represents a single release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// releaseURL is the GitHub API endpoint. Exported as a var for testing.
var releaseURL = "https://api.github.com/repos/leberkas-org/maggus/releases/latest"

// httpClient is the HTTP client used for API calls. Exported as a var for testing.
var httpClient = &http.Client{Timeout: 5 * time.Second}

// CheckLatestVersion checks GitHub Releases for the latest maggus version.
// It compares the release tag against currentVersion using semver.
// When currentVersion is "dev", any valid release is considered newer.
// Network and parse errors are handled gracefully (returns no-update, nil error).
func CheckLatestVersion(currentVersion string) UpdateInfo {
	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return UpdateInfo{}
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return UpdateInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{}
	}

	if release.TagName == "" {
		return UpdateInfo{}
	}

	var isNewer bool
	if strings.HasPrefix(currentVersion, "dev") {
		// Any valid release is newer than a dev build.
		isNewer = true
	} else {
		isNewer = semverNewer(release.TagName, currentVersion)
	}
	downloadURL := findAssetURL(release.Assets, runtime.GOOS, runtime.GOARCH)

	return UpdateInfo{
		TagName:     release.TagName,
		DownloadURL: downloadURL,
		IsNewer:     isNewer,
		Body:        release.Body,
	}
}

// semverNewer returns true if latest is a newer semver than current.
// Both may have a "v" prefix. Returns false on parse errors.
func semverNewer(latest, current string) bool {
	lMajor, lMinor, lPatch, ok := parseSemver(latest)
	if !ok {
		return false
	}
	cMajor, cMinor, cPatch, ok := parseSemver(current)
	if !ok {
		return false
	}

	if lMajor != cMajor {
		return lMajor > cMajor
	}
	if lMinor != cMinor {
		return lMinor > cMinor
	}
	return lPatch > cPatch
}

// parseSemver parses a version string like "v1.2.3" or "1.2.3" into components.
func parseSemver(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release suffix (e.g. "1.2.3-rc1")
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// findAssetURL finds the download URL for the asset matching the given OS and arch.
// GoReleaser names assets like: maggus_<version>_<os>_<arch>.<ext>
func findAssetURL(assets []githubAsset, goos, goarch string) string {
	// Determine expected file extension
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	// Build the expected suffix pattern: _<os>_<arch>.<ext>
	suffix := fmt.Sprintf("_%s_%s%s", goos, goarch, ext)

	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

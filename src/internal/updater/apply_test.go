package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// createTarGz creates a tar.gz archive containing a single file with the given name and content.
func createTarGz(t *testing.T, fileName string, content []byte) string {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer tmp.Close()

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: fileName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return tmp.Name()
}

// createZip creates a zip archive containing a single file with the given name and content.
func createZip(t *testing.T, fileName string, content []byte) string {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer tmp.Close()

	zw := zip.NewWriter(tmp)
	fw, err := zw.Create(fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return tmp.Name()
}

// setupApplyTest creates a fake executable in a temp dir and overrides osExecutable.
// Returns the fake exe path and a cleanup function.
func setupApplyTest(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	exeName := "maggus"
	if runtime.GOOS == "windows" {
		exeName = "maggus.exe"
	}
	exePath := filepath.Join(dir, exeName)
	if err := os.WriteFile(exePath, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	origExe := osExecutable
	osExecutable = func() (string, error) { return exePath, nil }

	return exePath, func() { osExecutable = origExe }
}

func TestApply_SuccessfulReplacementTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz test only applies to non-Windows")
	}

	exePath, cleanup := setupApplyTest(t)
	defer cleanup()

	newContent := []byte("new-binary-content")
	archivePath := createTarGz(t, "maggus", newContent)
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	if err := Apply(ts.URL + "/maggus_1.0.0_linux_amd64.tar.gz"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Errorf("binary content = %q, want %q", got, newContent)
	}
}

func TestApply_SuccessfulReplacementZip(t *testing.T) {
	exePath, cleanup := setupApplyTest(t)
	defer cleanup()

	binName := "maggus"
	if runtime.GOOS == "windows" {
		binName = "maggus.exe"
	}

	newContent := []byte("new-binary-zip")
	archivePath := createZip(t, binName, newContent)
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	if err := Apply(ts.URL + "/maggus_1.0.0_windows_amd64.zip"); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Errorf("binary content = %q, want %q", got, newContent)
	}

	// On Windows, .old file should exist.
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(exePath + ".old"); os.IsNotExist(err) {
			t.Error("expected .old backup file to exist on Windows")
		}
	}
}

func TestApply_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	// Point osExecutable to a read-only directory.
	dir := t.TempDir()
	exePath := filepath.Join(dir, "maggus")
	if err := os.WriteFile(exePath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	origExe := osExecutable
	osExecutable = func() (string, error) { return exePath, nil }
	defer func() { osExecutable = origExe }()

	// Make directory read-only.
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	err := Apply(ts.URL + "/maggus_1.0.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if err != ErrPermission {
		t.Errorf("expected ErrPermission, got: %v", err)
	}
}

func TestApply_CorruptArchiveTarGz(t *testing.T) {
	_, cleanup := setupApplyTest(t)
	defer cleanup()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not a valid archive"))
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	err := Apply(ts.URL + "/maggus_1.0.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error for corrupt tar.gz")
	}
}

func TestApply_CorruptArchiveZip(t *testing.T) {
	_, cleanup := setupApplyTest(t)
	defer cleanup()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not a valid zip"))
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	err := Apply(ts.URL + "/maggus_1.0.0_windows_amd64.zip")
	if err == nil {
		t.Fatal("expected error for corrupt zip")
	}
}

func TestApply_DownloadHTTPError(t *testing.T) {
	_, cleanup := setupApplyTest(t)
	defer cleanup()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	err := Apply(ts.URL + "/maggus_1.0.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestApply_BinaryNotInArchive(t *testing.T) {
	_, cleanup := setupApplyTest(t)
	defer cleanup()

	// Create a tar.gz with a different filename.
	archivePath := createTarGz(t, "not-maggus", []byte("content"))
	archiveData, _ := os.ReadFile(archivePath)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer ts.Close()

	origClient := httpClient
	httpClient = ts.Client()
	defer func() { httpClient = origClient }()

	err := Apply(ts.URL + "/maggus_1.0.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
}

func TestExtractFromTarGz_ValidArchive(t *testing.T) {
	content := []byte("binary-data")
	archivePath := createTarGz(t, "maggus", content)

	destPath := filepath.Join(t.TempDir(), "maggus-out")
	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz failed: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestExtractFromZip_ValidArchive(t *testing.T) {
	content := []byte("binary-data-zip")
	archivePath := createZip(t, "maggus.exe", content)

	destPath := filepath.Join(t.TempDir(), "maggus-out.exe")
	if err := extractFromZip(archivePath, destPath); err != nil {
		t.Fatalf("extractFromZip failed: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestCleanupOldBinary(t *testing.T) {
	dir := t.TempDir()
	exeName := "maggus"
	if runtime.GOOS == "windows" {
		exeName = "maggus.exe"
	}
	exePath := filepath.Join(dir, exeName)
	os.WriteFile(exePath, []byte("current"), 0755)

	oldPath := exePath + ".old"
	os.WriteFile(oldPath, []byte("old"), 0755)

	origExe := osExecutable
	osExecutable = func() (string, error) { return exePath, nil }
	defer func() { osExecutable = origExe }()

	cleanupOldBinary()

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected .old file to be removed")
	}
}

package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// osExecutable is injectable for testing.
var osExecutable = os.Executable

// ErrPermission is returned when the binary location is not writable.
var ErrPermission = errors.New("permission denied: cannot write to binary location")

// Apply downloads the release asset from downloadURL, extracts the binary,
// and replaces the currently running executable.
func Apply(downloadURL string) error {
	// Clean up any leftover .old file from a previous Windows update.
	if runtime.GOOS == "windows" {
		cleanupOldBinary()
	}

	// Download to temp file.
	tmpArchive, err := downloadToTemp(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpArchive)

	// Determine current executable path.
	exePath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	// Check write permissions by testing the directory.
	exeDir := filepath.Dir(exePath)
	if err := checkWritable(exeDir); err != nil {
		return ErrPermission
	}

	// Extract the binary from the archive to a temp file in the same directory.
	tmpBin := exePath + ".new"
	if err := extractBinary(tmpArchive, tmpBin, downloadURL); err != nil {
		os.Remove(tmpBin)
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Replace the running binary.
	if err := replaceBinary(exePath, tmpBin); err != nil {
		os.Remove(tmpBin)
		return err
	}

	return nil
}

// downloadToTemp downloads the URL to a temporary file and returns its path.
func downloadToTemp(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "maggus-update-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// extractBinary extracts the maggus binary from the archive to destPath.
// Uses the download URL to determine the archive type.
func extractBinary(archivePath, destPath, downloadURL string) error {
	if strings.HasSuffix(downloadURL, ".zip") {
		return extractFromZip(archivePath, destPath)
	}
	if strings.HasSuffix(downloadURL, ".tar.gz") {
		return extractFromTarGz(archivePath, destPath)
	}
	return fmt.Errorf("unsupported archive format: %s", downloadURL)
}

// extractFromTarGz extracts the maggus binary from a tar.gz archive.
func extractFromTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}

		name := filepath.Base(hdr.Name)
		if isMaggusBinary(name) {
			return writeFile(destPath, tr, hdr.FileInfo().Mode())
		}
	}

	return fmt.Errorf("maggus binary not found in archive")
}

// extractFromZip extracts the maggus binary from a zip archive.
func extractFromZip(archivePath, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if isMaggusBinary(name) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeFile(destPath, rc, f.Mode()|0755)
		}
	}

	return fmt.Errorf("maggus binary not found in archive")
}

// isMaggusBinary returns true if the filename looks like the maggus executable.
func isMaggusBinary(name string) bool {
	return name == "maggus" || name == "maggus.exe"
}

// writeFile writes the contents of r to path with the given permissions.
func writeFile(path string, r io.Reader, perm os.FileMode) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return err
	}
	return out.Close()
}

// checkWritable tests if the directory is writable by creating a temp file.
func checkWritable(dir string) error {
	tmp, err := os.CreateTemp(dir, ".maggus-perm-check-*")
	if err != nil {
		return err
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return nil
}

// replaceBinary replaces oldPath with newPath using a platform-appropriate strategy.
func replaceBinary(oldPath, newPath string) error {
	if runtime.GOOS == "windows" {
		return replaceBinaryWindows(oldPath, newPath)
	}
	return replaceBinaryUnix(oldPath, newPath)
}

// replaceBinaryUnix uses atomic rename: rename new over old.
func replaceBinaryUnix(oldPath, newPath string) error {
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	return nil
}

// replaceBinaryWindows renames old to .old, then renames new to the original path.
// The .old file is cleaned up on next run.
func replaceBinaryWindows(oldPath, newPath string) error {
	oldBackup := oldPath + ".old"
	// Remove any existing .old file first.
	os.Remove(oldBackup)

	if err := os.Rename(oldPath, oldBackup); err != nil {
		return fmt.Errorf("cannot rename current binary: %w", err)
	}

	if err := os.Rename(newPath, oldPath); err != nil {
		// Try to restore the old binary.
		os.Rename(oldBackup, oldPath)
		return fmt.Errorf("cannot place new binary: %w", err)
	}

	return nil
}

// cleanupOldBinary removes any leftover .old file from a previous Windows update.
func cleanupOldBinary() {
	exePath, err := osExecutable()
	if err != nil {
		return
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	os.Remove(exePath + ".old")
}

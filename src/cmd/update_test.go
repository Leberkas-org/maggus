package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/updater"
	"github.com/spf13/cobra"
)

func newTestUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "update",
		RunE: func(cmd *cobra.Command, args []string) error { return runUpdate(cmd) },
	}
	cmd.SetOut(&bytes.Buffer{})
	return cmd
}

func TestUpdate_DevVersion(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origPrompt := promptConfirm
	origApply := applyUpdate
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		promptConfirm = origPrompt
		applyUpdate = origApply
	}()
	Version = "dev"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v1.5.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "New release",
		}
	}
	promptConfirm = func(q string) bool { return false }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should proceed with update check, not skip
	if strings.Contains(output, "Skipping") {
		t.Errorf("dev build should not skip update check, got: %s", output)
	}
	// Should show "dev → vX.Y.Z" without "v" prefix on "dev"
	if !strings.Contains(output, "Update available: dev → v1.5.0") {
		t.Errorf("expected 'Update available: dev → v1.5.0', got: %s", output)
	}
}

func TestUpdate_DevVersion_ApplySuccessful(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origApply := applyUpdate
	origPrompt := promptConfirm
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		applyUpdate = origApply
		promptConfirm = origPrompt
	}()

	Version = "dev"
	var appliedURL string
	checkLatestVersion = func(v string) updater.UpdateInfo {
		if v != "dev" {
			t.Errorf("expected currentVersion 'dev', got: %s", v)
		}
		return updater.UpdateInfo{
			TagName:     "v2.1.0",
			DownloadURL: "https://example.com/maggus_v2.1.0.zip",
			IsNewer:     true,
			Body:        "Bug fixes",
		}
	}
	applyUpdate = func(url string) error {
		appliedURL = url
		return nil
	}
	promptConfirm = func(q string) bool { return true }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Update available: dev → v2.1.0") {
		t.Errorf("expected version comparison with 'dev', got: %s", output)
	}
	if !strings.Contains(output, "Bug fixes") {
		t.Errorf("expected changelog, got: %s", output)
	}
	if !strings.Contains(output, "Successfully updated to v2.1.0") {
		t.Errorf("expected success message, got: %s", output)
	}
	if appliedURL != "https://example.com/maggus_v2.1.0.zip" {
		t.Errorf("expected apply to be called with download URL, got: %s", appliedURL)
	}
}

func TestUpdate_AlreadyUpToDate(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
	}()
	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{TagName: "v1.0.0", IsNewer: false}
	}

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Already up to date (v1.0.0)") {
		t.Errorf("expected up-to-date message, got: %s", output)
	}
}

func TestUpdate_UpdateAvailable_Confirmed(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origApply := applyUpdate
	origPrompt := promptConfirm
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		applyUpdate = origApply
		promptConfirm = origPrompt
	}()

	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "Bug fixes and improvements",
		}
	}
	applyUpdate = func(url string) error { return nil }
	promptConfirm = func(q string) bool { return true }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Update available: v1.0.0 → v2.0.0") {
		t.Errorf("expected version comparison, got: %s", output)
	}
	if !strings.Contains(output, "Bug fixes and improvements") {
		t.Errorf("expected changelog, got: %s", output)
	}
	if !strings.Contains(output, "Successfully updated to v2.0.0") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "restart maggus") {
		t.Errorf("expected restart suggestion, got: %s", output)
	}
}

func TestUpdate_UpdateAvailable_Declined(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origPrompt := promptConfirm
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		promptConfirm = origPrompt
	}()

	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
		}
	}
	promptConfirm = func(q string) bool { return false }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Update cancelled") {
		t.Errorf("expected cancellation message, got: %s", output)
	}
}

func TestUpdate_ApplyError(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origApply := applyUpdate
	origPrompt := promptConfirm
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		applyUpdate = origApply
		promptConfirm = origPrompt
	}()

	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
		}
	}
	applyUpdate = func(url string) error { return fmt.Errorf("permission denied") }
	promptConfirm = func(q string) bool { return true }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from apply failure")
	}
	if !strings.Contains(err.Error(), "update failed") {
		t.Errorf("expected 'update failed' error, got: %v", err)
	}
}

func TestUpdate_NoDownloadURL(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origPrompt := promptConfirm
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		promptConfirm = origPrompt
	}()

	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v2.0.0",
			DownloadURL: "", // no asset for this platform
			IsNewer:     true,
		}
	}
	promptConfirm = func(q string) bool { return true }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing download URL")
	}
	if !strings.Contains(err.Error(), "no download available") {
		t.Errorf("expected 'no download available' error, got: %v", err)
	}
}

func TestUpdate_NoChangelog(t *testing.T) {
	origVersion := Version
	origCheck := checkLatestVersion
	origPrompt := promptConfirm
	origApply := applyUpdate
	defer func() {
		Version = origVersion
		checkLatestVersion = origCheck
		promptConfirm = origPrompt
		applyUpdate = origApply
	}()

	Version = "1.0.0"
	checkLatestVersion = func(v string) updater.UpdateInfo {
		return updater.UpdateInfo{
			TagName:     "v1.1.0",
			DownloadURL: "https://example.com/maggus.zip",
			IsNewer:     true,
			Body:        "", // no changelog
		}
	}
	promptConfirm = func(q string) bool { return true }
	applyUpdate = func(url string) error { return nil }

	cmd := newTestUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Changelog:") {
		t.Errorf("should not show changelog section when body is empty, got: %s", output)
	}
	if !strings.Contains(output, "Successfully updated to v1.1.0") {
		t.Errorf("expected success message, got: %s", output)
	}
}

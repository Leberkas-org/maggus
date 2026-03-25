package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
	"github.com/spf13/cobra"
)

// newTestCmd creates a cobra.Command with captured stdout/stderr for testing.
func newTestCmd(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

// setupApproveDir creates a temp dir with .maggus/features/ for approve/unapprove tests.
func setupApproveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeApproveFeature writes a minimal feature file with a maggus-id into .maggus/features/.
func writeApproveFeature(t *testing.T, dir, filename, maggusID string) {
	t.Helper()
	var content string
	if maggusID != "" {
		content = fmt.Sprintf("<!-- maggus-id: %s -->\n# Feature\n\n### TASK-001-001: Sample\n- [ ] Do something\n", maggusID)
	} else {
		content = "# Feature\n\n### TASK-001-001: Sample\n- [ ] Do something\n"
	}
	if err := os.WriteFile(filepath.Join(dir, ".maggus", "features", filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const testUUID = "d2062a56-007c-47cd-8e7b-ba3d2e361689"

func TestRunApprove_ApprovesByID(t *testing.T) {
	dir := setupApproveDir(t)
	writeApproveFeature(t, dir, "feature_001.md", testUUID)

	cmd, stdout, _ := newTestCmd(t)
	if err := runApprove(cmd, dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Approval should be persisted under the UUID key
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !a[testUUID] {
		t.Error("expected UUID key to be approved in feature_approvals.yml")
	}
	if !strings.Contains(stdout.String(), "Approved") {
		t.Errorf("expected confirmation message, got: %s", stdout.String())
	}
}

func TestRunApprove_AlreadyApproved(t *testing.T) {
	dir := setupApproveDir(t)
	writeApproveFeature(t, dir, "feature_001.md", testUUID)
	if err := approval.Approve(dir, testUUID); err != nil {
		t.Fatal(err)
	}

	cmd, stdout, _ := newTestCmd(t)
	if err := runApprove(cmd, dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "already approved") {
		t.Errorf("expected 'already approved' message, got: %s", stdout.String())
	}
}

func TestRunApprove_FeatureNotFound(t *testing.T) {
	dir := setupApproveDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runApprove(cmd, dir, "feature_099")
	if err == nil {
		t.Fatal("expected error for non-existent feature")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestRunApprove_FallbackToFilename(t *testing.T) {
	dir := setupApproveDir(t)
	writeApproveFeature(t, dir, "feature_001.md", "") // no maggus-id

	cmd, stdout, _ := newTestCmd(t)
	if err := runApprove(cmd, dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !a["feature_001"] {
		t.Error("expected filename fallback key to be approved")
	}
	if !strings.Contains(stdout.String(), "Approved") {
		t.Errorf("expected confirmation message, got: %s", stdout.String())
	}
}

func TestRunUnapprove_UnapprovesID(t *testing.T) {
	dir := setupApproveDir(t)
	writeApproveFeature(t, dir, "feature_001.md", testUUID)
	if err := approval.Approve(dir, testUUID); err != nil {
		t.Fatal(err)
	}

	cmd, stdout, _ := newTestCmd(t)
	if err := runUnapprove(cmd, dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a[testUUID] {
		t.Error("expected UUID key to be unapproved")
	}
	if !strings.Contains(stdout.String(), "Unapproved") {
		t.Errorf("expected confirmation message, got: %s", stdout.String())
	}
}

func TestRunUnapprove_NotApproved(t *testing.T) {
	dir := setupApproveDir(t)
	writeApproveFeature(t, dir, "feature_001.md", testUUID)

	cmd, stdout, _ := newTestCmd(t)
	if err := runUnapprove(cmd, dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "not approved") {
		t.Errorf("expected 'not approved' message, got: %s", stdout.String())
	}
}

func TestRunUnapprove_FeatureNotFound(t *testing.T) {
	dir := setupApproveDir(t)

	cmd, _, _ := newTestCmd(t)
	err := runUnapprove(cmd, dir, "feature_099")
	if err == nil {
		t.Fatal("expected error for non-existent feature")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

package approval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/approval"
)

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	a, err := approval.Load(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(a) != 0 {
		t.Errorf("expected empty approvals, got %v", a)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "feature_001: true\nfeature_002: false\n"
	if err := os.WriteFile(filepath.Join(maggusDir, "feature_approvals.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a["feature_001"] {
		t.Error("expected feature_001 to be approved")
	}
	if a["feature_002"] {
		t.Error("expected feature_002 to be unapproved")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(maggusDir, "feature_approvals.yml"), []byte(": invalid: yaml: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := approval.Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSave_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"feature_001": true, "feature_002": false}
	if err := approval.Save(dir, a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if !loaded["feature_001"] {
		t.Error("expected feature_001 to be approved after save+load")
	}
	if loaded["feature_002"] {
		t.Error("expected feature_002 to be unapproved after save+load")
	}
}

func TestApprove(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := approval.Approve(dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !a["feature_001"] {
		t.Error("expected feature_001 to be approved")
	}
}

func TestUnapprove(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	// First approve, then unapprove
	if err := approval.Approve(dir, "feature_001"); err != nil {
		t.Fatal(err)
	}
	if err := approval.Unapprove(dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a["feature_001"] {
		t.Error("expected feature_001 to be unapproved")
	}
}

func TestRemove_ExistingEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"feature_001": true, "feature_002": false}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	if err := approval.Remove(dir, "feature_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["feature_001"]; ok {
		t.Error("expected feature_001 to be removed")
	}
	if !loaded["feature_002"] || loaded["feature_002"] {
		// feature_002 is explicitly false — check it still exists
		if _, ok := loaded["feature_002"]; !ok {
			t.Error("expected feature_002 to still be present")
		}
	}
}

func TestRemove_AbsentEntryIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".maggus"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"feature_001": true}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, ".maggus", "feature_approvals.yml")
	infoBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := approval.Remove(dir, "feature_002"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infoAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error("expected file to not be rewritten when entry was absent")
	}
}

func TestPrune_RemovesStaleEntries(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Set up approvals with some stale entries
	a := approval.Approvals{"uuid-1": true, "uuid-2": false, "uuid-stale": true}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	// Prune with only uuid-1 and uuid-2 as known
	if err := approval.Prune(dir, []string{"uuid-1", "uuid-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["uuid-stale"]; ok {
		t.Error("expected uuid-stale to be pruned")
	}
	if !loaded["uuid-1"] {
		t.Error("expected uuid-1 to remain approved")
	}
	if loaded["uuid-2"] {
		t.Error("expected uuid-2 to remain unapproved (false)")
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(loaded))
	}
}

func TestPrune_NoOpWhenAllKnown(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"uuid-1": true, "uuid-2": false}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	// Get file mod time before prune
	path := filepath.Join(maggusDir, "feature_approvals.yml")
	infoBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Prune with all IDs known — should not rewrite file
	if err := approval.Prune(dir, []string{"uuid-1", "uuid-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infoAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error("expected file to not be rewritten when nothing was pruned")
	}
}

func TestPrune_EmptyKnownIDsIsNoOp(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"uuid-1": true, "uuid-2": true}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	// Prune with empty knownIDs — should not remove anything
	if err := approval.Prune(dir, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 entries preserved, got %d", len(loaded))
	}
}

func TestPrune_NilKnownIDsIsNoOp(t *testing.T) {
	dir := t.TempDir()
	maggusDir := filepath.Join(dir, ".maggus")
	if err := os.MkdirAll(maggusDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := approval.Approvals{"uuid-1": true}
	if err := approval.Save(dir, a); err != nil {
		t.Fatal(err)
	}

	if err := approval.Prune(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := approval.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 entry preserved, got %d", len(loaded))
	}
}

func TestIsApproved_OptIn(t *testing.T) {
	tests := []struct {
		name      string
		approvals approval.Approvals
		featureID string
		want      bool
	}{
		{"approved feature", approval.Approvals{"feature_001": true}, "feature_001", true},
		{"unapproved feature (explicit false)", approval.Approvals{"feature_001": false}, "feature_001", false},
		{"missing feature defaults to false", approval.Approvals{}, "feature_001", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := approval.IsApproved(tt.approvals, tt.featureID, true)
			if got != tt.want {
				t.Errorf("IsApproved(%v, %q, opt-in) = %v, want %v", tt.approvals, tt.featureID, got, tt.want)
			}
		})
	}
}

func TestIsApproved_OptOut(t *testing.T) {
	tests := []struct {
		name      string
		approvals approval.Approvals
		featureID string
		want      bool
	}{
		{"approved feature", approval.Approvals{"feature_001": true}, "feature_001", true},
		{"unapproved feature (explicit false)", approval.Approvals{"feature_001": false}, "feature_001", false},
		{"missing feature defaults to true in opt-out", approval.Approvals{}, "feature_001", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := approval.IsApproved(tt.approvals, tt.featureID, false)
			if got != tt.want {
				t.Errorf("IsApproved(%v, %q, opt-out) = %v, want %v", tt.approvals, tt.featureID, got, tt.want)
			}
		})
	}
}

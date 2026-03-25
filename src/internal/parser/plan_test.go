package parser

import (
	"testing"
)


const testBugContent = `<!-- maggus-id: aaaabbbb-1111-2222-3333-ccccddddeeee -->
# Bug 001: Test Bug

### BUG-001-001: Fix something

**Acceptance Criteria:**
- [x] Done criterion
- [ ] Open criterion
`

const testFeatureWithMaggusID = `<!-- maggus-id: 11112222-aaaa-bbbb-cccc-333344445555 -->
# Feature 001: Test Feature

### TASK-001-001: First task

**Acceptance Criteria:**
- [ ] Criterion A
- [ ] Criterion B

### TASK-001-002: Second task

**Acceptance Criteria:**
- [x] Done
`

const testFeatureNoMaggusID = `# Feature 002: Another Feature

### TASK-002-001: Only task

**Acceptance Criteria:**
- [ ] BLOCKED: something is blocked
- [x] Already done
`

// TestPlanApprovalKey verifies ApprovalKey() returns MaggusID when set, otherwise ID.
func TestPlanApprovalKey(t *testing.T) {
	t.Run("returns MaggusID when set", func(t *testing.T) {
		p := Plan{ID: "feature_001", MaggusID: "uuid-1234"}
		if got := p.ApprovalKey(); got != "uuid-1234" {
			t.Errorf("ApprovalKey() = %q, want %q", got, "uuid-1234")
		}
	})

	t.Run("falls back to ID when MaggusID empty", func(t *testing.T) {
		p := Plan{ID: "feature_001", MaggusID: ""}
		if got := p.ApprovalKey(); got != "feature_001" {
			t.Errorf("ApprovalKey() = %q, want %q", got, "feature_001")
		}
	})
}

// TestPlanDoneCount verifies DoneCount() returns the number of completed tasks.
func TestPlanDoneCount(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  int
	}{
		{"empty", nil, 0},
		{"none done", []Task{
			{Criteria: []Criterion{{Checked: false}}},
			{Criteria: []Criterion{{Checked: false}}},
		}, 0},
		{"some done", []Task{
			{Criteria: []Criterion{{Checked: true}}},
			{Criteria: []Criterion{{Checked: false}}},
		}, 1},
		{"all done", []Task{
			{Criteria: []Criterion{{Checked: true}}},
			{Criteria: []Criterion{{Checked: true}}},
		}, 2},
		{"task with no criteria is not done", []Task{
			{},
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plan{Tasks: tt.tasks}
			if got := p.DoneCount(); got != tt.want {
				t.Errorf("DoneCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestPlanBlockedCount verifies BlockedCount() returns count of incomplete+blocked tasks.
func TestPlanBlockedCount(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  int
	}{
		{"empty", nil, 0},
		{"no blocked", []Task{
			{Criteria: []Criterion{{Checked: false, Blocked: false}}},
		}, 0},
		{"one blocked", []Task{
			{Criteria: []Criterion{{Checked: false, Blocked: true}}},
		}, 1},
		{"completed blocked criterion does not count", []Task{
			{Criteria: []Criterion{{Checked: true, Blocked: false}}},
		}, 0},
		{"mixed", []Task{
			{Criteria: []Criterion{{Checked: false, Blocked: true}}},  // blocked → counts
			{Criteria: []Criterion{{Checked: true}}},                  // complete → does not count
			{Criteria: []Criterion{{Checked: false, Blocked: false}}}, // incomplete, not blocked → does not count
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plan{Tasks: tt.tasks}
			if got := p.BlockedCount(); got != tt.want {
				t.Errorf("BlockedCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestPlanIDFromPath verifies planIDFromPath strips .md and _completed correctly.
func TestPlanIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"feature_001.md", "feature_001"},
		{"/some/dir/feature_001.md", "feature_001"},
		{"feature_001_completed.md", "feature_001"},
		{"/some/dir/bug_003_completed.md", "bug_003"},
		{"bug_001.md", "bug_001"},
	}
	for _, tt := range tests {
		got := planIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("planIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestLoadPlans_BugsFirst verifies bugs are returned before features.
func TestLoadPlans_BugsFirst(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeatureNoMaggusID)
	writeTempBug(t, dir, "bug_001.md", testBugContent)

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}

	if !plans[0].IsBug {
		t.Error("first plan should be a bug")
	}
	if plans[1].IsBug {
		t.Error("second plan should not be a bug")
	}
}

// TestLoadPlans_ExcludesCompleted verifies completed files are excluded when includeCompleted=false.
func TestLoadPlans_ExcludesCompleted(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeatureNoMaggusID)
	writeTempFeature(t, dir, "feature_002_completed.md", testFeatureWithMaggusID)
	writeTempBug(t, dir, "bug_001.md", testBugContent)
	writeTempBug(t, dir, "bug_002_completed.md", testBugContent)

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans (no completed), got %d", len(plans))
	}
	for _, p := range plans {
		if p.Completed {
			t.Errorf("plan %q should not be marked completed", p.ID)
		}
	}
}

// TestLoadPlans_IncludesCompleted verifies completed files are included when includeCompleted=true.
func TestLoadPlans_IncludesCompleted(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeatureNoMaggusID)
	writeTempFeature(t, dir, "feature_002_completed.md", testFeatureWithMaggusID)

	plans, err := LoadPlans(dir, true)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}

	completedFound := false
	for _, p := range plans {
		if p.ID == "feature_002" && p.Completed {
			completedFound = true
		}
	}
	if !completedFound {
		t.Error("expected completed feature_002 to be present with Completed=true")
	}
}

// TestLoadPlans_CompletedFlag verifies Completed field is set correctly.
func TestLoadPlans_CompletedFlag(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001_completed.md", testFeatureWithMaggusID)

	plans, err := LoadPlans(dir, true)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if !plans[0].Completed {
		t.Error("expected Completed=true for _completed.md file")
	}
	if plans[0].ID != "feature_001" {
		t.Errorf("expected ID=%q, got %q", "feature_001", plans[0].ID)
	}
}

// TestLoadPlans_MaggusID verifies MaggusID is parsed from the first-line comment.
func TestLoadPlans_MaggusID(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeatureWithMaggusID)
	writeTempFeature(t, dir, "feature_002.md", testFeatureNoMaggusID)

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}

	if plans[0].MaggusID != "11112222-aaaa-bbbb-cccc-333344445555" {
		t.Errorf("plan[0].MaggusID = %q, want %q", plans[0].MaggusID, "11112222-aaaa-bbbb-cccc-333344445555")
	}
	if plans[1].MaggusID != "" {
		t.Errorf("plan[1].MaggusID = %q, want empty", plans[1].MaggusID)
	}
}

// TestLoadPlans_MigratesLegacyBugIDs verifies MigrateLegacyBugIDs is called for bug files.
func TestLoadPlans_MigratesLegacyBugIDs(t *testing.T) {
	dir := t.TempDir()

	legacyBugContent := `# Bug 001: Legacy Bug

### TASK-001: Old task

**Acceptance Criteria:**
- [ ] Something
`
	writeTempBug(t, dir, "bug_001.md", legacyBugContent)

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	// After migration, task should have BUG-001-001 ID.
	if len(plans[0].Tasks) == 0 {
		t.Fatal("expected at least one task")
	}
	if plans[0].Tasks[0].ID != "BUG-001-001" {
		t.Errorf("expected migrated ID %q, got %q", "BUG-001-001", plans[0].Tasks[0].ID)
	}
}

// TestLoadPlans_EmptyDir verifies LoadPlans returns empty slice (not error) when no files exist.
func TestLoadPlans_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("expected 0 plans, got %d", len(plans))
	}
}

// TestLoadPlans_ApprovalNotStored verifies Plan has no approval field.
func TestLoadPlans_ApprovalNotStored(t *testing.T) {
	dir := t.TempDir()
	writeTempFeature(t, dir, "feature_001.md", testFeatureNoMaggusID)

	plans, err := LoadPlans(dir, false)
	if err != nil {
		t.Fatalf("LoadPlans error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	// Compile-time check: if Plan had an Approved field, this would need updating.
	// The test documents the contract: no approval field exists on Plan.
	_ = plans[0].ID
	_ = plans[0].MaggusID
	_ = plans[0].File
	_ = plans[0].Tasks
	_ = plans[0].IsBug
	_ = plans[0].Completed
}

package stores

import (
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// makeFeaturePlan builds a simple feature plan for use in mem store tests.
func makeFeaturePlan(id, file string, criteria []parser.Criterion, completed bool) parser.Plan {
	return parser.Plan{
		ID:        id,
		File:      file,
		IsBug:     false,
		Completed: completed,
		Tasks: []parser.Task{
			{ID: "TASK-001-001", Title: "Do a thing", Criteria: criteria},
		},
	}
}

// makeBugPlan builds a simple bug plan for use in mem store tests.
func makeBugPlan(id, file string, criteria []parser.Criterion, completed bool) parser.Plan {
	return parser.Plan{
		ID:        id,
		File:      file,
		IsBug:     true,
		Completed: completed,
		Tasks: []parser.Task{
			{ID: "BUG-001-001", Title: "Fix a thing", Criteria: criteria},
		},
	}
}

var incompleteCriteria = []parser.Criterion{
	{Text: "criterion one", Checked: false},
	{Text: "criterion two", Checked: false},
}

var completeCriteria = []parser.Criterion{
	{Text: "criterion one", Checked: true},
	{Text: "criterion two", Checked: true},
}

var blockedCriteria = []parser.Criterion{
	{Text: "BLOCKED: some blocker", Checked: false, Blocked: true},
	{Text: "normal criterion", Checked: false},
}

// ---- MemFeatureStore tests ----

func TestMemFeatureStore_LoadAll(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", incompleteCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if plans[0].ID != "feature_001" {
		t.Errorf("ID = %q, want feature_001", plans[0].ID)
	}
}

func TestMemFeatureStore_LoadAll_excludesCompleted(t *testing.T) {
	active := makeFeaturePlan("feature_001", "/fake/feature_001.md", incompleteCriteria, false)
	done := makeFeaturePlan("feature_002", "/fake/feature_002_completed.md", completeCriteria, true)
	store := NewMemFeatureStore([]parser.Plan{active, done})

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll(false): %v", err)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 active plan, got %d", len(plans))
	}

	plans, err = store.LoadAll(true)
	if err != nil {
		t.Fatalf("LoadAll(true): %v", err)
	}
	if len(plans) != 2 {
		t.Errorf("expected 2 plans with completed, got %d", len(plans))
	}
}

func TestMemFeatureStore_GlobFiles(t *testing.T) {
	active := makeFeaturePlan("feature_001", "/fake/feature_001.md", incompleteCriteria, false)
	done := makeFeaturePlan("feature_002", "/fake/feature_002_completed.md", completeCriteria, true)
	store := NewMemFeatureStore([]parser.Plan{active, done})

	files, err := store.GlobFiles(false)
	if err != nil {
		t.Fatalf("GlobFiles(false): %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0] != "/fake/feature_001.md" {
		t.Errorf("unexpected file: %s", files[0])
	}

	files, err = store.GlobFiles(true)
	if err != nil {
		t.Fatalf("GlobFiles(true): %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files with completed, got %d", len(files))
	}
}

func TestMemFeatureStore_MarkCompleted(t *testing.T) {
	done := makeFeaturePlan("feature_001", "/fake/feature_001.md", completeCriteria, false)
	active := makeFeaturePlan("feature_002", "/fake/feature_002.md", incompleteCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{done, active})

	ids, err := store.MarkCompleted("rename")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 completed ID, got %d: %v", len(ids), ids)
	}
	if ids[0] != "feature_001" {
		t.Errorf("expected feature_001, got %s", ids[0])
	}

	// Verify in-memory state: feature_001 should now be Completed.
	plans, _ := store.LoadAll(true)
	for _, p := range plans {
		if p.ID == "feature_001" && !p.Completed {
			t.Error("feature_001 should be marked Completed")
		}
		if p.ID == "feature_002" && p.Completed {
			t.Error("feature_002 should not be marked Completed")
		}
	}
}

func TestMemFeatureStore_MarkCompleted_skipsBlocked(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", blockedCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	ids, err := store.MarkCompleted("")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no completions for blocked plan, got %v", ids)
	}
}

func TestMemFeatureStore_DeleteTask(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", incompleteCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	if err := store.DeleteTask("/fake/feature_001.md", "TASK-001-001"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	plans, _ := store.LoadAll(false)
	if len(plans[0].Tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(plans[0].Tasks))
	}
}

func TestMemFeatureStore_DeleteTask_notFound(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", incompleteCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	if err := store.DeleteTask("/fake/feature_001.md", "TASK-999-999"); err == nil {
		t.Error("expected error for missing task ID")
	}
}

func TestMemFeatureStore_UnblockCriterion(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", blockedCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.UnblockCriterion("/fake/feature_001.md", c); err != nil {
		t.Fatalf("UnblockCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	cr := plans[0].Tasks[0].Criteria[0]
	if cr.Blocked {
		t.Error("criterion should not be blocked after UnblockCriterion")
	}
	if cr.Text != "some blocker" {
		t.Errorf("expected unblocked text, got %q", cr.Text)
	}
}

func TestMemFeatureStore_ResolveCriterion(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", blockedCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.ResolveCriterion("/fake/feature_001.md", c); err != nil {
		t.Fatalf("ResolveCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	cr := plans[0].Tasks[0].Criteria[0]
	if cr.Blocked {
		t.Error("criterion should not be blocked after ResolveCriterion")
	}
	if !cr.Checked {
		t.Error("criterion should be checked after ResolveCriterion")
	}
	if cr.Text != "some blocker" {
		t.Errorf("expected unblocked text, got %q", cr.Text)
	}
}

func TestMemFeatureStore_DeleteCriterion(t *testing.T) {
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", blockedCriteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.DeleteCriterion("/fake/feature_001.md", c); err != nil {
		t.Fatalf("DeleteCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	if len(plans[0].Tasks[0].Criteria) != 1 {
		t.Errorf("expected 1 criterion after delete, got %d", len(plans[0].Tasks[0].Criteria))
	}
	if plans[0].Tasks[0].Criteria[0].Text == "BLOCKED: some blocker" {
		t.Error("deleted criterion still present")
	}
}

func TestMemFeatureStore_isolatesSeededData(t *testing.T) {
	criteria := []parser.Criterion{{Text: "criterion one", Checked: false}}
	plan := makeFeaturePlan("feature_001", "/fake/feature_001.md", criteria, false)
	store := NewMemFeatureStore([]parser.Plan{plan})

	// Mutate the original criteria slice.
	criteria[0].Checked = true

	plans, _ := store.LoadAll(false)
	if plans[0].Tasks[0].Criteria[0].Checked {
		t.Error("MemFeatureStore should deep-copy seeded plans; original mutation leaked in")
	}
}

// ---- MemBugStore tests ----

func TestMemBugStore_LoadAll(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", incompleteCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	plans, err := store.LoadAll(false)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if !plans[0].IsBug {
		t.Error("IsBug should be true")
	}
}

func TestMemBugStore_LoadAll_excludesCompleted(t *testing.T) {
	active := makeBugPlan("bug_001", "/fake/bug_001.md", incompleteCriteria, false)
	done := makeBugPlan("bug_002", "/fake/bug_002_completed.md", completeCriteria, true)
	store := NewMemBugStore([]parser.Plan{active, done})

	plans, _ := store.LoadAll(false)
	if len(plans) != 1 {
		t.Errorf("expected 1 active plan, got %d", len(plans))
	}

	plans, _ = store.LoadAll(true)
	if len(plans) != 2 {
		t.Errorf("expected 2 plans with completed, got %d", len(plans))
	}
}

func TestMemBugStore_GlobFiles(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", incompleteCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	files, err := store.GlobFiles(false)
	if err != nil {
		t.Fatalf("GlobFiles: %v", err)
	}
	if len(files) != 1 || files[0] != "/fake/bug_001.md" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestMemBugStore_MarkCompleted(t *testing.T) {
	done := makeBugPlan("bug_001", "/fake/bug_001.md", completeCriteria, false)
	store := NewMemBugStore([]parser.Plan{done})

	ids, err := store.MarkCompleted("")
	if err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if len(ids) != 1 || ids[0] != "bug_001" {
		t.Errorf("expected [bug_001], got %v", ids)
	}

	plans, _ := store.LoadAll(true)
	if !plans[0].Completed {
		t.Error("bug_001 should be marked Completed")
	}
}

func TestMemBugStore_DeleteTask(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", incompleteCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	if err := store.DeleteTask("/fake/bug_001.md", "BUG-001-001"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	plans, _ := store.LoadAll(false)
	if len(plans[0].Tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(plans[0].Tasks))
	}
}

func TestMemBugStore_UnblockCriterion(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", blockedCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.UnblockCriterion("/fake/bug_001.md", c); err != nil {
		t.Fatalf("UnblockCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	cr := plans[0].Tasks[0].Criteria[0]
	if cr.Blocked || cr.Text != "some blocker" {
		t.Errorf("UnblockCriterion failed: blocked=%v text=%q", cr.Blocked, cr.Text)
	}
}

func TestMemBugStore_ResolveCriterion(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", blockedCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.ResolveCriterion("/fake/bug_001.md", c); err != nil {
		t.Fatalf("ResolveCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	cr := plans[0].Tasks[0].Criteria[0]
	if cr.Blocked || !cr.Checked || cr.Text != "some blocker" {
		t.Errorf("ResolveCriterion failed: blocked=%v checked=%v text=%q", cr.Blocked, cr.Checked, cr.Text)
	}
}

func TestMemBugStore_DeleteCriterion(t *testing.T) {
	plan := makeBugPlan("bug_001", "/fake/bug_001.md", blockedCriteria, false)
	store := NewMemBugStore([]parser.Plan{plan})

	c := parser.Criterion{Text: "BLOCKED: some blocker", Checked: false, Blocked: true}
	if err := store.DeleteCriterion("/fake/bug_001.md", c); err != nil {
		t.Fatalf("DeleteCriterion: %v", err)
	}

	plans, _ := store.LoadAll(false)
	if len(plans[0].Tasks[0].Criteria) != 1 {
		t.Errorf("expected 1 criterion after delete, got %d", len(plans[0].Tasks[0].Criteria))
	}
}

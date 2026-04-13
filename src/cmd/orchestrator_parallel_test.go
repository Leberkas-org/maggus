package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

func TestOrchestratorClassifyWorkable(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{"TASK-001": true}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: true}}},
		{ID: "TASK-002", Parallel: true, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-003", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-004", Parallel: true, Predecessors: []string{"TASK-002"}, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-005", Parallel: true, Predecessors: nil, Criteria: []parser.Criterion{{Text: "BLOCKED: something", Blocked: true}}},
	}

	par, seq := o.classifyWorkable(tasks)

	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-003" {
		t.Errorf("sequential workable = %v, want [TASK-003]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_NoPredecessors(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 2 {
		t.Errorf("parallel workable count = %d, want 2", len(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestOrchestratorClassifyWorkable_SkipsFailedTasks(t *testing.T) {
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel workable = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 0 {
		t.Errorf("sequential workable count = %d, want 0", len(seq))
	}
}

func TestOrchestratorClassifyWorkable_MixedBatch(t *testing.T) {
	// Verify that when both parallel and sequential tasks are eligible,
	// classifyWorkable places them in the correct lists.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-003", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 2 {
		t.Errorf("parallel workable count = %d, want 2", len(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_BlockedPredecessorSatisfied(t *testing.T) {
	// Task 1 is blocked. Task 2 has Task 1 as predecessor.
	// A blocked predecessor must be treated as satisfied so Task 2 can run.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Text: "BLOCKED: something", Blocked: true}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_SkippedPredecessorSatisfied(t *testing.T) {
	// Task 1 is skipped. Task 2 has Task 1 as predecessor.
	// A skipped predecessor must be treated as satisfied so Task 2 can run.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"TASK-001": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Skipped: true}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable = %v, want [TASK-002]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_IncompletePredecessorStillHeld(t *testing.T) {
	// Task 1 is incomplete (not blocked, not skipped, not complete).
	// Task 2 must still be held back — only truly unsatisfied predecessors block.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-001"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-001" {
		t.Errorf("sequential workable = %v, want [TASK-001]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_UnknownPredecessorTreatedAsSatisfied(t *testing.T) {
	// NONEXISTENT-ID is not a real task in the group. runGroupTasks pre-adds it
	// to skippedOrBlockedIDs so the task is not permanently blocked.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"NONEXISTENT-ID": true}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: false, Predecessors: []string{"NONEXISTENT-ID"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-001" {
		t.Errorf("sequential workable = %v, want [TASK-001]", taskIDs(seq))
	}
}

func TestOrchestratorClassifyWorkable_MixedKnownUnknownPredecessors(t *testing.T) {
	// TASK-A is a real predecessor that must complete first.
	// NONEXISTENT-ID is unknown and pre-populated as satisfied by runGroupTasks.
	// TASK-002 should be held back until TASK-A completes, then become runnable.
	o := &Orchestrator{}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{"NONEXISTENT-ID": true}

	tasks := []parser.Task{
		{ID: "TASK-A", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-A", "NONEXISTENT-ID"}, Criteria: []parser.Criterion{{Checked: false}}},
	}

	// Before TASK-A completes: only TASK-A is runnable.
	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("parallel workable = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-A" {
		t.Errorf("sequential workable before TASK-A completes = %v, want [TASK-A]", taskIDs(seq))
	}

	// After TASK-A completes: update both completedIDs and the task struct
	// (as the real orchestrator does by re-parsing the feature file after commit).
	o.completedIDs["TASK-A"] = true
	tasksAfterA := []parser.Task{
		{ID: "TASK-A", Parallel: false, Criteria: []parser.Criterion{{Checked: true}}},
		{ID: "TASK-002", Parallel: false, Predecessors: []string{"TASK-A", "NONEXISTENT-ID"}, Criteria: []parser.Criterion{{Checked: false}}},
	}
	par, seq = o.classifyWorkable(tasksAfterA)
	if len(par) != 0 {
		t.Errorf("parallel workable after TASK-A completes = %v, want []", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-002" {
		t.Errorf("sequential workable after TASK-A completes = %v, want [TASK-002]", taskIDs(seq))
	}
}

// ─── Cross-feature predecessor blocking ──────────────────────────────────────

// crossFeatureFixture creates a temp dir with .maggus/features/ and writes
// feature files as specified. Returns the project root dir.
func crossFeatureFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(featDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestClassifyWorkable_CrossFeatureBlocksUnsatisfied(t *testing.T) {
	// feature_006.md has an incomplete task → Feature 006 is not complete.
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: Pending\n**Acceptance Criteria:**\n- [ ] Do it\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("expected no parallel tasks, got %v", taskIDs(par))
	}
	if len(seq) != 0 {
		t.Errorf("expected no sequential tasks (blocked by cross-feature dep), got %v", taskIDs(seq))
	}
}

func TestClassifyWorkable_CrossFeatureAllowsSatisfied(t *testing.T) {
	// feature_006_completed.md exists → Feature 006 is complete.
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006_completed.md": "# Feature 006\n## Tasks\n### TASK-006-001: Done\n**Acceptance Criteria:**\n- [x] Done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 0 {
		t.Errorf("expected no parallel tasks, got %v", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-014-001" {
		t.Errorf("expected [TASK-014-001] sequential (cross-feature satisfied), got %v", taskIDs(seq))
	}
}

func TestClassifyWorkable_NoCrossFeatureDepsUnaffected(t *testing.T) {
	// Tasks with no cross-feature deps should behave exactly as before.
	dir := crossFeatureFixture(t, nil)

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{ID: "TASK-001", Parallel: false, Criteria: []parser.Criterion{{Checked: false}}},
		{ID: "TASK-002", Parallel: true, Criteria: []parser.Criterion{{Checked: false}}},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par) != 1 || par[0].ID != "TASK-002" {
		t.Errorf("parallel = %v, want [TASK-002]", taskIDs(par))
	}
	if len(seq) != 1 || seq[0].ID != "TASK-001" {
		t.Errorf("sequential = %v, want [TASK-001]", taskIDs(seq))
	}
}

func TestClassifyWorkable_CrossFeatureRangePartiallyBlocked(t *testing.T) {
	// Range: Features 004-006. Feature 004 and 005 are complete, but 006 is not.
	dir := crossFeatureFixture(t, map[string]string{
		"feature_004_completed.md": "# complete\n",
		"feature_005_completed.md": "# complete\n",
		"feature_006.md":           "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{4, 5, 6}, Label: "dependencies"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(par)+len(seq) != 0 {
		t.Errorf("expected 0 workable (range partially incomplete), got par=%v seq=%v", taskIDs(par), taskIDs(seq))
	}
}

func TestClassifyWorkable_MissingFeatureTreatedAsComplete(t *testing.T) {
	// Feature 099 doesn't exist at all — treat as complete per spec.
	dir := crossFeatureFixture(t, nil)

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{99}}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	par, seq := o.classifyWorkable(tasks)
	if len(seq) != 1 || seq[0].ID != "TASK-014-001" {
		t.Errorf("expected [TASK-014-001] runnable (missing feature = complete), got par=%v seq=%v", taskIDs(par), taskIDs(seq))
	}
}

// ─── countWorkable with cross-feature blocking ──────────────────────────────

func TestCountWorkable_CrossFeatureBlocking(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})
	featureDir := filepath.Join(dir, ".maggus", "features")

	tasks := []parser.Task{
		{ID: "TASK-001", Criteria: []parser.Criterion{{Checked: false}}}, // no cross-feature deps
		{
			ID:                      "TASK-002",
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	// Without featureDir: both tasks counted.
	if got := countWorkable(tasks, nil, nil, ""); got != 2 {
		t.Errorf("countWorkable without featureDir = %d, want 2", got)
	}
	// With featureDir: only TASK-001 counted (TASK-002 blocked on Feature 006).
	if got := countWorkable(tasks, nil, nil, featureDir); got != 1 {
		t.Errorf("countWorkable with featureDir = %d, want 1", got)
	}
}

// ─── firstWorkableTask with cross-feature blocking ──────────────────────────

func TestFirstWorkableTask_CrossFeatureBlocking(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})
	featureDir := filepath.Join(dir, ".maggus", "features")

	plans := []parser.Plan{
		{
			Tasks: []parser.Task{
				{
					ID:                      "TASK-002",
					CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}}},
					Criteria:                []parser.Criterion{{Checked: false}},
				},
				{ID: "TASK-003", Criteria: []parser.Criterion{{Checked: false}}},
			},
		},
	}

	// Without featureDir: TASK-002 is returned (first in list).
	got := firstWorkableTask(plans, nil, nil, "")
	if got == nil || got.ID != "TASK-002" {
		t.Errorf("firstWorkableTask without featureDir: got %v, want TASK-002", got)
	}

	// With featureDir: TASK-002 is blocked, TASK-003 returned.
	got = firstWorkableTask(plans, nil, nil, featureDir)
	if got == nil || got.ID != "TASK-003" {
		t.Errorf("firstWorkableTask with featureDir: got %v, want TASK-003", got)
	}
}

func TestFirstWorkableTask_AllCrossFeatureBlocked(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})
	featureDir := filepath.Join(dir, ".maggus", "features")

	plans := []parser.Plan{
		{
			Tasks: []parser.Task{
				{
					ID:                      "TASK-002",
					CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}}},
					Criteria:                []parser.Criterion{{Checked: false}},
				},
			},
		},
	}

	got := firstWorkableTask(plans, nil, nil, featureDir)
	if got != nil {
		t.Errorf("expected nil when all tasks are cross-feature blocked, got %v", got.ID)
	}
}

// ─── crossFeatureWaitStatus ─────────────────────────────────────────────────

func TestCrossFeatureWaitStatus_SingleRef(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	status, blockedTask := o.crossFeatureWaitStatus(tasks)
	if blockedTask == nil {
		t.Fatal("expected non-nil blockedTask")
	}
	if blockedTask.ID != "TASK-014-001" {
		t.Errorf("blockedTask.ID = %q, want TASK-014-001", blockedTask.ID)
	}
	want := "Waiting for Feature 006 (DTOs)"
	if status != want {
		t.Errorf("status = %q, want %q", status, want)
	}
}

func TestCrossFeatureWaitStatus_MultipleRefs(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
		"feature_008.md": "# Feature 008\n## Tasks\n### TASK-008-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:       "TASK-014-001",
			Parallel: false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{
				{FeatureNums: []int{6}, Label: "DTOs"},
				{FeatureNums: []int{8}, Label: "API"},
			},
			Criteria: []parser.Criterion{{Checked: false}},
		},
	}

	status, blockedTask := o.crossFeatureWaitStatus(tasks)
	if blockedTask == nil {
		t.Fatal("expected non-nil blockedTask")
	}
	want := "Waiting for Feature 006 (DTOs), Feature 008 (API)"
	if status != want {
		t.Errorf("status = %q, want %q", status, want)
	}
}

func TestCrossFeatureWaitStatus_NoBlockedTasks(t *testing.T) {
	// All cross-feature deps satisfied — no wait status.
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006_completed.md": "# complete\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			Parallel:                false,
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	status, blockedTask := o.crossFeatureWaitStatus(tasks)
	if blockedTask != nil {
		t.Errorf("expected nil blockedTask when all satisfied, got %v", blockedTask.ID)
	}
	if status != "" {
		t.Errorf("expected empty status when all satisfied, got %q", status)
	}
}

func TestCrossFeatureWaitStatus_DeduplicatesRefs(t *testing.T) {
	// Two tasks both blocked on the same feature — display text should appear only once.
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
		{
			ID:                      "TASK-014-002",
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	status, _ := o.crossFeatureWaitStatus(tasks)
	want := "Waiting for Feature 006 (DTOs)"
	if status != want {
		t.Errorf("status = %q, want %q (should deduplicate)", status, want)
	}
}

// ─── Cross-feature deps become satisfied on next cycle ──────────────────────

func TestClassifyWorkable_CrossFeatureBecomeSatisfied(t *testing.T) {
	dir := crossFeatureFixture(t, map[string]string{
		"feature_006.md": "# Feature 006\n## Tasks\n### TASK-006-001: WIP\n**Acceptance Criteria:**\n- [ ] Not done\n",
	})

	o := &Orchestrator{cfg: OrchestratorConfig{Dir: dir}}
	o.completedIDs = map[string]bool{}
	o.failedIDs = map[string]bool{}
	o.skippedOrBlockedIDs = map[string]bool{}

	tasks := []parser.Task{
		{
			ID:                      "TASK-014-001",
			CrossFeaturePredecessors: []parser.CrossFeatureRef{{FeatureNums: []int{6}, Label: "DTOs"}},
			Criteria:                []parser.Criterion{{Checked: false}},
		},
	}

	// Initially blocked.
	par, seq := o.classifyWorkable(tasks)
	if len(par)+len(seq) != 0 {
		t.Fatalf("expected blocked initially, got par=%v seq=%v", taskIDs(par), taskIDs(seq))
	}

	// Simulate feature completing between daemon cycles by renaming the file.
	featDir := filepath.Join(dir, ".maggus", "features")
	if err := os.Rename(
		filepath.Join(featDir, "feature_006.md"),
		filepath.Join(featDir, "feature_006_completed.md"),
	); err != nil {
		t.Fatal(err)
	}

	// Now the task should be workable.
	par, seq = o.classifyWorkable(tasks)
	if len(seq) != 1 || seq[0].ID != "TASK-014-001" {
		t.Errorf("expected [TASK-014-001] after feature completes, got par=%v seq=%v", taskIDs(par), taskIDs(seq))
	}
}

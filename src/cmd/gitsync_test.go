package cmd

import (
	"testing"

	"github.com/leberkas-org/maggus/internal/gitsync"
	"github.com/leberkas-org/maggus/internal/tui/styles"
)

func TestSyncActionConstants(t *testing.T) {
	// Verify iota ordering: syncProceed=0, syncAbort=1
	if syncProceed != 0 {
		t.Errorf("syncProceed = %d, want 0", syncProceed)
	}
	if syncAbort != 1 {
		t.Errorf("syncAbort = %d, want 1", syncAbort)
	}
	// They must be distinct
	if syncProceed == syncAbort {
		t.Error("syncProceed and syncAbort must be distinct")
	}
}

func TestSyncStateConstants(t *testing.T) {
	states := []syncState{
		syncStateLoading,
		syncStateClean,
		syncStateMenu,
		syncStateDirtyOnly,
		syncStateConfirmForce,
		syncStateRunning,
		syncStateDone,
		syncStateError,
	}

	// All states must be unique
	seen := make(map[syncState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("duplicate syncState value: %d", s)
		}
		seen[s] = true
	}

	// Loading should be zero (iota)
	if syncStateLoading != 0 {
		t.Errorf("syncStateLoading = %d, want 0", syncStateLoading)
	}
}

func TestNewSyncModel(t *testing.T) {
	m := newSyncModel("/some/dir")

	if m.dir != "/some/dir" {
		t.Errorf("dir = %q, want /some/dir", m.dir)
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
	if m.state != syncStateLoading {
		t.Errorf("state = %d, want syncStateLoading (%d)", m.state, syncStateLoading)
	}
	if m.frame != 0 {
		t.Errorf("frame = %d, want 0", m.frame)
	}
	if len(m.options) != 0 {
		t.Errorf("options should be empty, got %d", len(m.options))
	}
}

func TestDetermineState_CleanAndUpToDate(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: true, Behind: 0, Ahead: 0}
	m.workTree = gitsync.WorkTree{}

	m.determineState()

	if m.state != syncStateClean {
		t.Errorf("state = %d, want syncStateClean (%d)", m.state, syncStateClean)
	}
}

func TestDetermineState_NoRemote(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: false}
	m.workTree = gitsync.WorkTree{}

	m.determineState()

	if m.state != syncStateClean {
		t.Errorf("state = %d, want syncStateClean (%d) when no remote", m.state, syncStateClean)
	}
}

func TestDetermineState_DirtyOnly_Uncommitted(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: true, Behind: 0}
	m.workTree = gitsync.WorkTree{HasUncommittedChanges: true}

	m.determineState()

	if m.state != syncStateDirtyOnly {
		t.Errorf("state = %d, want syncStateDirtyOnly (%d)", m.state, syncStateDirtyOnly)
	}
}

func TestDetermineState_DirtyOnly_Untracked(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: true, Behind: 0}
	m.workTree = gitsync.WorkTree{HasUntrackedFiles: true}

	m.determineState()

	if m.state != syncStateDirtyOnly {
		t.Errorf("state = %d, want syncStateDirtyOnly (%d)", m.state, syncStateDirtyOnly)
	}
}

func TestDetermineState_BehindRemote(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: true, Behind: 3}
	m.workTree = gitsync.WorkTree{}

	m.determineState()

	if m.state != syncStateMenu {
		t.Errorf("state = %d, want syncStateMenu (%d)", m.state, syncStateMenu)
	}
	if len(m.options) == 0 {
		t.Error("options should be populated when behind remote")
	}
}

func TestDetermineState_BehindAndDirty(t *testing.T) {
	m := newSyncModel("")
	m.remote = gitsync.Status{HasRemote: true, Behind: 2}
	m.workTree = gitsync.WorkTree{HasUncommittedChanges: true}

	m.determineState()

	if m.state != syncStateMenu {
		t.Errorf("state = %d, want syncStateMenu (%d)", m.state, syncStateMenu)
	}
}

func TestBuildOptions_Clean(t *testing.T) {
	m := newSyncModel("")
	m.workTree = gitsync.WorkTree{} // no dirty files

	m.buildOptions()

	if len(m.options) != 6 {
		t.Fatalf("expected 6 options, got %d", len(m.options))
	}

	expectedLabels := []string{"Pull", "Pull with rebase", "Force pull", "Stash & pull", "Skip", "Abort"}
	for i, want := range expectedLabels {
		if m.options[i].label != want {
			t.Errorf("option[%d].label = %q, want %q", i, m.options[i].label, want)
		}
	}

	// No warnings when clean
	for i, opt := range m.options[:4] {
		if opt.warning {
			t.Errorf("option[%d] (%s) should not have warning when clean", i, opt.label)
		}
	}

	// Cursor should default to 0
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 when clean", m.cursor)
	}
}

func TestBuildOptions_Dirty(t *testing.T) {
	m := newSyncModel("")
	m.workTree = gitsync.WorkTree{HasUncommittedChanges: true}

	m.buildOptions()

	if len(m.options) != 6 {
		t.Fatalf("expected 6 options, got %d", len(m.options))
	}

	// First 3 options should have warnings
	for i := 0; i < 3; i++ {
		if !m.options[i].warning {
			t.Errorf("option[%d] (%s) should have warning when dirty", i, m.options[i].label)
		}
	}

	// "Stash & pull" should be recommended
	stashOpt := m.options[3]
	if stashOpt.label != "Stash & pull" {
		t.Fatalf("option[3].label = %q, want 'Stash & pull'", stashOpt.label)
	}
	if !stashOpt.recommended {
		t.Error("Stash & pull should be recommended when dirty")
	}

	// Cursor should default to 3 (Stash & pull)
	if m.cursor != 3 {
		t.Errorf("cursor = %d, want 3 (Stash & pull) when dirty", m.cursor)
	}
}

func TestBuildOptions_DirtyUntracked(t *testing.T) {
	m := newSyncModel("")
	m.workTree = gitsync.WorkTree{HasUntrackedFiles: true}

	m.buildOptions()

	// Untracked files count as dirty for option building
	if !m.options[0].warning {
		t.Error("Pull should have warning when untracked files present")
	}
	if m.cursor != 3 {
		t.Errorf("cursor = %d, want 3 when untracked files present", m.cursor)
	}
}

func TestBuildOptions_AlwaysHasSkipAndAbort(t *testing.T) {
	for _, dirty := range []bool{true, false} {
		m := newSyncModel("")
		if dirty {
			m.workTree = gitsync.WorkTree{HasUncommittedChanges: true}
		}
		m.buildOptions()

		n := len(m.options)
		if n < 2 {
			t.Fatalf("expected at least 2 options, got %d", n)
		}
		if m.options[n-2].label != "Skip" {
			t.Errorf("second-to-last option = %q, want Skip (dirty=%v)", m.options[n-2].label, dirty)
		}
		if m.options[n-1].label != "Abort" {
			t.Errorf("last option = %q, want Abort (dirty=%v)", m.options[n-1].label, dirty)
		}
	}
}

func TestBuildOptions_ResetsState(t *testing.T) {
	m := newSyncModel("")
	m.workTree = gitsync.WorkTree{HasUncommittedChanges: true}
	m.buildOptions()
	firstCursor := m.cursor

	// Rebuild with clean state
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()

	if m.cursor == firstCursor && firstCursor != 0 {
		t.Error("buildOptions should reset cursor")
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after rebuild with clean state", m.cursor)
	}
}

func TestSyncResult_Fields(t *testing.T) {
	r := syncResult{action: syncProceed, message: "Pulled 3 commits"}
	if r.action != syncProceed {
		t.Errorf("action = %d, want syncProceed", r.action)
	}
	if r.message != "Pulled 3 commits" {
		t.Errorf("message = %q, want 'Pulled 3 commits'", r.message)
	}

	r2 := syncResult{action: syncAbort}
	if r2.action != syncAbort {
		t.Errorf("action = %d, want syncAbort", r2.action)
	}
	if r2.message != "" {
		t.Errorf("message = %q, want empty", r2.message)
	}
}

func TestSyncOption_Fields(t *testing.T) {
	opt := syncOption{label: "Pull", desc: "git pull", warning: true, recommended: false}
	if opt.label != "Pull" {
		t.Errorf("label = %q, want Pull", opt.label)
	}
	if opt.desc != "git pull" {
		t.Errorf("desc = %q, want 'git pull'", opt.desc)
	}
	if !opt.warning {
		t.Error("warning should be true")
	}
	if opt.recommended {
		t.Error("recommended should be false")
	}
}

func TestSyncSpinner(t *testing.T) {
	if len(styles.SpinnerFrames) == 0 {
		t.Error("styles.SpinnerFrames should not be empty")
	}
	// Should have 10 frames (Braille spinner pattern)
	if len(styles.SpinnerFrames) != 10 {
		t.Errorf("styles.SpinnerFrames has %d frames, want 10", len(styles.SpinnerFrames))
	}
}

func TestSelectOption_Skip(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()

	// Find Skip option index
	for i, opt := range m.options {
		if opt.label == "Skip" {
			m.cursor = i
			break
		}
	}

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.result.action != syncProceed {
		t.Errorf("Skip should result in syncProceed, got %d", rm.result.action)
	}
	if rm.result.message != "Skipped git sync" {
		t.Errorf("message = %q, want 'Skipped git sync'", rm.result.message)
	}
	if cmd == nil {
		t.Error("Skip should return a quit command")
	}
}

func TestSelectOption_Abort(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()

	// Find Abort option index
	for i, opt := range m.options {
		if opt.label == "Abort" {
			m.cursor = i
			break
		}
	}

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.result.action != syncAbort {
		t.Errorf("Abort should result in syncAbort, got %d", rm.result.action)
	}
	if cmd == nil {
		t.Error("Abort should return a quit command")
	}
}

func TestSelectOption_ForcePull_GoesToConfirm(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()

	// Find Force pull option
	for i, opt := range m.options {
		if opt.label == "Force pull" {
			m.cursor = i
			break
		}
	}

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.state != syncStateConfirmForce {
		t.Errorf("Force pull should transition to syncStateConfirmForce, got %d", rm.state)
	}
	if cmd != nil {
		t.Error("Force pull confirm should not return a command")
	}
}

func TestSelectOption_Pull_GoesToRunning(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()
	m.cursor = 0 // Pull is first

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.state != syncStateRunning {
		t.Errorf("Pull should transition to syncStateRunning, got %d", rm.state)
	}
	if cmd == nil {
		t.Error("Pull should return a command")
	}
}

func TestSelectOption_PullRebase_GoesToRunning(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()
	m.cursor = 1 // Pull with rebase

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.state != syncStateRunning {
		t.Errorf("Pull with rebase should transition to syncStateRunning, got %d", rm.state)
	}
	if cmd == nil {
		t.Error("Pull with rebase should return a command")
	}
}

func TestSelectOption_StashAndPull_GoesToRunning(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.workTree = gitsync.WorkTree{}
	m.buildOptions()
	m.cursor = 3 // Stash & pull

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	if rm.state != syncStateRunning {
		t.Errorf("Stash & pull should transition to syncStateRunning, got %d", rm.state)
	}
	if cmd == nil {
		t.Error("Stash & pull should return a command")
	}
}

func TestSelectOption_OutOfBounds(t *testing.T) {
	m := newSyncModel("")
	m.state = syncStateMenu
	m.options = []syncOption{{label: "Test"}}
	m.cursor = 5 // out of bounds

	result, cmd := m.selectOption()
	rm := result.(syncModel)

	// Should be a no-op
	if rm.state != syncStateMenu {
		t.Errorf("out-of-bounds cursor should not change state, got %d", rm.state)
	}
	if cmd != nil {
		t.Error("out-of-bounds cursor should return nil command")
	}
}

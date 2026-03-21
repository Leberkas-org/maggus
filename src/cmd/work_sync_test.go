package cmd

import (
	"errors"
	"testing"

	"github.com/leberkas-org/maggus/internal/gitsync"
)

// stubSyncDeps replaces checkSync's function variables with the given stubs
// and restores the originals when the test finishes.
func stubSyncDeps(t *testing.T, opts syncStubs) {
	t.Helper()

	origFetch := fetchRemoteFn
	origRemote := remoteStatusFn
	origWorkTree := workingTreeStatusFn
	origTUI := runGitSyncTUI

	if opts.fetchRemote != nil {
		fetchRemoteFn = opts.fetchRemote
	}
	if opts.remoteStatus != nil {
		remoteStatusFn = opts.remoteStatus
	}
	if opts.workingTreeStatus != nil {
		workingTreeStatusFn = opts.workingTreeStatus
	}
	if opts.syncTUI != nil {
		runGitSyncTUI = opts.syncTUI
	}

	t.Cleanup(func() {
		fetchRemoteFn = origFetch
		remoteStatusFn = origRemote
		workingTreeStatusFn = origWorkTree
		runGitSyncTUI = origTUI
	})
}

type syncStubs struct {
	fetchRemote       func(string) error
	remoteStatus      func(string) (gitsync.Status, error)
	workingTreeStatus func(string) (gitsync.WorkTree, error)
	syncTUI           func(string) (syncResult, error)
}

func TestCheckSync_NoRemote(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote:       func(string) error { return nil },
		remoteStatus:      func(string) (gitsync.Status, error) { return gitsync.Status{HasRemote: false}, nil },
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abort {
		t.Error("expected shouldAbort=false")
	}
	if msg != "" {
		t.Errorf("expected empty message for no remote, got %q", msg)
	}
}

func TestCheckSync_UpToDate(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 0}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abort {
		t.Error("expected shouldAbort=false")
	}
	want := "✓ Branch up to date with origin/main"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}

func TestCheckSync_FetchFailure_ShowsOfflineMessage(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return errors.New("network error") },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 0}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abort {
		t.Error("expected shouldAbort=false")
	}
	want := "⚠ Could not reach remote — working offline"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}

func TestCheckSync_BehindRemote_TUIAbort(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 3}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
		syncTUI: func(string) (syncResult, error) {
			return syncResult{action: syncAbort}, nil
		},
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !abort {
		t.Error("expected shouldAbort=true when user aborts")
	}
	if msg != "" {
		t.Errorf("expected empty message on abort, got %q", msg)
	}
}

func TestCheckSync_BehindRemote_TUIProceed(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 3}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
		syncTUI: func(string) (syncResult, error) {
			return syncResult{action: syncProceed, message: "Pulled 3 commits"}, nil
		},
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abort {
		t.Error("expected shouldAbort=false")
	}
	if msg != "Pulled 3 commits" {
		t.Errorf("got %q, want %q", msg, "Pulled 3 commits")
	}
}

func TestCheckSync_DirtyWorkTree_TriggersTUI(t *testing.T) {
	tuiCalled := false
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 0}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) {
			return gitsync.WorkTree{HasUncommittedChanges: true}, nil
		},
		syncTUI: func(string) (syncResult, error) {
			tuiCalled = true
			return syncResult{action: syncProceed, message: "Proceeding with uncommitted changes"}, nil
		},
	})

	msg, abort, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tuiCalled {
		t.Error("expected sync TUI to be called for dirty work tree")
	}
	if abort {
		t.Error("expected shouldAbort=false")
	}
	if msg != "Proceeding with uncommitted changes" {
		t.Errorf("got %q, want %q", msg, "Proceeding with uncommitted changes")
	}
}

func TestCheckSync_UntrackedFiles_TriggersTUI(t *testing.T) {
	tuiCalled := false
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, RemoteBranch: "origin/main", Behind: 0}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) {
			return gitsync.WorkTree{HasUntrackedFiles: true}, nil
		},
		syncTUI: func(string) (syncResult, error) {
			tuiCalled = true
			return syncResult{action: syncProceed, message: "ok"}, nil
		},
	})

	_, _, err := checkSync("/fake")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tuiCalled {
		t.Error("expected sync TUI to be called for untracked files")
	}
}

func TestCheckSync_TUIError_ReturnsError(t *testing.T) {
	stubSyncDeps(t, syncStubs{
		fetchRemote: func(string) error { return nil },
		remoteStatus: func(string) (gitsync.Status, error) {
			return gitsync.Status{HasRemote: true, Behind: 1}, nil
		},
		workingTreeStatus: func(string) (gitsync.WorkTree, error) { return gitsync.WorkTree{}, nil },
		syncTUI: func(string) (syncResult, error) {
			return syncResult{}, errors.New("TUI crashed")
		},
	})

	_, _, err := checkSync("/fake")
	if err == nil {
		t.Fatal("expected error from TUI failure")
	}
	if err.Error() != "TUI crashed" {
		t.Errorf("got error %q, want %q", err.Error(), "TUI crashed")
	}
}

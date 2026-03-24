package gitsync

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leberkas-org/maggus/internal/gitutil"
)

// ErrStashPopConflict is returned when `git stash pop` results in merge conflicts.
type ErrStashPopConflict struct {
	Output string
}

func (e *ErrStashPopConflict) Error() string {
	return fmt.Sprintf("stash pop resulted in merge conflicts: %s", e.Output)
}

// ErrForcePullNotConfirmed is returned when ForcePull is called without confirmation.
var ErrForcePullNotConfirmed = errors.New("force pull requires explicit confirmation")

// WorkTree represents the local working tree state.
type WorkTree struct {
	HasUncommittedChanges bool
	HasUntrackedFiles     bool
	ModifiedFiles         []string // capped to first 10 entries
	TotalModified         int
}

// Status represents the relationship between the local branch and its remote tracking branch.
type Status struct {
	Behind       int
	Ahead        int
	RemoteBranch string
	HasRemote    bool
}

// FetchRemote runs `git fetch` to update remote tracking refs.
// Returns an error if the fetch fails (e.g. no network), which callers
// should treat as a warning rather than a fatal error.
func FetchRemote(dir string) error {
	cmd := gitutil.Command("fetch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoteStatus compares the local HEAD with its remote tracking branch.
// If no upstream is configured, it returns Status{HasRemote: false} with no error.
// If the HEAD is detached, it returns Status{HasRemote: false} with no error.
func RemoteStatus(dir string) (Status, error) {
	// Check if HEAD is detached
	branchCmd := gitutil.Command("symbolic-ref", "--short", "HEAD")
	branchCmd.Dir = dir
	if _, err := branchCmd.Output(); err != nil {
		// Detached HEAD — no upstream possible
		return Status{HasRemote: false}, nil
	}

	// Get the upstream tracking branch name
	upstreamCmd := gitutil.Command("rev-parse", "--abbrev-ref", "@{upstream}")
	upstreamCmd.Dir = dir
	upstreamOut, err := upstreamCmd.Output()
	if err != nil {
		// No upstream configured
		return Status{HasRemote: false}, nil
	}
	remoteBranch := strings.TrimSpace(string(upstreamOut))

	// Get ahead/behind counts
	revListCmd := gitutil.Command("rev-list", "--count", "--left-right", "HEAD...@{upstream}")
	revListCmd.Dir = dir
	revListOut, err := revListCmd.Output()
	if err != nil {
		return Status{}, fmt.Errorf("git rev-list: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(revListOut)))
	if len(parts) != 2 {
		return Status{}, fmt.Errorf("unexpected rev-list output: %q", string(revListOut))
	}

	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return Status{}, fmt.Errorf("parse ahead count: %w", err)
	}
	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return Status{}, fmt.Errorf("parse behind count: %w", err)
	}

	return Status{
		Behind:       behind,
		Ahead:        ahead,
		RemoteBranch: remoteBranch,
		HasRemote:    true,
	}, nil
}

// WorkingTreeStatus detects uncommitted local changes using `git status --porcelain`.
func WorkingTreeStatus(dir string) (WorkTree, error) {
	cmd := gitutil.Command("status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return WorkTree{}, fmt.Errorf("git status: %w", err)
	}

	var wt WorkTree
	var allFiles []string

	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// Porcelain format: XY filename
		// X = index status, Y = work-tree status
		// "??" = untracked
		if len(line) < 2 {
			continue
		}
		xy := line[:2]
		file := strings.TrimSpace(line[2:])

		if xy == "??" {
			wt.HasUntrackedFiles = true
		} else {
			wt.HasUncommittedChanges = true
		}
		allFiles = append(allFiles, file)
	}

	wt.TotalModified = len(allFiles)
	if len(allFiles) > 10 {
		wt.ModifiedFiles = allFiles[:10]
	} else {
		wt.ModifiedFiles = allFiles
	}

	return wt, nil
}

// Pull performs a standard `git pull` (fast-forward merge).
func Pull(dir string) error {
	cmd := gitutil.Command("pull")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PullRebase performs `git pull --rebase`.
func PullRebase(dir string) error {
	cmd := gitutil.Command("pull", "--rebase")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull --rebase: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ForcePull fetches from origin and resets to the upstream branch, discarding
// all local commits and changes. The confirm parameter must be true or
// ErrForcePullNotConfirmed is returned.
func ForcePull(dir string, confirm bool) error {
	if !confirm {
		return ErrForcePullNotConfirmed
	}

	fetchCmd := gitutil.Command("fetch", "origin")
	fetchCmd.Dir = dir
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin: %w: %s", err, strings.TrimSpace(string(out)))
	}

	resetCmd := gitutil.Command("reset", "--hard", "@{upstream}")
	resetCmd.Dir = dir
	if out, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset --hard @{upstream}: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// StashAndPull stashes local changes, pulls, then pops the stash.
// Returns *ErrStashPopConflict if stash pop results in merge conflicts.
func StashAndPull(dir string) error {
	stashCmd := gitutil.Command("stash", "push", "-m", "maggus: auto-stash before pull")
	stashCmd.Dir = dir
	if out, err := stashCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git stash push: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pullCmd := gitutil.Command("pull")
	pullCmd.Dir = dir
	if out, err := pullCmd.CombinedOutput(); err != nil {
		// Pull failed — try to restore the stash
		popCmd := gitutil.Command("stash", "pop")
		popCmd.Dir = dir
		_ = popCmd.Run()
		return fmt.Errorf("git pull: %w: %s", err, strings.TrimSpace(string(out)))
	}

	popCmd := gitutil.Command("stash", "pop")
	popCmd.Dir = dir
	popOut, err := popCmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(popOut))
		// Check if this is a conflict situation
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "conflict") {
			return &ErrStashPopConflict{Output: output}
		}
		return fmt.Errorf("git stash pop: %w: %s", err, output)
	}

	return nil
}

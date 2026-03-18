package gitsync

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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
	cmd := exec.Command("git", "fetch")
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
	branchCmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	branchCmd.Dir = dir
	if _, err := branchCmd.Output(); err != nil {
		// Detached HEAD — no upstream possible
		return Status{HasRemote: false}, nil
	}

	// Get the upstream tracking branch name
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}")
	upstreamCmd.Dir = dir
	upstreamOut, err := upstreamCmd.Output()
	if err != nil {
		// No upstream configured
		return Status{HasRemote: false}, nil
	}
	remoteBranch := strings.TrimSpace(string(upstreamOut))

	// Get ahead/behind counts
	revListCmd := exec.Command("git", "rev-list", "--count", "--left-right", "HEAD...@{upstream}")
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

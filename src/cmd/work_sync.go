package cmd

import (
	"fmt"

	"github.com/leberkas-org/maggus/internal/gitsync"
)

// checkSync performs the pre-work git sync check. It fetches from the remote,
// checks for uncommitted changes and diverged branches, and optionally shows
// an interactive sync TUI if action is needed.
//
// Returns:
//   - syncInfoMsg: a status message to display in the work TUI banner
//   - shouldAbort: true if the user chose to abort from the sync TUI
//   - err: non-nil only on unexpected errors (TUI failure, etc.)
func checkSync(dir string) (syncInfoMsg string, shouldAbort bool, err error) {
	fetchErr := gitsync.FetchRemote(dir)
	remoteStatus, _ := gitsync.RemoteStatus(dir)
	workTreeStatus, _ := gitsync.WorkingTreeStatus(dir)

	hasDirty := workTreeStatus.HasUncommittedChanges || workTreeStatus.HasUntrackedFiles
	isBehind := remoteStatus.HasRemote && remoteStatus.Behind > 0

	if !remoteStatus.HasRemote {
		// No remote configured: silently skip
		return "", false, nil
	}

	if isBehind || hasDirty {
		// Behind remote or uncommitted changes: show interactive sync TUI
		result, syncErr := runGitSyncTUI(dir)
		if syncErr != nil {
			return "", false, syncErr
		}
		if result.action == syncAbort {
			return "", true, nil
		}
		return result.message, false, nil
	}

	if fetchErr != nil {
		return "⚠ Could not reach remote — working offline", false, nil
	}

	return fmt.Sprintf("✓ Branch up to date with %s", remoteStatus.RemoteBranch), false, nil
}

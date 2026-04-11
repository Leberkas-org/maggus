<!-- maggus-id: 25b5dd9e-a208-42af-a838-9eff5ac6f764 -->
# Bug: Creating a new plan/bug file causes the currently processing feature to lose approval

## Summary

When a new feature or bug file is created (e.g., via the `/maggus-plan` skill), the currently approved and processing feature gets unapproved. The daemon then stops working on it because it's no longer in the approved set.

## Steps to Reproduce

1. Approve a feature (e.g., bug_028) and start the daemon
2. While the daemon is processing bug_028, create a new plan file (e.g., `feature_045.md`) via Claude Code or manually
3. Observe in the status TUI: bug_028's approval badge changes from `✓` to `○`
4. The daemon stops working on bug_028

## Expected Behavior

Creating a new plan file should not affect the approval status of existing plans. The daemon should continue processing the approved feature undisturbed.

## Root Cause

The daemon writes to `feature_approvals.yml` when it shouldn't. `buildApprovedPlans()` in `src/cmd/run_loop.go:193` calls `approval.Prune(dir, knownIDs)` on every work cycle. `Prune` does a non-atomic read-modify-write:

1. Reads `feature_approvals.yml` from disk
2. Removes keys not in `knownIDs`
3. Writes the file back

The daemon only needs to **read** approvals to decide which plans to work on. It has no business writing to the approval file. When the TUI simultaneously writes to the same file (via `reloadPlans()` → `pruneStaleApprovals()` triggered by the filewatcher detecting the new plan file), the two writers race and one overwrites the other's changes.

The concrete race:
1. User creates `feature_045.md` → filewatcher fires
2. TUI calls `reloadPlans()` → `pruneStaleApprovals()` → `approval.Prune()` reads file, prunes, writes
3. Daemon wakes from idle → `buildApprovedPlans()` → `approval.Prune()` reads file, prunes, writes
4. If the daemon's read happens before the TUI's write lands, the daemon writes a stale copy — potentially losing the approval entry

## User Stories

### BUG-032-001: Remove approval.Prune from the daemon work loop

**Description:** As a user, I want the daemon to only read approvals, never write them, so it cannot corrupt the approval file.

**Acceptance Criteria:**
- [x] `buildApprovedPlans()` in `src/cmd/run_loop.go` no longer calls `approval.Prune()`
- [x] The daemon only calls `approval.Load()` (read-only) to determine which plans are approved
- [x] Pruning remains in the TUI path only (`reloadPlans()` in `status_model.go` and `pruneStaleApprovals` calls in `menu_model.go` / `app_model.go`)
- [x] Creating a new feature file while the daemon is processing an approved feature does NOT change the approved feature's approval state
- [x] The daemon continues working on the approved feature undisturbed after new files are created
- [x] No regression in approval toggle from the TUI (`a` key)
- [x] `go vet ./...` and `go test ./...` pass

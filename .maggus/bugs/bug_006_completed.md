<!-- maggus-id: 77d4cc77-33a7-44a1-b289-8121e9b76315 -->
# Bug: Approved plan renders as unapproved (○) in left pane

## Summary

An approved plan sometimes shows the orange ○ badge in the left pane of `maggus status` instead of the green ✓. The wrong state appears when the status view first opens and persists across restarts.

## Steps to Reproduce

1. Run `maggus status`
2. Observe a plan that is approved shows ○ instead of ✓ in the left pane badge

## Expected Behavior

An approved plan should show the green ✓ badge. The badge should reflect the actual approval state stored in `.maggus/feature_approvals.yml`.

## Root Cause

The badge in `status_leftpane.go:129` is computed as:

```go
} else if isPlanApproved(plan, m.approvals, m.approvalRequired) {
    badge = greenStyle.Render("✓")
} else {
    badge = orangeStyle.Render("○")
}
```

`m.approvals` is loaded by `loadPlansWithApprovals` at startup and on every `reloadPlans()` call. `isPlanApproved` looks up `plan.ApprovalKey()` (returns `MaggusID` if set, else filename-based `ID`) in `m.approvals`.

**First investigation step:** Check `.maggus/feature_approvals.yml`. The key stored there may not match what `plan.ApprovalKey()` returns for the affected plan.

**Candidate causes (ranked by likelihood):**

1. **`pruneStaleApprovals` removes a legitimate entry at startup** — `runStatus` calls `pruneStaleApprovals(dir, features)` immediately after loading approvals but before creating the model. If the plan's `ApprovalKey()` at that moment differs from the key stored in the YAML (e.g. due to a key mismatch between the in-memory plan and the YAML), the entry is pruned. The watcher then fires ~300ms later (because `feature_approvals.yml` was written), `reloadPlans()` reads the pruned file, and the badge flips to ○. The model's initial render may be correct for a brief moment before the watcher-triggered reload overwrites it with the pruned state.
   - **Key mismatch scenario**: the approval was stored under the filename-based ID (e.g. `bug_006`) but `plan.ApprovalKey()` now returns the MaggusID UUID (because the `<!-- maggus-id: ... -->` comment was added/changed after the approval was first saved).

2. **`reloadPlans()` silently fails after approve toggle** — in `reloadPlans()` (`status_model.go:184`), if `loadPlansWithApprovals` returns an error, it returns early without updating `m.approvals`. If this happens after a successful `approval.Approve()` call, the disk has the correct state but the in-memory `m.approvals` remains stale, showing ○ until the next successful reload.

3. **Opt-out mode toggle confusion** — if `approval_required` is unset/false, pressing `a` on a plan that is approved-by-default (no explicit entry) calls `approval.Unapprove()`, writing an explicit `false` entry. On next startup `IsApproved` finds the explicit `false` and returns `false`. The plan shows ○ persistently. The user pressed `a` thinking to confirm approval, but it actually removed it.

## User Stories

### BUG-006-001: Investigate and fix approval key mismatch causing stale-prune badge regression

**Description:** As a user, I want the left-pane approval badge to always reflect the actual approval state so I can trust what I see.

**Acceptance Criteria:**
- [x] Check `.maggus/feature_approvals.yml` on a repo where the bug is reproducible; confirm whether the affected plan's `ApprovalKey()` matches the key stored in the file
- [x] If key mismatch: add a migration step in `loadPlansWithApprovals` or `pruneStaleApprovals` so that when a plan's `MaggusID`-based key is unknown but its filename-based ID is present, the entry is migrated rather than pruned
- [x] If mismatch is not the cause: add a debug log or test to catch when `reloadPlans()` returns without updating `m.approvals` (silent error path in `status_model.go:186-189`) and handle it gracefully
- [x] If opt-out toggle confusion: display a confirmation or change the toggle to be additive-only (approval stores explicit `true`, pressing `a` again removes the explicit entry rather than writing explicit `false`)
- [x] Approval badge in left pane matches actual `feature_approvals.yml` state on every render
- [x] Badge state is stable across open/close cycles
- [x] No regression in approve/unapprove toggle behavior
- [x] `go vet ./...` and `go test ./...` pass

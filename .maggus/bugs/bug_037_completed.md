<!-- maggus-id: add9ded2-cc87-4798-be67-8a9705c04382 -->
# Bug: Approve/unapprove should be disabled when approval_mode is opt-out

## Summary

When `approval_mode: opt-out` is configured, all features are approved by default. But the `maggus approve`, `maggus unapprove` CLI commands and the `a` key toggle in the status TUI still work — they write to `feature_approvals.yml` unnecessarily, can cause confusion (showing approval badges that don't matter), and contribute to the concurrent write race (bug_032).

## Steps to Reproduce

1. Set `approval_mode: opt-out` in `.maggus/config.yml`
2. Open status view, press `a` on a feature
3. Observe: "feature approved" status note appears, approval file is written
4. Or run `maggus approve 3` from CLI
5. Observe: approval entry is written even though everything is already approved by default

## Expected Behavior

When `approval_mode: opt-out`:
- The `a` key in the status TUI should show a message like "approval not required (opt-out mode)" and do nothing
- `maggus approve` and `maggus unapprove` CLI commands should print a message and exit without writing
- Approval badges in the TUI should either not be shown or always show as approved

## Root Cause

No guard in the approval paths checks the approval mode before writing:

- `handleApproveToggle()` in `src/cmd/status_update.go:871` directly calls `approval.Approve` / `approval.Remove` without checking `m.approvalRequired`
- `approveCmd.RunE` in `src/cmd/approve.go` loads config but doesn't check `approval_mode` before writing
- `unapproveCmd.RunE` in `src/cmd/approve.go` same issue

## User Stories

### BUG-037-001: Guard approval writes behind approval_mode check

**Description:** As a user with opt-out mode, I want approve/unapprove to be no-ops so I don't accidentally write to the approval file or get confused by approval badges.

**Acceptance Criteria:**
- [x] `handleApproveToggle()` in `status_update.go` checks `m.approvalRequired` — if false, sets `m.statusNote = "approval not required (opt-out mode)"` and returns without writing
- [x] `approveCmd.RunE` in `approve.go` loads config, checks `IsApprovalRequired()` — if false, prints "approval not required (opt-out mode)" and returns nil
- [x] `unapproveCmd.RunE` in `approve.go` same check
- [x] The `a` key hint in the status footer is hidden when `approvalRequired` is false
- [x] Approval badges (`✓`/`○`) in the left pane are either all shown as `✓` or hidden entirely when opt-out mode is active
- [x] No regression in opt-in mode — approve/unapprove works as before
- [x] `go vet ./...` and `go test ./...` pass

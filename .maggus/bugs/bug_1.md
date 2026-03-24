# Bug: New features display as approved but behave as unapproved

## Summary

In opt-in approval mode (the default), new features that have no entry in `feature_approvals.yml` are displayed as "✓ approved" in `maggus status` and the menu, but are actually skipped during `maggus work`. The display and the execution logic use different defaults.

## Steps to Reproduce

1. Ensure `config.yml` has `approval_mode: ""` (opt-in mode, the default)
2. Create a new feature file (e.g. `feature_004.md`) with workable tasks
3. Do NOT add it to `feature_approvals.yml`
4. Run `maggus status` — the feature shows `✓` (approved)
5. Run `maggus work` — the feature is skipped entirely

## Expected Behavior

A feature not present in `feature_approvals.yml` under opt-in mode should display as `✗ unapproved` in `maggus status`, matching the actual skip behavior of `maggus work`.

## Root Cause

`parseFeatures()` and `parseBugs()` in `src/cmd/status_plans.go` hard-code `false` as the `approvalRequired` argument when calling `approval.IsApproved()`:

```go
// status_plans.go line ~86 (parseFeatures)
approved: approval.IsApproved(a, featureID, false),  // false = opt-out semantics

// status_plans.go line ~115 (parseBugs)
approved: approval.IsApproved(a, featureID, false),  // false = opt-out semantics
```

With `approvalRequired=false`, `IsApproved()` returns `true` for any feature not in the approvals map (opt-out default). This makes every new feature appear approved.

The work loop in `src/cmd/work_loop.go` correctly reads the config:

```go
// work_loop.go line ~176
approvalRequired := cfg.IsApprovalRequired()  // true in opt-in mode
if !approval.IsApproved(approvals, id, approvalRequired) {
    continue  // correctly skips unapproved features
}
```

`IsApprovalRequired()` returns `true` when `ApprovalMode != "opt-out"` (i.e. the default empty string), so new features are skipped. The display never reflects this because `status_plans.go` doesn't receive or use the config.

## User Stories

### BUG-001-001: Pass config to status display functions so approval state renders correctly

**Description:** As a user, I want `maggus status` to show features as unapproved when they haven't been explicitly approved in opt-in mode, so the display matches what `maggus work` will actually do.

**Acceptance Criteria:**
- [ ] `parseFeatures(dir string)` is updated to accept a `cfg config.Config` parameter (or `approvalRequired bool`)
- [ ] `parseBugs(dir string)` is updated the same way
- [ ] Both functions pass `cfg.IsApprovalRequired()` (or the equivalent bool) to `approval.IsApproved()` instead of the hard-coded `false`
- [ ] All call sites of `parseFeatures` and `parseBugs` are updated to pass the config
- [ ] A new feature with no `feature_approvals.yml` entry shows `✗ unapproved` in `maggus status` when `approval_mode` is `""` (opt-in)
- [ ] The same feature shows `✓ approved` when `approval_mode: opt-out`
- [ ] Existing approved features (`feature_approvals.yml: true`) still show `✓` in both modes
- [ ] No regression in `maggus work` behavior
- [ ] `go vet ./...` and `go test ./...` pass

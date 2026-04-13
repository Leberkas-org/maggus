<!-- maggus-id: 2066d98f-e286-425e-96b0-b6742e242e91 -->
# Bug: Plan tab labels tasks with unknown predecessors as "(unresolved)" — should be "(cross-feature)"

## Summary

Tasks whose `**Predecessors:**` reference IDs not found in the current feature file are shown under a "Phase N (unresolved)" heading. "Unresolved" is cryptic — these tasks have cross-feature dependencies (e.g. `Feature 004 (mapping)`), not broken references. The label should be "(cross-feature)" to communicate that clearly.

## Root Cause

`src/cmd/status_plantab.go:53`:

```go
lines = append(lines, unresolvedHeaderStyle.Render(" "+label+" (unresolved)"))
```

## User Stories

### BUG-050-001: Rename "(unresolved)" to "(cross-feature)" in the plan tab

**Description:** As a user, I want tasks with cross-feature predecessor references to be labelled "(cross-feature)" in the plan tab so I understand why they appear separately.

**Acceptance Criteria:**
- [x] `status_plantab.go:53` renders `" "+label+" (cross-feature)"` instead of `" "+label+" (unresolved)"`
- [x] The `Unresolved` field name on `executionStep` and all related comments are updated to reflect the new meaning (`CrossFeature` or similar)
- [x] `go vet ./...` and `go test ./...` pass

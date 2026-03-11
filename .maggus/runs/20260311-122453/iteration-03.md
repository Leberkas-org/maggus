# Iteration 03 — TASK-503

## Task Selected
**TASK-503:** Hide completed plans by default; add `--all` flag

## Changes Made
- `src/cmd/status.go`:
  - Added `all` bool flag read from `--all` cobra flag
  - In task sections loop: added `if p.completed && !all { continue }` guard
  - In Plans table loop: added `if p.completed && !all { continue }` guard
  - Registered `--all` flag in `init()` with description "Show completed plans in task sections and Plans table"

## Commands Run
- `powershell.exe -NoProfile -Command "cd C:\c\maggus\src; go build -o maggus.exe . 2>&1"` — succeeded, no errors

## Acceptance Criteria Status
All 8 criteria met:
- [x] Without `--all`: completed plan task sections not printed
- [x] Without `--all`: completed plan rows in Plans table not printed
- [x] Without `--all`: header count still reflects all plans
- [x] Without `--all`: summary totals still include completed plan tasks
- [x] With `--all`: completed plan task sections printed before active plan sections (natural sort order)
- [x] With `--all`: completed plan rows in Plans table (chronological order)
- [x] `--plain` and `--all` can be combined
- [x] Typecheck/lint passes

## Deviations
None.

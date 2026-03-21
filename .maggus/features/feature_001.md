# Feature 001: Remove List Command & Reorder Menu

## Introduction

Remove the `list` command entirely — Maggus has outgrown it and the `status` view is now the superior way to browse tasks. Additionally, reorder the AI-assisted creation group in the main menu so that `prompt` appears first.

## Goals

- Eliminate dead code by fully removing the list command (source, tests, menu entry)
- Improve menu ergonomics by placing `prompt` first in the AI-assisted group
- Keep all keyboard shortcuts unchanged

## Tasks

### TASK-001-001: Remove list command and menu entry
**Description:** As a maintainer, I want the list command fully removed so there is no dead code or confusing menu entry.

**Token Estimate:** ~25k tokens
**Predecessors:** none
**Successors:** TASK-001-002
**Parallel:** no

**Acceptance Criteria:**
- [ ] `src/cmd/list.go` is deleted
- [ ] `src/cmd/list_test.go` is deleted
- [ ] The `list` menu item is removed from `allMenuItems` in `menu.go`
- [ ] The `tasklist.go` shared component is NOT removed (still used by status)
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes

### TASK-001-002: Reorder AI-assisted creation group
**Description:** As a user, I want `prompt` to appear first in the AI-assisted creation menu group so it's more prominent.

**Token Estimate:** ~15k tokens
**Predecessors:** TASK-001-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] Group 3 order in `allMenuItems` is: prompt, vision, architecture, plan
- [x] `prompt` entry has `separator: true` (it's now the first item in the group)
- [x] `vision` entry no longer has `separator: true`
- [x] All shortcut keys remain unchanged (prompt=o, vision=v, architecture=a, plan=p)
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes
- [x] `go vet ./...` passes

## Task Dependency Graph

```
TASK-001-001 ──→ TASK-001-002
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-001-001 | ~25k | none | no | — |
| TASK-001-002 | ~15k | 001 | no | — |

**Total estimated tokens:** ~40k

## Functional Requirements

- FR-1: The `maggus list` CLI command must no longer exist
- FR-2: The main menu must not show a "list" entry
- FR-3: The `l` shortcut key must no longer trigger any action in the menu
- FR-4: The AI-assisted creation group order must be: prompt, vision, architecture, plan
- FR-5: All other menu items, shortcuts, and behavior remain unchanged

## Non-Goals

- No changes to the `status` command or its task browsing capabilities
- No changes to the `tasklist.go` shared component
- No changes to keyboard shortcuts (all stay as-is)
- No changes to the worktree sub-menu (its "list" action is unrelated)

## Technical Considerations

- The `taskListComponent` in `tasklist.go` is shared between `listModel` and `statusModel` — only `listModel` is removed
- The `menu_test.go` worktree tests reference `"list"` as a worktree action value, not the list command — these stay unchanged
- CLAUDE.md mentions `list` in the CLI Commands section and should be updated

## Success Metrics

- Zero references to `listCmd`, `listModel`, `runList`, or `renderListPlain` in the codebase
- Menu displays prompt as first AI-assisted item
- All tests pass

## Open Questions

None — all resolved during planning.

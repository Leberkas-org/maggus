# Bug: Git commands still cause terminal flicker on Windows

## Summary

`CREATE_NO_WINDOW` was applied to `gitcommit` and `gitbranch` in commit c452fce, but the majority of git subprocesses in the codebase still run without it. Every uncovered `exec.Command("git", ...)` call spawns a visible console window flash on Windows.

## Related

- **Commit:** c452fce (fix(git): suppress console window flicker for git subprocesses on Windows)

## Steps to Reproduce

1. Run `maggus work` on Windows
2. Observe console window flicker as tasks execute (git status checks, push, rev-parse, stash operations, worktree operations, etc.)

## Expected Behavior

No console window flicker for any git subprocess anywhere in maggus.

## Root Cause

Commit c452fce added `setProcAttr` with `CREATE_NO_WINDOW` only to `gitcommit` and `gitbranch`. The following packages and files run `exec.Command("git", ...)` without `setProcAttr` and are unaffected by the fix:

| Location | Commands |
|---|---|
| `src/internal/gitsync/gitsync.go:43,56,64,74,105,148,158,174,180,192,198,202,208` | fetch, symbolic-ref, rev-parse, rev-list, status, pull, reset, stash push/pop |
| `src/internal/worktree/worktree.go:18,29,40,59,88,98` | worktree add/remove/list/prune, branch -D |
| `src/internal/resolver/resolver.go:122` | rev-parse --is-inside-work-tree |
| `src/internal/release/release.go:47,75` | describe --tags, various tag/push commands |
| `src/cmd/gitsync.go:113` | rev-parse --abbrev-ref HEAD |
| `src/cmd/repos.go:546` | rev-parse --is-inside-work-tree |
| `src/cmd/release.go:140` | git tag/push commands |
| `src/cmd/work.go:231` | rev-parse --abbrev-ref HEAD |
| `src/cmd/work_task.go:147` | git add -- .maggus/ |
| `src/cmd/work_loop.go:520,525,566,568,595` | rev-parse --short HEAD, rev-parse --abbrev-ref HEAD, push |

The correct fix is to introduce a shared `internal/gitutil` package that exposes a `Command(args ...string) *exec.Cmd` factory. On Windows this factory sets `CREATE_NO_WINDOW`; on other platforms it is a no-op wrapper. All callers replace `exec.Command("git", ...)` with `gitutil.Command(...)`. This avoids duplicating `setProcAttr` boilerplate into every package.

## User Stories

### BUG-002-001: Add shared gitutil.Command factory with CREATE_NO_WINDOW on Windows

**Description:** As a developer, I want a single `gitutil.Command(args...)` function that centralizes the `CREATE_NO_WINDOW` flag so no git subprocess causes a console flash on Windows.

**Acceptance Criteria:**
- [ ] `src/internal/gitutil/gitutil.go` exposes `func Command(args ...string) *exec.Cmd` that calls `exec.Command("git", args...)` and applies `setProcAttr`
- [ ] `src/internal/gitutil/procattr_windows.go` sets `CREATE_NO_WINDOW` (0x08000000) on `SysProcAttr.CreationFlags`
- [ ] `src/internal/gitutil/procattr_other.go` is a no-op
- [ ] `go vet ./...` and `go test ./...` pass

### BUG-002-002: Migrate all remaining git exec.Command calls to gitutil.Command

**Description:** As a user, I want all git subprocesses to use the shared factory so that no console window flickers on Windows during any maggus operation.

**Acceptance Criteria:**
- [ ] All `exec.Command("git", ...)` calls in `src/internal/gitsync/gitsync.go` replaced with `gitutil.Command(...)`
- [ ] All `exec.Command("git", ...)` calls in `src/internal/worktree/worktree.go` replaced with `gitutil.Command(...)`
- [ ] All `exec.Command("git", ...)` calls in `src/internal/resolver/resolver.go` replaced with `gitutil.Command(...)`
- [ ] All `exec.Command("git", ...)` calls in `src/internal/release/release.go` replaced with `gitutil.Command(...)`
- [ ] All `exec.Command("git", ...)` calls in `src/cmd/gitsync.go`, `src/cmd/repos.go`, `src/cmd/release.go`, `src/cmd/work.go`, `src/cmd/work_task.go`, `src/cmd/work_loop.go` replaced with `gitutil.Command(...)`
- [ ] Existing `setProcAttr` boilerplate in `gitcommit` and `gitbranch` packages replaced with `gitutil.Command(...)` (removes now-redundant per-package procattr files)
- [ ] No regression in git operations across work, sync, release, and worktree flows
- [ ] `go vet ./...` and `go test ./...` pass

<!-- maggus-id: 0d546c00-66a2-4717-9780-8cdf9c7f8c2e -->
# Bug: Filewatcher misses new bug files when .maggus/bugs/ directory didn't exist at startup

## Summary

When `maggus status` is launched before any bugs exist in a repo, the `.maggus/bugs/` directory is not yet present and is never added to the fsnotify watch list. Creating the first bug file (which also creates the `bugs/` directory) produces no `UpdateMsg`, so the TUI does not refresh. The new bug only becomes visible when the user triggers a manual reload (e.g. `alt+a` to toggle show-all).

## Steps to Reproduce

1. Open a repo where no bug files have ever been created (`.maggus/bugs/` does not exist)
2. Run `maggus status`
3. In another terminal or via the bugreport skill, create a new bug file at `.maggus/bugs/bug_001.md` (this also creates the `bugs/` directory)
4. Observe: the TUI does not refresh to show the new bug
5. Press `alt+a` to toggle "show all" — the bug now appears

## Expected Behavior

Creating a bug file should trigger a plan reload in the TUI within the debounce window (~300 ms), regardless of whether the `bugs/` directory existed when the status command launched.

## Root Cause

`filewatcher.New()` at `src/internal/filewatcher/filewatcher.go:55-59` calls `fsw.Add(d)` only for directories that **already exist** at startup:

```go
for _, d := range dirs {
    if info, err := os.Stat(d); err == nil && info.IsDir() {
        _ = fsw.Add(d)
    }
}
```

When `.maggus/bugs/` is absent, it is silently skipped and never re-checked.

When the user later creates `bug_001.md`, the OS creates the `bugs/` subdirectory first, which fires a Create event in the `.maggus/` watcher. But `isRelevantEvent()` at line 205–220 only matches filenames like `bug_*.md`, `feature_*.md`, `feature_approvals.yml`, and `config.yml` — not the bare directory name `bugs`. The event is discarded, no `UpdateMsg` is sent, and the TUI never refreshes.

The subsequent `bug_001.md` creation inside `bugs/` also fires no event, because `.maggus/bugs/` was never added to `fsw`.

## User Stories

### BUG-051-001: Dynamically add newly-created watched directories to fsnotify

**Description:** As a user running `maggus status`, I want the TUI to refresh when I create the first bug file in a repo so that I don't have to manually trigger a reload.

**Acceptance Criteria:**
- [x] In `filewatcher.go` `loop()`, when a `fsnotify.Create` event is received and its `event.Name` matches one of the entries in `w.dirs`, call `w.fsw.Add(event.Name)` to start watching it
- [x] After adding the new directory, trigger the debounce timer (set `hasCreate = true`, reset/start the timer) so an `UpdateMsg` is sent even if no matching file event follows
- [x] Creating `bug_001.md` in a repo where `.maggus/bugs/` did not previously exist causes a `featureSummaryUpdateMsg` to be delivered and the TUI to reload within the debounce window
- [x] Creating a `feature_001.md` in a repo where `.maggus/features/` did not previously exist behaves the same way
- [x] No regression when directories already exist at startup (normal case still fires on file changes)
- [x] `go vet ./...` and `go test ./...` pass

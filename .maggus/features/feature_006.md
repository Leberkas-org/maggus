<!-- maggus-id: 7fc334c3-4ef4-4dd1-ac17-f8161db72048 -->
# Feature 006: CLI Cleanup — Remove Worktree Support & Hide TUI-Only Commands

## Introduction

Maggus currently exposes several commands that are only meaningful in an interactive TUI context (`config`, `prompt`, `repos`, `status`) as top-level CLI subcommands, adding noise to the help output and inviting misuse via scripting. Additionally, the `worktree` feature (isolated git worktrees per task) adds significant complexity across the codebase but is not actively used. This feature removes worktree support entirely and refactors the four TUI-only commands so they are only reachable through the interactive menu, not via direct CLI invocation.

### Architecture Context

- **Vision alignment:** Reduces surface area and cognitive overhead, keeping the CLI focused on the core `work` loop.
- **Components involved:** `cmd/` (all affected commands), `internal/config`, `internal/prompt`, `internal/worktree` (deleted), `internal/tasklock` (deleted).
- **New patterns:** Menu dispatch map in `root.go` for in-process TUI command invocation, bypassing cobra routing for TUI-only commands.

## Goals

- Remove `maggus worktree`, `maggus config`, `maggus prompt`, `maggus repos`, and `maggus status` from the CLI help output.
- Keep `config`, `prompt`, `repos`, and `status` accessible via the interactive menu (in-process dispatch, no cobra routing).
- Delete `internal/worktree` and `internal/tasklock` packages entirely.
- Remove the `Worktree` config field, `--worktree`/`--no-worktree` work flags, and all worktree-related prompt metadata.
- Leave `go build ./...` and `go test ./...` fully clean after all changes.

## Tasks

### TASK-006-001: Remove worktree plumbing from the work command

**Description:** As a developer, I want all worktree-related code removed from the work command files so that the internal/worktree and internal/tasklock packages have no remaining callers and can be deleted.

**Token Estimate:** ~60k tokens
**Predecessors:** none
**Successors:** TASK-006-002
**Parallel:** yes — can run alongside TASK-006-003

**Acceptance Criteria:**
- [x] `cmd/work.go`: `worktreeFlag` and `noWorktreeFlag` vars removed from the var block and from `resetWorkFlags()`; their flag registrations removed from `init()`; `cleanStaleWorktrees()` function deleted; `findNextUnlocked()` function deleted; `"github.com/leberkas-org/maggus/internal/worktree"` import removed.
- [x] `cmd/work_task.go`: `useWorktree bool` field removed from `taskContext`; lock acquisition block (`var lock tasklock.Lock` + `if tc.useWorktree { ... }`) removed from `runTask()`; all `releaseLock(lock, tc.useWorktree)` call sites removed; `completeTask()` signature updated to drop the `lock tasklock.Lock` parameter and its internal usage; `findNextWorkableTask()` simplified to `func findNextWorkableTask(tasks []parser.Task) *parser.Task` (remove `useWorktree bool` and `repoDir string` params, remove the `if useWorktree` branch, always call `parser.FindNextIncomplete(tasks)`); `releaseLock()` function deleted; `"github.com/leberkas-org/maggus/internal/tasklock"` import removed.
- [x] `cmd/work_setup.go`: `useWorktree bool` removed from `workConfig` struct; worktree flag resolution block (`useWorktree := cfg.Worktree` ... `if noWorktreeFlag { ... }`) removed from `workSetup()`; `useWorktree: useWorktree` removed from the returned `workConfig` literal; `setupBranch()` simplified — remove `useWorktree bool` parameter and the `if useWorktree { ... }` branch entirely, leaving only the auto-branch path; `"github.com/leberkas-org/maggus/internal/worktree"` import removed; unused `"path/filepath"` import removed if no longer needed.
- [x] `cmd/work.go`: `useWorktree: wc.useWorktree` removed from the `taskContext{...}` literal.
- [x] `cmd/work_task.go`: doc comment on `runTask` updated to remove the "acquires a lock (in worktree mode)" phrase.
- [x] `cmd/work_test.go`: all test functions testing `findNextUnlocked` deleted (approx. 4 functions, lines ~60–162).
- [x] `go build ./...` passes from `src/`.

---

### TASK-006-002: Delete worktree packages and clean up all worktree references

**Description:** As a developer, I want the internal/worktree and internal/tasklock packages deleted and every remaining reference to worktree (config field, prompt metadata, menu entry, init template, config TUI row) removed so the codebase contains no dead worktree code.

**Token Estimate:** ~70k tokens
**Predecessors:** TASK-006-001
**Successors:** none
**Parallel:** no

**Acceptance Criteria:**
- [x] `src/internal/worktree/` directory and all its files deleted.
- [x] `src/internal/tasklock/` directory and all its files deleted.
- [x] `cmd/worktree.go` deleted.
- [x] `cmd/worktree_test.go` deleted.
- [x] `cmd/menu_model.go`: `{name: "worktree", ...}` entry removed from `allMenuItems`; `"worktree"` key removed from `buildSubMenus()` return value (if it was the only key, return an empty map); `case "worktree":` removed from `buildArgs()`; keyboard shortcut `'t'` freed.
- [x] `cmd/menu_test.go`: test assertions about the worktree sub-menu removed.
- [x] `internal/config/config.go`: `Worktree bool \`yaml:"worktree"\`` field removed from the `Config` struct.
- [x] `internal/config/config_test.go`: all test cases that set `worktree: true` or `worktree: false` in YAML fixtures and assert on `cfg.Worktree` removed.
- [x] `cmd/init.go`: `# worktree: false` line (or equivalent) removed from the embedded config template.
- [x] `cmd/config.go`: `worktreeValues`, `worktreeIdx` variables removed; the worktree toggle row removed from the config builder; the `worktree:` write-back removed from the YAML serialisation block.
- [x] `cmd/config_test.go`: all test assertions referencing the worktree row or `worktree` YAML key removed.
- [x] `internal/prompt/prompt.go`: `Worktree bool` and `WorktreeDir string` fields removed from `Options`; the `if opts.Worktree { ... }` block removed from `writeMetadata()`; the `if opts.Worktree { ... }` block removed from `writeInstructions()`.
- [x] `internal/prompt/prompt_test.go`: test cases that set `opts.Worktree = true` or assert worktree strings in prompt output removed.
- [x] `go build ./...` and `go test ./...` pass from `src/`.

---

### TASK-006-003: Refactor menu to dispatch TUI commands directly; remove cobra registrations

**Description:** As a user, I want `config`, `prompt`, `repos`, and `status` to be accessible only through the interactive menu (not as direct CLI subcommands) so that `maggus --help` lists only the core workflow commands.

**Token Estimate:** ~50k tokens
**Predecessors:** none
**Successors:** none
**Parallel:** yes — can run alongside TASK-006-001

**Acceptance Criteria:**
- [ ] `cmd/config.go`: `var configCmd = &cobra.Command{...}` struct and `func init() { rootCmd.AddCommand(configCmd) }` removed; the existing `runConfig(cmd *cobra.Command, args []string) error` function signature simplified to `runConfig() error` (no cobra args needed — it already ignores them); cobra import removed if unused.
- [ ] `cmd/prompt.go`: same pattern — remove `promptCmd` struct and its `init()` registration; simplify run function to no-cobra signature.
- [ ] `cmd/repos.go`: same pattern — remove `reposCmd` struct and its `init()` registration; simplify run function.
- [ ] `cmd/status_cmd.go`: remove `var statusCmd = &cobra.Command{...}` struct and `func init() { rootCmd.AddCommand(statusCmd) ... }` block; extract the RunE body into `runStatus() error` that always uses default flag values (`plain=false`, `all=false`, `showLog=false`) — the plain/all/show-log flags only mattered for CLI piping which is no longer supported.
- [ ] `cmd/root.go` (`runMenu` function): add a direct dispatch map **before** the `rootCmd.Find(cmdArgs)` call:
  ```go
  directCmds := map[string]func() error{
      "config": runConfig,
      "prompt": runPrompt,
      "repos":  runRepos,
      "status": runStatus,
  }
  if fn, ok := directCmds[final.selected]; ok {
      _ = fn()
      continue
  }
  ```
- [ ] `maggus --help` no longer lists `config`, `prompt`, `repos`, or `status`.
- [ ] Running `maggus` → menu → selecting `status` still opens the status TUI.
- [ ] Running `maggus` → menu → selecting `config` still opens the config TUI.
- [ ] `go build ./...` and `go test ./...` pass from `src/`.

## Task Dependency Graph

```
TASK-006-001 ──→ TASK-006-002
TASK-006-003 (independent)
```

| Task | Estimate | Predecessors | Parallel | Model |
|------|----------|--------------|----------|-------|
| TASK-006-001 | ~60k | none | yes (with 003) | — |
| TASK-006-002 | ~70k | 001 | no | — |
| TASK-006-003 | ~50k | none | yes (with 001) | — |

**Total estimated tokens:** ~180k

## Functional Requirements

- FR-1: `maggus --help` must not list `config`, `prompt`, `repos`, `status`, or `worktree`.
- FR-2: `maggus config` (direct CLI invocation) must return "unknown command" or similar — it is no longer a registered subcommand.
- FR-3: Selecting `config`, `prompt`, `repos`, or `status` from the interactive menu must open the corresponding TUI view as before.
- FR-4: The `maggus work` command must accept no `--worktree` or `--no-worktree` flags.
- FR-5: `internal/worktree` and `internal/tasklock` packages must not exist in the repository.
- FR-6: The `Config` struct must not contain a `Worktree` field; `config.yml` files with `worktree:` keys are silently ignored by the YAML parser (yaml.v3 behaviour with unknown fields).
- FR-7: Agent prompts must contain no worktree-related metadata.

## Non-Goals

- Re-implementing worktree support in a cleaner form (deferred to a future feature).
- Removing the `--plain`, `--all`, or `--show-log` behaviour from the status view itself (the TUI still supports all modes; they just can't be set via CLI flags anymore).
- Adding new menu navigation or keyboard shortcuts for the four TUI commands beyond what already exists.
- Changing the behaviour of any other command (`work`, `plan`, `approve`, etc.).

## Technical Considerations

- `setupBranch()` in `work_setup.go` previously had two paths: worktree creation and auto-branch creation. After removing worktree, it always takes the auto-branch path. The `useWorktree bool` parameter is dropped entirely.
- `findNextWorkableTask()` previously had three paths: `--task` flag, worktree (unlocked), default. After removal it has two: `--task` flag, default. The function signature shrinks to `(tasks []parser.Task) *parser.Task`.
- `completeTask()` takes a `tasklock.Lock` parameter today. This must be removed; search for all call sites.
- The status command currently reads `--plain`, `--all`, and `--show-log` flags via `cmd.Flags().GetBool()`. These cobra references must be removed when extracting the function. The TUI default (`plain=false, all=false, showLog=false`) is always used from the menu.
- `prompt.go` (cmd) conditionally adds `promptCmd` only when `caps.HasClaude` is true (in `Execute()`). After this change, the direct dispatch map in `runMenu` should still gate the `prompt` entry on `caps.HasClaude` to preserve this behaviour.

## Success Metrics

- `maggus --help` output is visibly shorter (5 fewer commands listed).
- `go build ./...` and `go test ./...` are green with zero worktree-related test files or source files remaining.
- `git grep -r "internal/worktree\|internal/tasklock"` returns no results.

## Open Questions

_(none)_

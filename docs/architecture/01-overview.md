# Maggus — Architecture Overview

Maggus is a standalone AI agent orchestrator. It reads plan files from disk, manages a task queue with approval, dispatches work to AI agents (Claude Code, OpenCode), and handles all git operations (branching, worktrees, merging, cleanup). It supports parallel work across multiple repositories.

Bryan (C# gRPC backend) is an **optional** enhancement for distributed coordination, remote task dispatch, live dashboards, and cross-machine memory sync. Maggus works fully without it.

**Tech Stack:** Go 1.25+, `bubbletea` (TUI), `lipgloss` (styling), `cobra` (CLI), optionally `google.golang.org/grpc` (Bryan client)

---

## CLI Design

| Command | Behavior |
|---------|----------|
| `maggus` | Start daemon if not running + attach TUI. If daemon already running, just attach TUI. |
| `maggus -d` | Start daemon in detached mode (no TUI). |
| `maggus stop <repo>` | Stop running tasks for that repo. Daemon stays alive. |
| `maggus stop -a` / `--all` | Stop everything (daemon + all tasks). |

**TUI quit behavior:** Closing the TUI (q/Ctrl+C) shows a modal: "Stop daemon or detach?" User picks one.

---

## Directory Layout

```
~/.maggus/                             # global (daemon-owned, repo-independent)
  config.yml                           # global config (agent, model, auto_approve, max_workers)
  daemon.pid                           # daemon PID
  state.json                           # daemon state snapshot (IPC for TUI)
  keys/                                # RSA keys for Bryan auth (only if Bryan enabled)
  logs/                                # daemon logs

<repo>/.maggus/                        # per-repo state
  config.yml                           # repo-specific overrides
  logs/                                # run/task logs
  tasks/                               # plan files (source of truth for work items)
  MEMORY.md                            # project memory
```

---

## Core Flow (Standalone)

```
1. User creates plan file in <repo>/.maggus/tasks/
2. Daemon detects new file (fsnotify)
3. Parser reads plan → extracts feature context + tasks
4. If auto_approve: items go straight to "ready" queue
   If not: items appear as "pending" in TUI, user approves/reorders
5. Daemon picks highest-priority ready item
6. For each task in the item:
   a. Create branch + worktree
   b. Build prompt (task + feature context + MEMORY.md)
   c. Invoke agent subprocess in worktree
   d. Stream output → state (TUI) + local log
   e. Commit, rebase, merge into feature branch
   f. Cleanup worktree + task branch
7. Item complete → mark plan file as done
```

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Standalone first, Bryan optional | Maggus must work without any server. Bryan adds distributed features on top. |
| Plan files are source of truth | No database. A plan file contains everything a worker needs. |
| Workers are goroutines, not OS sub-processes | Simpler state sharing (mutex), lower overhead. Agent subprocess is the real OS process. |
| Single `git` package | Avoids import cycles between gitbranch/gitworktree/gitmerge. Files separate concerns. |
| `OutputSink` interface, not `tea.Msg` | Decouples agent from bubbletea. Worker fans out to state + log simultaneously. |
| File-based IPC (`~/.maggus/state.json`) | Cross-platform, simple, proven. TUI is a separate process. |
| `Commander` interface for git | All git operations testable via mocks without filesystem. |
| `Pane` interface + `BasePane` embedding | Shared layout/focus logic. All panes are modular and replaceable. |
| `~/.maggus/` as global home | Simple cross-platform via `os.UserHomeDir()`. Daemon is repo-independent. |
| Auto-approve via config | `auto_approve: true` skips manual approval. Default: false (explicit approval). |

# Plan: Repository Switcher

## Introduction

Add a repository switching feature to Maggus so users can manage multiple projects from a single TUI session. Repositories are stored in a global config file (`~/.maggus/repositories.yml`), and users can add/remove them via a TUI file browser. On startup, Maggus determines which repo to use based on the current working directory: if it matches a configured repo, use it; if it's an unconfigured git repo, offer to add it; otherwise, switch to the last-opened repo.

## Goals

- Allow users to switch between multiple project directories without restarting Maggus
- Persist a list of known repositories in a global config file (`~/.maggus/repositories.yml`)
- Provide a TUI file browser for navigating to and selecting new repositories
- Implement smart startup behavior based on the current working directory
- Keep the UX consistent with existing menu patterns (cursor navigation, styled output)

## User Stories

### TASK-001: Global repositories config file
**Description:** As a user, I want my list of known repositories stored in a global config file so that it persists across sessions and projects.

**Acceptance Criteria:**
- [x] New file `~/.maggus/repositories.yml` stores an array of repository entries
- [x] Each entry contains at minimum: `path` (absolute directory path)
- [x] A `last_opened` field tracks which repository was last used
- [x] New package `internal/globalconfig` handles loading, saving, and modifying this file
- [x] The `~/.maggus/` directory is created automatically if it doesn't exist
- [x] Loading a non-existent file returns an empty config (no error)
- [x] Unit tests cover load, save, add, remove, and last-opened operations

### TASK-002: Startup working directory resolution
**Description:** As a user, I want Maggus to intelligently determine which repository to work in on startup so that I land in the right project automatically.

**Acceptance Criteria:**
- [x] On startup, Maggus checks if the current working directory matches a configured repository — if yes, use it
- [x] If the current directory is a git repo but NOT in the configured list, prompt the user: "Add this repository?" (yes adds it and continues, no continues without adding)
- [x] If the current directory is neither a configured repo nor a git repo, switch to the `last_opened` repository from the global config
- [x] If `last_opened` is set but the path no longer exists, fall back to the first available configured repo
- [x] If no repositories are configured at all, stay in the current directory (existing behavior)
- [x] When a repository is selected (by any path), update `last_opened` in the global config
- [x] The actual `os.Chdir()` call is made so all subsequent operations (plan loading, config loading, git operations) work relative to the new directory
- [x] Unit tests cover all resolution branches (configured repo, unconfigured git repo, non-git directory, missing last_opened)

### TASK-003: TUI file browser for directory selection
**Description:** As a user, I want a TUI file browser so I can navigate the filesystem and select a repository directory to add.

**Acceptance Criteria:**
- [ ] New bubbletea model in `internal/tui/filebrowser/` implements a directory browser
- [ ] Browser starts in the current working directory
- [ ] Shows only directories (no files) in the listing, sorted alphabetically
- [ ] `..` entry at the top navigates to the parent directory
- [ ] Supports keyboard navigation: up/down to move, enter to descend into a directory, backspace to go up
- [ ] A "Select" action (e.g., pressing `s` or a dedicated button) confirms the current directory as the chosen repo
- [ ] Pressing `esc` cancels and returns without selection
- [ ] The current path is displayed at the top of the browser
- [ ] Directories starting with `.` are shown but visually muted
- [ ] The browser handles permission errors gracefully (shows error inline, doesn't crash)
- [ ] Long directory lists are scrollable with the viewport staying centered on the cursor
- [ ] Uses the existing styles package for consistent theming
- [ ] Unit tests cover navigation logic (enter, backspace, parent traversal, boundary cases)

### TASK-004: Repository menu item with sub-menu
**Description:** As a user, I want a "Repositories" entry in the main menu so I can switch between repos, add new ones, or remove existing ones.

**Acceptance Criteria:**
- [ ] New menu item "repos" added to the main menu between the project management group and exit
- [ ] Selecting "repos" opens a repository management screen (not a sub-menu with options, but a dedicated bubbletea view)
- [ ] The repository screen shows: a list of all configured repos with their paths, the currently active repo highlighted
- [ ] Actions available: "Switch" (enter on a repo), "Add" (opens the file browser from TASK-003), "Remove" (removes selected repo from config)
- [ ] Switching to a repo calls `os.Chdir()`, updates `last_opened`, reloads the plan summary, and returns to the main menu
- [ ] Adding a repo via the file browser validates that the selected directory is a git repo before adding
- [ ] If the selected directory is a git repo but has no `.maggus/` directory, ask whether to initialize it
- [ ] Removing a repo only removes it from the global config — does not delete any files on disk
- [ ] The current working directory is shown in the main menu header (e.g., below the plan summary line) so the user always knows which repo is active
- [ ] Uses the existing styles package for consistent theming

### TASK-005: Display active repository in menu header
**Description:** As a user, I want to see which repository I'm currently working in displayed in the main menu so I always have context.

**Acceptance Criteria:**
- [ ] The main menu header (below the plan summary line) shows the current repository path
- [ ] The path is displayed in a muted style to not distract from the main content
- [ ] Long paths are truncated from the left with `...` prefix to fit within the content width (e.g., `...org/maggus` instead of `/home/user/projects/leberkas-org/maggus`)
- [ ] If the current directory is the user's home directory or not a repo, show the full path without truncation
- [ ] The path updates after switching repositories (the menu reloads with the new working directory)

## Functional Requirements

- FR-1: The global config file must be located at `~/.maggus/repositories.yml`
- FR-2: The repositories list must be a YAML array with `path` fields containing absolute paths
- FR-3: The `last_opened` field must store the absolute path of the most recently used repository
- FR-4: The startup resolution order must be: configured repo > prompt for unconfigured git repo > last_opened fallback
- FR-5: The file browser must only show directories, never files
- FR-6: The file browser must start from the current working directory
- FR-7: `os.Chdir()` must be called when switching repos so all subsequent operations use the new directory
- FR-8: Adding a repository must validate it is a git repository (contains `.git/` directory or is inside a git worktree)
- FR-9: When switching to an uninitialized repo, the user must be prompted whether to run initialization
- FR-10: Removing a repository from the list must not delete any files from disk

## Non-Goals

- No repository cloning from remote URLs — only local directories
- No multi-repo parallel work — only one active repo at a time
- No repository status display (dirty/clean, branch info) in the repo list — keep it simple
- No drag-and-drop or mouse support in the file browser
- No repo aliasing or renaming — paths are the identifiers
- No automatic discovery/scanning of repos on the filesystem

## Technical Considerations

- The global config (`~/.maggus/`) is a new concept — currently all config is project-local in `.maggus/config.yml`
- `os.Chdir()` affects the entire process, which is fine since Maggus runs single-threaded in the menu loop
- The file browser should handle Windows paths (backslashes, drive letters) and Unix paths transparently using `filepath` package
- Git repo detection: check for `.git` directory or run `git rev-parse --is-inside-work-tree`
- The `runMenu` loop in `root.go` already reloads `loadPlanSummary()` each iteration, so switching repos will naturally refresh plan data
- YAML marshaling should use `gopkg.in/yaml.v3` consistent with existing config code

## Success Metrics

- User can add 3+ repositories and switch between them without restarting Maggus
- Starting Maggus in a configured repo directory lands directly in that project
- Starting Maggus in a non-repo directory automatically switches to the last-used project
- File browser allows navigating to any directory on the system from the starting point
- Repository list persists correctly across sessions

## Open Questions

- Should the repo list show a short name (directory basename) alongside the full path for readability?
- Should there be a limit on the number of configured repositories?
- Should the file browser support typing a path directly (text input) as an alternative to navigation?

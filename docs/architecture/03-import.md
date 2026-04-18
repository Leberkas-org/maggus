# Plan File Import & Task Splitting

Plan files are the source of truth for work items. They are dropped into a watched directory as plain markdown, then imported by the daemon: assigned a GUID, parsed, split into individual task files, and enqueued.

---

## Import Flow

```
1. User drops plan file into <repo>/.maggus/tasks/
   (or creates one via `maggus-plan` skill, CLI, etc.)

2. Daemon detects new file (fsnotify)

3. Import:
   a. Assign item_id (UUID v4)
   b. Parse header → context (goal, architecture, tech stack, file map)
   c. Parse body → individual tasks (## Task N sections)
   d. Create item directory: <repo>/.maggus/tasks/<item_id>/
   e. Write context.md, task-NNN.md files, item.yaml
   f. Move original file to <repo>/.maggus/tasks/<item_id>/source.md (archive)

4. Enqueue item:
   - If auto_approve: status = ready
   - If not: status = pending (user approves in TUI)
```

---

## Input Format

Plan files follow this structure (see `example/` for full examples):

```markdown
# <Title>

> **For agentic workers:** ...

**Goal:** <paragraph describing what to implement>

**Architecture:** <paragraph describing the design approach>

**Tech Stack:** <tools, frameworks, dependencies>

---

## File Map

<directory tree of files to create/modify>

---

## Task 1: <title>

**Files:**
- Create: `path/to/file.go`
- Modify: `path/to/other.go`

- [ ] **Step 1: <description>**

<code blocks, instructions, verification commands>

- [ ] **Step 2: <description>**

...

---

## Task 2: <title>

...
```

---

## Output Structure

After import, a plan file becomes:

```
<repo>/.maggus/tasks/<item_id>/
  source.md               # original plan file (archived, not modified)
  context.md              # extracted header: goal, architecture, tech stack, file map
  task-001.md             # content of "## Task 1: ..." section
  task-002.md             # content of "## Task 2: ..." section
  task-003.md             # ...
  item.yaml               # metadata and state
```

### context.md

The shared knowledge every task needs. Extracted from the header section (everything before the first `## Task`):

```markdown
# File Protocol Client Implementation Plan

**Goal:** Implement `DeviceHub.Protocols.File` — a shared file-based transport...

**Architecture:** `FileTransportClient` owns a `FileSystemWatcher`...

**Tech Stack:** `Akka.Streams` (1.5+), `System.Threading.Channels`...

## File Map

devicehub/
├── DeviceHub.Protocols.File/
│   ├── ...
```

### task-NNN.md

One file per task. Contains the exact content from its `## Task N:` section, including the title, files list, steps with checkboxes, code blocks, and verification commands.

```markdown
## Task 3: Implement WpcsParser

**Files:**
- Create: `DeviceHub.Protocols.File/Codecs/WpcsBlock.cs`
- Create: `DeviceHub.Protocols.File/Codecs/WpcsParser.cs`
- Create: `DeviceHub.Protocols.File.Tests/Codecs/WpcsParserTests.cs`

- [ ] **Step 1: Write failing tests**
...
```

### item.yaml

Metadata and state tracking:

```yaml
item_id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
title: "File Protocol Client Implementation Plan"
repo_url: "git@github.com:example/devicehub.git"
status: pending          # pending | ready | active | done | failed | skipped
priority: 0              # lower = higher priority, user-assignable
auto_approve: false
created_at: "2026-04-18T14:30:00Z"
source_file: "2026-04-16-file-protocol-client.md"

tasks:
  - id: "task-001"
    title: "Create the projects and add dependencies"
    status: pending      # pending | ready | active | done | failed | skipped
    file: "task-001.md"
  - id: "task-002"
    title: "Define RawFile"
    status: pending
    file: "task-002.md"
  - id: "task-003"
    title: "Implement WpcsParser"
    status: pending
    file: "task-003.md"
```

---

## Parser

```go
// internal/daemon/parser.go

type ParsedPlan struct {
    Title        string
    Context      string      // raw markdown of header section
    Tasks        []ParsedTask
}

type ParsedTask struct {
    Number  int
    Title   string
    Content string           // raw markdown of ## Task N section
}

// Parse splits a plan file into context + tasks.
func Parse(content string) (*ParsedPlan, error)
```

### Parsing Rules

1. **Title**: first `# ` line
2. **Context**: everything from start until the first `## Task` heading (excluding the agentic worker blockquote — that's an instruction for the skill, not context for the agent)
3. **Task split**: each `## Task N:` heading starts a new task, content runs until the next `## Task` or end of file
4. **Task title**: extracted from the `## Task N: <title>` heading
5. **Task numbering**: sequential from the heading number, mapped to `task-NNN` filenames

### Edge Cases

- Plan with 0 tasks → import error, reject file
- Plan with no header context → allowed (context.md will be minimal)
- Task without steps → allowed (worker will attempt it, may have nothing to do)
- Duplicate task numbers → renumber sequentially on import
- File map section (`## File Map`) is part of context, not a task

---

## Importer

```go
// internal/daemon/importer.go

type Importer struct {
    tasksDir string           // <repo>/.maggus/tasks/
}

// Import parses a plan file, creates the item directory structure,
// and returns the WorkItem ready for enqueue.
func (imp *Importer) Import(planPath string) (*WorkItem, error) {
    // 1. Read file
    // 2. Parse → ParsedPlan
    // 3. Generate item_id (UUID v4)
    // 4. Create <tasksDir>/<item_id>/
    // 5. Write context.md
    // 6. Write task-NNN.md for each task
    // 7. Write item.yaml
    // 8. Move original file to source.md
    // 9. Return WorkItem for queue
}
```

---

## How Workers Use Imported Files

When a worker picks up a task:

1. Load `context.md` into memory
2. Load the specific `task-NNN.md`
3. Build prompt: context + task content + MEMORY.md + bootstrap files
4. Pass prompt to agent

On task completion:
1. Update `item.yaml` → task status = done
2. If all tasks done → item status = done

On task failure:
1. Update `item.yaml` → task status = failed
2. Item stays active (remaining tasks may still run if independent)

---

## Re-import / Update

If a plan file is modified after import:
- The daemon detects the change but does **not** re-import automatically
- A modified `source.md` inside an existing item directory is ignored
- To re-import: user deletes the item directory and drops the file again
- Future: TUI could offer a "re-import" action that diffs and updates

---

## File Watcher Behavior

The daemon watches `<repo>/.maggus/tasks/` for:
- **New `.md` files** in the root → trigger import
- **Subdirectories** (item dirs) are not watched for new files — only `item.yaml` changes are tracked for status updates

Files that are not `.md` or are inside subdirectories are ignored by the importer.

---
name: maggus-bug
description: "Create a structured bug ticket for maggus to work on. Use when reporting a bug, filing a defect, or documenting unexpected behavior. Triggers on: file a bug, report bug, create bug ticket, something is broken"
user-invocable: true
---

# Bug Ticket Generator

Create structured, workable bug tickets that maggus can pick up and fix autonomously.

---

## The Job

1. Receive a bug description from the user
2. Investigate the codebase to understand the bug
3. Ask 2-4 targeted diagnostic questions (with lettered options)
4. Generate a structured bug ticket based on findings
5. Save to `.maggus/bugs/bug_[rolling_number].md`

**Important:** Do NOT start fixing the bug. Just create the bug ticket.

---

## Step 1: Investigate

Before asking questions, do your own research:

- Read the files/functions mentioned or implied by the user
- Check recent git history for related changes (`git log --oneline -20`)
- Look for related test files
- Try to identify the root cause or narrow down candidates

This investigation informs your questions — don't ask what you can figure out yourself.

---

## Step 2: Diagnostic Questions

Ask only what you couldn't determine from investigation. Focus on:

- **Reproduction:** Can you reproduce it? Under what conditions?
- **Scope:** Is this the only symptom, or are there others?
- **Regression:** Did this work before? When did it break?
- **Priority:** Is this blocking other work?

### Format Questions Like This:

```
1. When did this start happening?
   A. After the last refactoring commit
   B. It's always been this way
   C. After a specific change (please describe)
   D. Not sure

2. Is this blocking other work?
   A. Yes — can't proceed without fixing this
   B. No — annoying but workaround exists
   C. Partially — some tasks are affected
```

This lets users respond with "1A, 2B" for quick iteration. Remember to indent the options.

---

## Step 3: Bug Ticket Structure

Generate the ticket with these sections:

### Title
`# Bug: [Short descriptive title]`

Keep it specific. "Tab switching broken" not "UI doesn't work".

### Summary
1-2 sentences. What happens vs. what should happen. No fluff.

### Related (Optional)
Link to related plan, task, or commit if applicable. Format:
```markdown
## Related

- **Plan:** plan_3.md
- **Task:** TASK-001
- **Commit:** 426494a
```

Only include fields that are relevant. Omit the section entirely if standalone.

### Steps to Reproduce
Numbered steps. Be specific enough that maggus (or a developer) can reproduce it.

### Expected Behavior
What should happen instead.

### Root Cause
This is the most important section. Based on your investigation:
- Reference specific files and line numbers
- Explain WHY the bug occurs, not just WHAT happens
- If uncertain, list candidate causes ranked by likelihood

### User Stories
Each fix should be a `TASK-NNN` that maggus can work on:

```markdown
### TASK-001: [Fix title]

**Description:** As a [user/developer], I want [fix] so that [expected behavior is restored].

**Acceptance Criteria:**
- [ ] [Specific verifiable fix criterion]
- [ ] [Another criterion]
- [ ] No regression in related functionality
- [ ] `go vet ./...` and `go test ./...` pass
```

**Rules for bug fix tasks:**
- Keep tasks small — one logical fix per task
- Always include a regression check criterion
- Always include `go vet` and `go test` criteria
- If the root cause is clear, one task is usually enough
- If multiple independent things are broken, use multiple tasks

---

## Writing Style

- Be direct and technical. No hedging, no "it seems like".
- Reference exact file paths and line numbers.
- Use code blocks for code references.
- If you found the root cause, state it with confidence.
- If you're uncertain, say "Candidate causes:" and list them.

---

## Output

- **Format:** Markdown (`.md`)
- **Location:** `.maggus/bugs/`
- **Filename:** `bug_[rolling-number].md` (e.g., `bug_1.md`, `bug_2.md`)
- **Numbering:** Check existing `.maggus/bugs/bug_*.md` files and use the next number

---

## Example Bug Ticket

```markdown
# Bug: Tab switching broken in work view after TUI refactoring

## Summary

Arrow keys and number keys (1-4) no longer switch tabs in the work view. The active tab stays on the first tab regardless of input. This regressed after the TUI refactoring in commit 426494a.

## Related

- **Commit:** 426494a (refactor(tui): delegate Update() to sub-component handlers)

## Steps to Reproduce

1. Run `maggus work`
2. Wait for a task to start (so tabs become active)
3. Press right arrow or number keys 2/3/4
4. Observe: active tab does not change

## Expected Behavior

Pressing right arrow or number keys should switch between the Output, Detail, Task, and Tokens tabs.

## Root Cause

The refactoring extracted `handleTabSwitch` as a value-receiver method at `src/internal/runner/tui_keys.go:172`:

    func (m TUIModel) handleTabSwitch(msg tea.KeyMsg) (tea.Cmd, bool)

This method modifies `m.activeTab` on a **copy** of the model. The caller at line 50 receives `handled=true` but the original `m` is unchanged — the tab switch is silently discarded.

The same bug affects `handleDetailScroll` (line 145) which modifies scroll offsets on a copy.

## User Stories

### TASK-001: Fix value-receiver mutations in tui_keys.go

**Description:** As a user, I want tab switching and detail scrolling to work in the work view so I can monitor different aspects of running tasks.

**Acceptance Criteria:**
- [ ] `handleTabSwitch` uses pointer receiver `*TUIModel`
- [ ] `handleDetailScroll` uses pointer receiver `*TUIModel`
- [ ] `handleKeyMsg` uses pointer receiver `*TUIModel`
- [ ] Tab switching works with arrow keys and number keys 1-4
- [ ] Detail panel scrolling works on the detail tab
- [ ] Stop picker (Alt+S) still works
- [ ] No regression in summary screen behavior
- [ ] `go vet ./...` and `go test ./...` pass
```

---

## Checklist

Before saving the bug ticket:

- [ ] Investigated the codebase (read relevant files, checked git history)
- [ ] Asked diagnostic questions only for things you couldn't determine
- [ ] Incorporated user's answers
- [ ] Root cause section references specific files and line numbers
- [ ] Tasks are small and have verifiable acceptance criteria
- [ ] Every task includes regression check and test criteria
- [ ] Saved to `.maggus/bugs/bug_[N].md`

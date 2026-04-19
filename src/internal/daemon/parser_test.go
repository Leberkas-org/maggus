package daemon

import (
	"testing"
)

const samplePlan = `# File Protocol Implementation

> **For agentic workers:** Follow these steps precisely.

**Goal:** Implement the file protocol transport.

**Architecture:** FileTransportClient owns a watcher.

**Tech Stack:** Go 1.25+

---

## File Map

src/
├── transport/
│   └── file.go

---

## Task 1: Create project structure

**Files:**
- Create: ` + "`src/transport/file.go`" + `

- [ ] **Step 1: Create the file**

Write the transport interface.

---

## Task 2: Implement watcher

**Files:**
- Create: ` + "`src/transport/watcher.go`" + `

- [ ] **Step 1: Write failing tests**

Test the watcher.

## Task 3: Integration tests

- [ ] **Step 1: Write integration tests**
`

func TestParse_Basic(t *testing.T) {
	plan, err := Parse(samplePlan)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if plan.Title != "File Protocol Implementation" {
		t.Errorf("title = %q", plan.Title)
	}

	if len(plan.Tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(plan.Tasks))
	}

	if plan.Tasks[0].Title != "Create project structure" {
		t.Errorf("task[0].title = %q", plan.Tasks[0].Title)
	}
	if plan.Tasks[1].Title != "Implement watcher" {
		t.Errorf("task[1].title = %q", plan.Tasks[1].Title)
	}
	if plan.Tasks[2].Title != "Integration tests" {
		t.Errorf("task[2].title = %q", plan.Tasks[2].Title)
	}

	// Context should not contain the agentic blockquote
	if contains(plan.Context, "For agentic workers") {
		t.Error("context should not contain agentic blockquote")
	}

	// Context should contain goal, architecture, file map
	if !contains(plan.Context, "Goal:") {
		t.Error("context should contain Goal")
	}
	if !contains(plan.Context, "File Map") {
		t.Error("context should contain File Map")
	}
}

func TestParse_NoTasks(t *testing.T) {
	_, err := Parse("# Title\n\nJust a description, no tasks.")
	if err == nil {
		t.Error("expected error for plan with no tasks")
	}
}

func TestParse_NoHeader(t *testing.T) {
	plan, err := Parse("## Task 1: Only task\n\nDo something.")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Errorf("tasks = %d, want 1", len(plan.Tasks))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

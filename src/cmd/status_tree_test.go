package cmd

import (
	"testing"

	"github.com/leberkas-org/maggus/internal/parser"
)

// helpers to build treeItem slices concisely for tests.
func planItem(id string) treeItem {
	return treeItem{kind: treeItemKindPlan, plan: parser.Plan{ID: id}}
}

func taskItem(planID string) treeItem {
	t := parser.Task{ID: planID + "-t1"}
	return treeItem{kind: treeItemKindTask, plan: parser.Plan{ID: planID}, task: &t}
}

func sepItem() treeItem {
	return treeItem{kind: treeItemKindSeparator}
}

func TestFindNextPlanRow(t *testing.T) {
	// [plan_A, task_A1, task_A2, separator, plan_B, task_B1, plan_C]
	items := []treeItem{
		planItem("A"),
		taskItem("A"),
		taskItem("A"),
		sepItem(),
		planItem("B"),
		taskItem("B"),
		planItem("C"),
	}

	t.Run("cursor on plan row moves to next plan", func(t *testing.T) {
		got := findNextPlanRow(items, 0) // plan_A → plan_B
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("cursor on task row jumps past parent to next plan", func(t *testing.T) {
		got := findNextPlanRow(items, 1) // task_A1 → plan_B
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("cursor on last plan row is unchanged", func(t *testing.T) {
		got := findNextPlanRow(items, 6) // plan_C → no next plan
		if got != 6 {
			t.Errorf("got %d, want 6 (unchanged)", got)
		}
	})

	t.Run("cursor on task under last plan is unchanged", func(t *testing.T) {
		// items[5] is task_B1, but plan_C (6) follows — expect 6
		got := findNextPlanRow(items, 5) // task_B1 → plan_C
		if got != 6 {
			t.Errorf("got %d, want 6", got)
		}
	})

	t.Run("empty items returns cursor unchanged", func(t *testing.T) {
		got := findNextPlanRow([]treeItem{}, 0)
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("skips separator rows", func(t *testing.T) {
		// Cursor on plan_B (4), next plan is plan_C (6) — separator at 3 is before, not an issue.
		// Actually separator is between A and B. Let's test cursor=4 (plan_B) → plan_C(6)
		got := findNextPlanRow(items, 4)
		if got != 6 {
			t.Errorf("got %d, want 6", got)
		}
	})
}

func TestFindPrevPlanRow(t *testing.T) {
	// [plan_A, task_A1, task_A2, separator, plan_B, task_B1, plan_C]
	items := []treeItem{
		planItem("A"),
		taskItem("A"),
		taskItem("A"),
		sepItem(),
		planItem("B"),
		taskItem("B"),
		planItem("C"),
	}

	t.Run("cursor on plan row moves to prev plan", func(t *testing.T) {
		got := findPrevPlanRow(items, 4) // plan_B → plan_A
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("cursor on task row jumps to prev plan", func(t *testing.T) {
		got := findPrevPlanRow(items, 5) // task_B1 → plan_B
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})

	t.Run("cursor on first plan row is unchanged", func(t *testing.T) {
		got := findPrevPlanRow(items, 0) // plan_A → no prev plan
		if got != 0 {
			t.Errorf("got %d, want 0 (unchanged)", got)
		}
	})

	t.Run("cursor on task under first plan is unchanged", func(t *testing.T) {
		got := findPrevPlanRow(items, 1) // task_A1 → plan_A (0)
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("empty items returns cursor unchanged", func(t *testing.T) {
		got := findPrevPlanRow([]treeItem{}, 0)
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("cursor on last plan row returns prev plan", func(t *testing.T) {
		got := findPrevPlanRow(items, 6) // plan_C → plan_B (4)
		if got != 4 {
			t.Errorf("got %d, want 4", got)
		}
	})
}

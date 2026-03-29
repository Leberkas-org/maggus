package cmd

import "github.com/leberkas-org/maggus/internal/parser"

type treeItemKind int

const (
	treeItemKindPlan treeItemKind = iota
	treeItemKindTask
	treeItemKindSeparator
)

// treeItem is a single row in the left-pane tree. Both plan-rows and task-rows
// carry the parent plan; task is non-nil only for task rows.
type treeItem struct {
	kind treeItemKind
	plan parser.Plan
	task *parser.Task // non-nil for task rows
}

// buildTreeItems returns the flat, ordered list of visible tree rows reflecting
// the current expand state. For each visible plan one plan-row is always emitted;
// if that plan's ID is in expandedPlans, one task-row per task in plan.Tasks
// (all tasks, including complete and blocked) is emitted immediately after.
// A treeItemKindSeparator is inserted at the boundary between bug plans and
// feature plans when both kinds are present.
// The method is cheap to call and allocates at most one slice.
func (m statusModel) buildTreeItems() []treeItem {
	visible := m.visiblePlans()
	items := make([]treeItem, 0, len(visible)+1)
	bugSeen := false
	sepInserted := false
	for _, p := range visible {
		if p.IsBug {
			bugSeen = true
		}
		// Insert separator when transitioning from bug rows to feature rows.
		if !p.IsBug && bugSeen && !sepInserted {
			sepInserted = true
			items = append(items, treeItem{kind: treeItemKindSeparator})
		}
		items = append(items, treeItem{kind: treeItemKindPlan, plan: p})
		if m.expandedPlans[p.ID] {
			for i := range p.Tasks {
				t := p.Tasks[i] // copy so each task-row has its own pointer
				items = append(items, treeItem{kind: treeItemKindTask, plan: p, task: &t})
			}
		}
	}
	return items
}

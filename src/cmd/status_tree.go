package cmd

import "github.com/leberkas-org/maggus/internal/parser"

type treeItemKind int

const (
	treeItemKindPlan treeItemKind = iota
	treeItemKindTask
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
// The method is cheap to call and allocates at most one slice.
func (m statusModel) buildTreeItems() []treeItem {
	visible := m.visiblePlans()
	items := make([]treeItem, 0, len(visible))
	for _, p := range visible {
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

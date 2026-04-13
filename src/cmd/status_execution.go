package cmd

import (
	"github.com/leberkas-org/maggus/internal/parser"
)

// executionStep represents one step in the ordered execution plan for a feature.
// A step is either a parallel batch (multiple tasks running simultaneously) or a
// sequential step (a single task running alone).
type executionStep struct {
	StepNumber   int      // 1-based step index
	TaskIDs      []string // task IDs assigned to this step
	Parallel     bool     // true if tasks in this step run in parallel
	CrossFeature bool     // true if this is the catch-all group for tasks with cross-feature predecessor references
}

// buildExecutionPlan computes an ordered execution plan from a list of tasks.
//
// featureDir is the path to the directory containing feature_*.md files (e.g.
// ".maggus/features/"). When non-empty, cross-feature predecessor completion is
// checked via IsFeatureComplete; tasks whose cross-feature deps are not yet
// complete are placed in the cross-feature group regardless of same-feature
// predecessor state. When featureDir is empty the cross-feature check is
// skipped and only same-feature unknown predecessors drive the grouping.
//
// The plan is built as follows:
//  1. Tasks whose predecessors reference unknown task IDs (i.e. cross-feature
//     references) or whose cross-feature deps are not yet satisfied are
//     collected into a final "cross-feature" step.
//  2. The remaining tasks are sorted into topological waves: wave 0 contains
//     tasks with no predecessors; wave k contains tasks whose all predecessors
//     are in waves 0..k-1.
//  3. Each wave becomes exactly one step containing all tasks in that wave.
//     A step is marked Parallel=true only when it contains 2 or more tasks
//     that all have Parallel==true, meaning they will genuinely run concurrently
//     in separate worktrees. A single Parallel==true task still gets
//     Parallel=false on the step because it runs alone with no concurrency.
//  4. Step numbers are 1-based and contiguous across all waves.
//  5. The cross-feature group (if non-empty) is appended as the final step.
func buildExecutionPlan(tasks []parser.Task, featureDir string) []executionStep {
	if len(tasks) == 0 {
		return nil
	}

	// Build a set of known task IDs for fast lookup.
	knownIDs := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		knownIDs[t.ID] = true
	}

	// Separate tasks into "resolved" (all predecessors known) and "unresolved".
	// A task is unresolved when:
	//   (a) at least one same-feature predecessor ID is not in the known task
	//       set (unknown cross-feature task reference), OR
	//   (b) featureDir is non-empty and at least one cross-feature predecessor
	//       feature is not yet complete (checked via IsFeatureComplete).
	// Uses PredecessorsSatisfied — the same logic as the runtime scheduler —
	// instead of a separate hasUnknownPredecessor check.
	var resolved []parser.Task
	var unresolved []parser.Task
	for _, t := range tasks {
		sameFeatureUnresolved := !t.PredecessorsSatisfied(knownIDs, knownIDs)
		crossFeatureUnresolved := featureDir != "" && !t.CrossFeaturePredecessorsSatisfied(featureDir)
		if sameFeatureUnresolved || crossFeatureUnresolved {
			unresolved = append(unresolved, t)
		} else {
			resolved = append(resolved, t)
		}
	}

	// Compute topological wave assignments for resolved tasks.
	// waveOf maps task ID → wave number (0-based).
	waveOf := computeWaves(resolved)

	// Determine the total number of waves.
	maxWave := -1
	for _, w := range waveOf {
		if w > maxWave {
			maxWave = w
		}
	}

	// Group resolved tasks by wave, preserving original order within each wave.
	waveGroups := make([][]parser.Task, maxWave+1)
	for _, t := range resolved {
		w := waveOf[t.ID]
		waveGroups[w] = append(waveGroups[w], t)
	}

	// Convert wave groups into execution steps — one step per wave.
	var steps []executionStep
	stepNum := 1

	for _, group := range waveGroups {
		taskIDs := make([]string, 0, len(group))
		parallelCount := 0
		for _, t := range group {
			taskIDs = append(taskIDs, t.ID)
			if t.Parallel {
				parallelCount++
			}
		}
		// Mark as parallel only when 2+ tasks will genuinely run concurrently.
		steps = append(steps, executionStep{
			StepNumber: stepNum,
			TaskIDs:    taskIDs,
			Parallel:   parallelCount >= 2,
		})
		stepNum++
	}

	// Append the cross-feature group as a final step if there are any.
	if len(unresolved) > 0 {
		ids := make([]string, 0, len(unresolved))
		for _, t := range unresolved {
			ids = append(ids, t.ID)
		}
		steps = append(steps, executionStep{
			StepNumber:   stepNum,
			TaskIDs:      ids,
			CrossFeature: true,
		})
	}

	return steps
}

// computeWaves assigns each resolved task to a topological wave (0-based).
// Wave 0 contains tasks with no predecessors. Wave k contains tasks whose all
// predecessors are in waves 0..k-1. Tasks that cannot be placed (cycles or
// dependencies on unresolved tasks) are assigned the next available wave number
// after all placeable tasks.
func computeWaves(tasks []parser.Task) map[string]int {
	waveOf := make(map[string]int, len(tasks))
	assigned := make(map[string]bool, len(tasks))

	// Index tasks by ID for fast lookup.
	byID := make(map[string]parser.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	wave := 0
	for len(assigned) < len(tasks) {
		var batch []string
		for _, t := range tasks {
			if assigned[t.ID] {
				continue
			}
			if allPredecessorsAssigned(t, assigned) {
				batch = append(batch, t.ID)
			}
		}

		if len(batch) == 0 {
			// Cycle or unreachable tasks — assign remaining to current wave and break.
			for _, t := range tasks {
				if !assigned[t.ID] {
					waveOf[t.ID] = wave
					assigned[t.ID] = true
				}
			}
			break
		}

		for _, id := range batch {
			waveOf[id] = wave
			assigned[id] = true
		}
		wave++
	}

	return waveOf
}

// allPredecessorsAssigned returns true if every predecessor of t is already in
// the assigned set.
func allPredecessorsAssigned(t parser.Task, assigned map[string]bool) bool {
	for _, pred := range t.Predecessors {
		if !assigned[pred] {
			return false
		}
	}
	return true
}

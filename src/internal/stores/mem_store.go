package stores

import (
	"fmt"
	"strings"

	"github.com/leberkas-org/maggus/internal/parser"
)

// memStore is a shared in-memory backing store used by both MemFeatureStore and MemBugStore.
type memStore struct {
	plans []parser.Plan
}

func newMemStore(plans []parser.Plan) *memStore {
	cp := make([]parser.Plan, len(plans))
	copy(cp, plans)
	// Deep-copy tasks and criteria so mutations don't affect the caller's slice.
	for i, p := range cp {
		tasks := make([]parser.Task, len(p.Tasks))
		copy(tasks, p.Tasks)
		for j, t := range tasks {
			criteria := make([]parser.Criterion, len(t.Criteria))
			copy(criteria, t.Criteria)
			tasks[j].Criteria = criteria
		}
		cp[i].Tasks = tasks
	}
	return &memStore{plans: cp}
}

func (m *memStore) loadAll(includeCompleted bool) ([]parser.Plan, error) {
	result := make([]parser.Plan, 0, len(m.plans))
	for _, p := range m.plans {
		if !includeCompleted && p.Completed {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

func (m *memStore) markCompleted(_ string) ([]string, error) {
	var ids []string
	for i := range m.plans {
		p := &m.plans[i]
		if p.Completed || len(p.Tasks) == 0 {
			continue
		}
		allDone := true
		for _, t := range p.Tasks {
			if !t.IsComplete() || t.IsBlocked() {
				allDone = false
				break
			}
		}
		if allDone {
			p.Completed = true
			ids = append(ids, p.ID)
		}
	}
	return ids, nil
}

func (m *memStore) globFiles(includeCompleted bool) ([]string, error) {
	var files []string
	for _, p := range m.plans {
		if !includeCompleted && p.Completed {
			continue
		}
		files = append(files, p.File)
	}
	return files, nil
}

func (m *memStore) deleteTask(filePath, taskID string) error {
	for i := range m.plans {
		if m.plans[i].File != filePath {
			continue
		}
		tasks := m.plans[i].Tasks
		for j, t := range tasks {
			if t.ID == taskID {
				m.plans[i].Tasks = append(tasks[:j], tasks[j+1:]...)
				return nil
			}
		}
		return fmt.Errorf("task %s not found in %s", taskID, filePath)
	}
	return fmt.Errorf("plan not found for file: %s", filePath)
}

func (m *memStore) findCriterion(filePath string, c parser.Criterion) (pi, ti, ci int, err error) {
	for pi = range m.plans {
		if m.plans[pi].File != filePath {
			continue
		}
		for ti = range m.plans[pi].Tasks {
			for ci, cr := range m.plans[pi].Tasks[ti].Criteria {
				if cr.Text == c.Text && cr.Checked == c.Checked && cr.Blocked == c.Blocked {
					return pi, ti, ci, nil
				}
			}
		}
	}
	return 0, 0, 0, fmt.Errorf("criterion not found in %s: %s", filePath, c.Text)
}

func unblockText(text string) string {
	if strings.HasPrefix(text, "⚠️ BLOCKED: ") {
		return strings.TrimPrefix(text, "⚠️ BLOCKED: ")
	}
	return strings.TrimPrefix(text, "BLOCKED: ")
}

func (m *memStore) unblockCriterion(filePath string, c parser.Criterion) error {
	pi, ti, ci, err := m.findCriterion(filePath, c)
	if err != nil {
		return err
	}
	cr := &m.plans[pi].Tasks[ti].Criteria[ci]
	cr.Text = unblockText(cr.Text)
	cr.Blocked = false
	return nil
}

func (m *memStore) resolveCriterion(filePath string, c parser.Criterion) error {
	pi, ti, ci, err := m.findCriterion(filePath, c)
	if err != nil {
		return err
	}
	cr := &m.plans[pi].Tasks[ti].Criteria[ci]
	cr.Text = unblockText(cr.Text)
	cr.Blocked = false
	cr.Checked = true
	return nil
}

func (m *memStore) deleteCriterion(filePath string, c parser.Criterion) error {
	pi, ti, ci, err := m.findCriterion(filePath, c)
	if err != nil {
		return err
	}
	criteria := m.plans[pi].Tasks[ti].Criteria
	m.plans[pi].Tasks[ti].Criteria = append(criteria[:ci], criteria[ci+1:]...)
	return nil
}

// MemFeatureStore is an in-memory fake implementation of FeatureStore for use in tests.
type MemFeatureStore struct{ s *memStore }

// Compile-time assertion that MemFeatureStore satisfies FeatureStore.
var _ FeatureStore = &MemFeatureStore{}

// NewMemFeatureStore creates a MemFeatureStore seeded with the given plans.
func NewMemFeatureStore(plans []parser.Plan) *MemFeatureStore {
	return &MemFeatureStore{s: newMemStore(plans)}
}

func (f *MemFeatureStore) LoadAll(includeCompleted bool) ([]parser.Plan, error) {
	return f.s.loadAll(includeCompleted)
}

func (f *MemFeatureStore) MarkCompleted(action string) ([]string, error) {
	return f.s.markCompleted(action)
}

func (f *MemFeatureStore) GlobFiles(includeCompleted bool) ([]string, error) {
	return f.s.globFiles(includeCompleted)
}

func (f *MemFeatureStore) DeleteTask(filePath, taskID string) error {
	return f.s.deleteTask(filePath, taskID)
}

func (f *MemFeatureStore) UnblockCriterion(filePath string, c parser.Criterion) error {
	return f.s.unblockCriterion(filePath, c)
}

func (f *MemFeatureStore) ResolveCriterion(filePath string, c parser.Criterion) error {
	return f.s.resolveCriterion(filePath, c)
}

func (f *MemFeatureStore) DeleteCriterion(filePath string, c parser.Criterion) error {
	return f.s.deleteCriterion(filePath, c)
}

// MemBugStore is an in-memory fake implementation of BugStore for use in tests.
type MemBugStore struct{ s *memStore }

// Compile-time assertion that MemBugStore satisfies BugStore.
var _ BugStore = &MemBugStore{}

// NewMemBugStore creates a MemBugStore seeded with the given plans.
func NewMemBugStore(plans []parser.Plan) *MemBugStore {
	return &MemBugStore{s: newMemStore(plans)}
}

func (b *MemBugStore) LoadAll(includeCompleted bool) ([]parser.Plan, error) {
	return b.s.loadAll(includeCompleted)
}

func (b *MemBugStore) MarkCompleted(action string) ([]string, error) {
	return b.s.markCompleted(action)
}

func (b *MemBugStore) GlobFiles(includeCompleted bool) ([]string, error) {
	return b.s.globFiles(includeCompleted)
}

func (b *MemBugStore) DeleteTask(filePath, taskID string) error {
	return b.s.deleteTask(filePath, taskID)
}

func (b *MemBugStore) UnblockCriterion(filePath string, c parser.Criterion) error {
	return b.s.unblockCriterion(filePath, c)
}

func (b *MemBugStore) ResolveCriterion(filePath string, c parser.Criterion) error {
	return b.s.resolveCriterion(filePath, c)
}

func (b *MemBugStore) DeleteCriterion(filePath string, c parser.Criterion) error {
	return b.s.deleteCriterion(filePath, c)
}

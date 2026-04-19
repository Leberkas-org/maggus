package daemon

import "sync"

type ItemStatus string

const (
	ItemPending ItemStatus = "pending"
	ItemReady   ItemStatus = "ready"
	ItemActive  ItemStatus = "active"
	ItemDone    ItemStatus = "done"
	ItemFailed  ItemStatus = "failed"
	ItemSkipped ItemStatus = "skipped"
)

type WorkItem struct {
	ID          string
	PlanFile    string
	RepoURL     string
	RepoPath    string
	Title       string
	Description string
	Tasks       []TaskSpec
	Status      ItemStatus
	Priority    int
}

type TaskSpec struct {
	ID           string
	Title        string
	Content      string
	Status       ItemStatus
	Predecessors []string
}

type TaskQueue struct {
	items []*WorkItem
	mu    sync.RWMutex
}

func NewTaskQueue() *TaskQueue {
	return &TaskQueue{}
}

func (q *TaskQueue) Enqueue(item *WorkItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

func (q *TaskQueue) Approve(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, item := range q.items {
		if item.ID == id {
			switch item.Status {
			case ItemPending, ItemFailed:
				item.Status = ItemReady
				// Reset failed tasks so they can be retried
				for i := range item.Tasks {
					if item.Tasks[i].Status == ItemFailed {
						item.Tasks[i].Status = ItemPending
					}
				}
				return nil
			case ItemReady, ItemActive:
				return nil // already approved/running
			}
		}
	}
	return errItemNotFound(id)
}

func (q *TaskQueue) Skip(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, item := range q.items {
		if item.ID == id {
			item.Status = ItemSkipped
			return nil
		}
	}
	return errItemNotFound(id)
}

func (q *TaskQueue) Reorder(id string, newPriority int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, item := range q.items {
		if item.ID == id {
			item.Priority = newPriority
			return nil
		}
	}
	return errItemNotFound(id)
}

func (q *TaskQueue) NextReady() *WorkItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	var best *WorkItem
	for _, item := range q.items {
		if item.Status != ItemReady {
			continue
		}
		if best == nil || item.Priority < best.Priority {
			best = item
		}
	}
	return best
}

func (q *TaskQueue) Pending() []*WorkItem {
	q.mu.RLock()
	defer q.mu.RUnlock()
	var out []*WorkItem
	for _, item := range q.items {
		if item.Status == ItemPending {
			out = append(out, item)
		}
	}
	return out
}

func (q *TaskQueue) All() []*WorkItem {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]*WorkItem, len(q.items))
	copy(out, q.items)
	return out
}

func (q *TaskQueue) Get(id string) *WorkItem {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, item := range q.items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

type errItemNotFound string

func (e errItemNotFound) Error() string {
	return "item not found: " + string(e)
}

package daemon

import "testing"

func TestTaskQueue_EnqueueAndAll(t *testing.T) {
	q := NewTaskQueue()
	q.Enqueue(&WorkItem{ID: "a", Status: ItemPending})
	q.Enqueue(&WorkItem{ID: "b", Status: ItemReady})

	all := q.All()
	if len(all) != 2 {
		t.Fatalf("got %d items, want 2", len(all))
	}
}

func TestTaskQueue_Approve(t *testing.T) {
	q := NewTaskQueue()
	q.Enqueue(&WorkItem{ID: "a", Status: ItemPending})

	if err := q.Approve("a"); err != nil {
		t.Fatal(err)
	}
	item := q.Get("a")
	if item.Status != ItemReady {
		t.Errorf("status = %s, want ready", item.Status)
	}
}

func TestTaskQueue_ApproveNotFound(t *testing.T) {
	q := NewTaskQueue()
	if err := q.Approve("nope"); err == nil {
		t.Error("expected error")
	}
}

func TestTaskQueue_Skip(t *testing.T) {
	q := NewTaskQueue()
	q.Enqueue(&WorkItem{ID: "a", Status: ItemPending})

	if err := q.Skip("a"); err != nil {
		t.Fatal(err)
	}
	if q.Get("a").Status != ItemSkipped {
		t.Error("expected skipped")
	}
}

func TestTaskQueue_NextReady_Priority(t *testing.T) {
	q := NewTaskQueue()
	q.Enqueue(&WorkItem{ID: "low", Status: ItemReady, Priority: 10})
	q.Enqueue(&WorkItem{ID: "high", Status: ItemReady, Priority: 1})
	q.Enqueue(&WorkItem{ID: "pending", Status: ItemPending})

	next := q.NextReady()
	if next == nil || next.ID != "high" {
		t.Errorf("expected high priority item, got %v", next)
	}
}

func TestTaskQueue_Pending(t *testing.T) {
	q := NewTaskQueue()
	q.Enqueue(&WorkItem{ID: "a", Status: ItemPending})
	q.Enqueue(&WorkItem{ID: "b", Status: ItemReady})
	q.Enqueue(&WorkItem{ID: "c", Status: ItemPending})

	pending := q.Pending()
	if len(pending) != 2 {
		t.Errorf("got %d pending, want 2", len(pending))
	}
}

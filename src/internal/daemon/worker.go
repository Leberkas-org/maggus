package daemon

import "sync"

type WorkerPool struct {
	maxWorkers int
	active     map[string]*TaskWorker
	mergeMu    map[string]*sync.Mutex
	mu         sync.Mutex
}

func NewWorkerPool(maxWorkers int) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
		active:     make(map[string]*TaskWorker),
		mergeMu:    make(map[string]*sync.Mutex),
	}
}

func (p *WorkerPool) Submit(w *TaskWorker) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.active) >= p.maxWorkers {
		return errPoolFull{}
	}

	p.active[w.task.ID] = w
	go func() {
		w.Run()
		p.mu.Lock()
		delete(p.active, w.task.ID)
		p.mu.Unlock()
	}()
	return nil
}

func (p *WorkerPool) MergeMutex(featureBranch string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.mergeMu[featureBranch]; !ok {
		p.mergeMu[featureBranch] = &sync.Mutex{}
	}
	return p.mergeMu[featureBranch]
}

func (p *WorkerPool) Cancel(taskID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.active[taskID]; ok {
		w.Cancel()
		return nil
	}
	return errWorkerNotFound(taskID)
}

func (p *WorkerPool) CancelAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, w := range p.active {
		w.Cancel()
	}
}

func (p *WorkerPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active)
}

func (p *WorkerPool) Wait() {
	for {
		p.mu.Lock()
		n := len(p.active)
		p.mu.Unlock()
		if n == 0 {
			return
		}
	}
}

type errPoolFull struct{}

func (e errPoolFull) Error() string { return "worker pool is full" }

type errWorkerNotFound string

func (e errWorkerNotFound) Error() string { return "worker not found: " + string(e) }

package worker

import "sync"

// Pool is a simple goroutine worker pool that limits concurrent task execution.
type Pool struct {
	sem  chan struct{}
	wg   sync.WaitGroup
}

// NewPool creates a worker pool with the given max concurrency.
func NewPool(size int) *Pool {
	if size < 1 {
		size = 1
	}
	return &Pool{
		sem: make(chan struct{}, size),
	}
}

// Submit enqueues a task to be executed by the pool.
// It blocks if all workers are busy.
func (p *Pool) Submit(task func()) {
	p.wg.Add(1)
	go func() {
		p.sem <- struct{}{}
		defer func() {
			<-p.sem
			p.wg.Done()
		}()
		task()
	}()
}

// Shutdown waits for all submitted tasks to complete.
func (p *Pool) Shutdown() {
	p.wg.Wait()
}

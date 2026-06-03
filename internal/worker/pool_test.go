package worker

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_LimitsConcurrency(t *testing.T) {
	pool := NewPool(1)

	var running atomic.Int32
	var maxConcurrent atomic.Int32

	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			cur := running.Add(1)
			// Track max concurrency
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		})
	}

	pool.Shutdown()

	if maxConcurrent.Load() > 1 {
		t.Errorf("expected max concurrency of 1, got %d", maxConcurrent.Load())
	}
}

func TestPool_ExecutesAllTasks(t *testing.T) {
	pool := NewPool(3)

	var count atomic.Int32
	for i := 0; i < 10; i++ {
		pool.Submit(func() {
			count.Add(1)
		})
	}

	pool.Shutdown()

	if count.Load() != 10 {
		t.Errorf("expected 10 tasks executed, got %d", count.Load())
	}
}

func TestPool_ShutdownWaitsForCompletion(t *testing.T) {
	pool := NewPool(1)
	done := make(chan struct{})

	pool.Submit(func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	})

	pool.Shutdown()

	select {
	case <-done:
		// OK
	default:
		t.Error("Shutdown returned before task completed")
	}
}

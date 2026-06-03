package worker

import (
	"log"
	"time"
)

// RetryTicker periodically calls a retry function at the given interval.
type RetryTicker struct {
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewRetryTicker starts a background ticker that calls retryFn every interval.
func NewRetryTicker(interval time.Duration, retryFn func()) *RetryTicker {
	rt := &RetryTicker{
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	go func() {
		defer close(rt.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("AI retry ticker started (interval: %s)", interval)
		for {
			select {
			case <-ticker.C:
				retryFn()
			case <-rt.stop:
				return
			}
		}
	}()

	return rt
}

// Stop halts the retry ticker and waits for it to finish.
func (rt *RetryTicker) Stop() {
	close(rt.stop)
	<-rt.done
}

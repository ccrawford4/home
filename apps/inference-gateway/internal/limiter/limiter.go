// Package limiter provides a counting semaphore with a bounded FIFO wait queue.
package limiter

import (
	"context"
	"errors"
	"sync"
)

// ErrBusy is returned when all slots are taken and the wait queue is full.
var ErrBusy = errors.New("backend busy")

// Limiter allows up to capacity concurrent holders; up to queueDepth more
// callers wait in FIFO order. Capacity 0 means unlimited.
type Limiter struct {
	mu         sync.Mutex
	capacity   int
	queueDepth int
	active     int
	waiters    []chan struct{}
}

// New returns a Limiter.
func New(capacity, queueDepth int) *Limiter {
	return &Limiter{capacity: capacity, queueDepth: queueDepth}
}

// Acquire takes a slot, waiting in line if necessary. It returns ErrBusy when
// the queue is full, or ctx.Err() if ctx ends while waiting. On success the
// caller must call Release exactly once.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l.capacity <= 0 {
		return nil
	}
	l.mu.Lock()
	if l.active < l.capacity && len(l.waiters) == 0 {
		l.active++
		l.mu.Unlock()
		return nil
	}
	if len(l.waiters) >= l.queueDepth {
		l.mu.Unlock()
		return ErrBusy
	}
	ch := make(chan struct{})
	l.waiters = append(l.waiters, ch)
	l.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		for i, w := range l.waiters {
			if w == ch {
				l.waiters = append(l.waiters[:i], l.waiters[i+1:]...)
				l.mu.Unlock()
				return ctx.Err()
			}
		}
		l.mu.Unlock()
		// The slot was handed to us just as ctx ended; give it back.
		l.Release()
		return ctx.Err()
	}
}

// Release frees a slot, handing it directly to the oldest waiter if any.
func (l *Limiter) Release() {
	if l.capacity <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.waiters) > 0 {
		ch := l.waiters[0]
		l.waiters = l.waiters[1:]
		close(ch)
		return
	}
	l.active--
}

// Stats returns the number of active holders and queued waiters.
func (l *Limiter) Stats() (active, waiting int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active, len(l.waiters)
}

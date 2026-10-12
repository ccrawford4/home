package limiter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitStats(t *testing.T, l *Limiter, active, waiting int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a, w := l.Stats(); a == active && w == waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	a, w := l.Stats()
	t.Fatalf("active=%d waiting=%d, want %d/%d", a, w, active, waiting)
}

func TestLimiter(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"unlimited never blocks", func(t *testing.T) {
			l := New(0, 0)
			for range 100 {
				if err := l.Acquire(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"overflow returns ErrBusy", func(t *testing.T) {
			l := New(1, 1)
			if err := l.Acquire(t.Context()); err != nil {
				t.Fatal(err)
			}
			go l.Acquire(t.Context())
			waitStats(t, l, 1, 1)
			if err := l.Acquire(t.Context()); !errors.Is(err, ErrBusy) {
				t.Fatalf("got %v, want ErrBusy", err)
			}
			l.Release() // hands slot to the waiter
			waitStats(t, l, 1, 0)
			l.Release()
			waitStats(t, l, 0, 0)
		}},
		{"waiters served FIFO", func(t *testing.T) {
			l := New(1, 3)
			l.Acquire(t.Context())
			order := make(chan int, 3)
			for i := range 3 {
				go func() {
					l.Acquire(t.Context())
					order <- i
				}()
				waitStats(t, l, 1, i+1)
			}
			for want := range 3 {
				l.Release()
				if got := <-order; got != want {
					t.Fatalf("served %d, want %d", got, want)
				}
			}
			l.Release()
			waitStats(t, l, 0, 0)
		}},
		{"cancel while waiting leaves queue", func(t *testing.T) {
			l := New(1, 2)
			l.Acquire(t.Context())
			ctx, cancel := context.WithCancel(t.Context())
			errc := make(chan error, 1)
			go func() { errc <- l.Acquire(ctx) }()
			waitStats(t, l, 1, 1)
			cancel()
			if err := <-errc; !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v", err)
			}
			waitStats(t, l, 1, 0)
			l.Release()
			waitStats(t, l, 0, 0)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

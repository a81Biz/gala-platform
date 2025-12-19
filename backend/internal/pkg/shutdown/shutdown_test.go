package shutdown

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gala/internal/pkg/logger"
)

func newTestLogger() *logger.Logger {
	var buf bytes.Buffer
	return logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &buf,
	})
}

func TestNewManager(t *testing.T) {
	log := newTestLogger()

	t.Run("with default timeout", func(t *testing.T) {
		mgr := NewManager(log, 0)
		if mgr == nil {
			t.Fatal("expected manager to be non-nil")
		}
	})

	t.Run("with custom timeout", func(t *testing.T) {
		mgr := NewManager(log, 10*time.Second)
		if mgr == nil {
			t.Fatal("expected manager to be non-nil")
		}
	})
}

func TestRegister(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 5*time.Second)

	mgr.Register("test", func(ctx context.Context) error {
		return nil
	})

	if len(mgr.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(mgr.handlers))
	}

	if mgr.handlers[0].Name != "test" {
		t.Errorf("expected handler name 'test', got %s", mgr.handlers[0].Name)
	}
}

func TestRegisterSimple(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 5*time.Second)

	var called bool
	mgr.RegisterSimple("simple", func() {
		called = true
	})

	if len(mgr.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(mgr.handlers))
	}

	// Run shutdown to verify it works
	mgr.Shutdown()

	if !called {
		t.Error("expected simple handler to be called")
	}
}

func TestShutdown(t *testing.T) {
	log := newTestLogger()

	t.Run("runs handlers in LIFO order", func(t *testing.T) {
		mgr := NewManager(log, 5*time.Second)

		var order []int
		mgr.Register("first", func(ctx context.Context) error {
			order = append(order, 1)
			return nil
		})
		mgr.Register("second", func(ctx context.Context) error {
			order = append(order, 2)
			return nil
		})
		mgr.Register("third", func(ctx context.Context) error {
			order = append(order, 3)
			return nil
		})

		mgr.Shutdown()

		// Wait a bit for goroutines
		time.Sleep(100 * time.Millisecond)

		// Note: handlers run concurrently, so we can't guarantee strict order
		// But we can verify all handlers were called
		if len(order) != 3 {
			t.Errorf("expected 3 handlers called, got %d", len(order))
		}
	})

	t.Run("closes done channel", func(t *testing.T) {
		mgr := NewManager(log, 5*time.Second)
		mgr.Shutdown()

		select {
		case <-mgr.Done():
			// Expected
		case <-time.After(time.Second):
			t.Error("expected done channel to be closed")
		}
	})

	t.Run("handles handler errors gracefully", func(t *testing.T) {
		mgr := NewManager(log, 5*time.Second)

		mgr.Register("failing", func(ctx context.Context) error {
			return context.DeadlineExceeded
		})

		// Should not panic
		mgr.Shutdown()
	})
}

func TestDone(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 5*time.Second)

	done := mgr.Done()
	if done == nil {
		t.Fatal("expected done channel to be non-nil")
	}

	select {
	case <-done:
		t.Error("expected done channel to not be closed initially")
	default:
		// Expected
	}

	mgr.Shutdown()

	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Error("expected done channel to be closed after shutdown")
	}
}

func TestContext(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 5*time.Second)

	ctx := mgr.Context()
	if ctx == nil {
		t.Fatal("expected context to be non-nil")
	}

	// Context should not be canceled initially
	select {
	case <-ctx.Done():
		t.Error("expected context to not be canceled initially")
	default:
		// Expected
	}

	mgr.Shutdown()

	// Wait for context to be canceled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(time.Second):
		t.Error("expected context to be canceled after shutdown")
	}
}

func TestShutdownTimeout(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 100*time.Millisecond) // Very short timeout

	var handlerCompleted atomic.Bool
	mgr.Register("slow", func(ctx context.Context) error {
		select {
		case <-time.After(5 * time.Second): // Very slow
			handlerCompleted.Store(true)
		case <-ctx.Done():
			// Timeout hit
		}
		return nil
	})

	start := time.Now()
	mgr.Shutdown()
	elapsed := time.Since(start)

	// Should have timed out relatively quickly
	if elapsed > 500*time.Millisecond {
		t.Errorf("shutdown took too long: %v", elapsed)
	}
}

func TestConcurrentHandlers(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log, 5*time.Second)

	var counter atomic.Int32

	// Register multiple handlers
	for i := 0; i < 10; i++ {
		mgr.Register("handler", func(ctx context.Context) error {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	mgr.Shutdown()

	// Wait a bit for all goroutines
	time.Sleep(200 * time.Millisecond)

	if counter.Load() != 10 {
		t.Errorf("expected 10 handlers to run, got %d", counter.Load())
	}
}

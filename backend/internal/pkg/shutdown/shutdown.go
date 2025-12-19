// Package shutdown provides graceful shutdown utilities for GALA platform.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gala/internal/pkg/logger"
)

// Manager handles graceful shutdown of services.
type Manager struct {
	log      *logger.Logger
	timeout  time.Duration
	handlers []Handler
	mu       sync.Mutex
	done     chan struct{}
}

// Handler is a function that performs cleanup during shutdown.
type Handler struct {
	Name    string
	Cleanup func(ctx context.Context) error
}

// NewManager creates a new shutdown manager.
func NewManager(log *logger.Logger, timeout time.Duration) *Manager {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Manager{
		log:      log,
		timeout:  timeout,
		handlers: make([]Handler, 0),
		done:     make(chan struct{}),
	}
}

// Register adds a cleanup handler.
func (m *Manager) Register(name string, cleanup func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, Handler{Name: name, Cleanup: cleanup})
	m.log.Debug("registered shutdown handler", "name", name)
}

// RegisterSimple adds a simple cleanup handler without context.
func (m *Manager) RegisterSimple(name string, cleanup func()) {
	m.Register(name, func(ctx context.Context) error {
		cleanup()
		return nil
	})
}

// Wait blocks until shutdown signal is received, then runs cleanup.
func (m *Manager) Wait() {
	// Listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for signal
	sig := <-sigChan
	m.log.Info("shutdown signal received", "signal", sig.String())

	// Run cleanup
	m.Shutdown()
}

// Shutdown runs all cleanup handlers.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	handlers := make([]Handler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.Unlock()

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.log.Info("starting graceful shutdown", "handlers", len(handlers), "timeout", m.timeout.String())

	// Run handlers in reverse order (LIFO)
	var wg sync.WaitGroup
	errors := make(chan error, len(handlers))

	for i := len(handlers) - 1; i >= 0; i-- {
		h := handlers[i]
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			m.log.Debug("running shutdown handler", "name", h.Name)
			start := time.Now()
			
			if err := h.Cleanup(ctx); err != nil {
				m.log.Error("shutdown handler failed", 
					"name", h.Name, 
					"error", err.Error(),
					"duration_ms", time.Since(start).Milliseconds(),
				)
				errors <- err
			} else {
				m.log.Debug("shutdown handler completed", 
					"name", h.Name,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
		}(h)
	}

	// Wait for all handlers or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.log.Info("graceful shutdown completed")
	case <-ctx.Done():
		m.log.Warn("shutdown timeout exceeded, forcing exit")
	}

	close(m.done)
}

// Done returns a channel that is closed when shutdown is complete.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// Context returns a context that is canceled on shutdown.
func (m *Manager) Context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-m.done
		cancel()
	}()
	return ctx
}

// WaitWithContext waits for shutdown signal with a custom context.
func (m *Manager) WaitWithContext(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case sig := <-sigChan:
		m.log.Info("shutdown signal received", "signal", sig.String())
	case <-ctx.Done():
		m.log.Info("context canceled, initiating shutdown")
	}

	m.Shutdown()
}

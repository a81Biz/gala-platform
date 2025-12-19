package handlers

import (
	"context"
	"net/http"
	"time"

	"gala/internal/httpkit"
)

// Health performs a health check of the service.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.FromContext(ctx)

	// Basic health response
	health := map[string]any{
		"status":  "ok",
		"service": "gala-api",
		"version": "0.1.0",
	}

	// Check if deep health check is requested
	if r.URL.Query().Get("deep") == "true" {
		checks := h.deepHealthCheck(ctx)
		health["checks"] = checks

		// If any check failed, change status
		for _, check := range checks {
			if checkMap, ok := check.(map[string]any); ok {
				if checkMap["status"] != "ok" {
					health["status"] = "degraded"
					log.Warn("health check degraded", "checks", checks)
					break
				}
			}
		}
	}

	httpkit.WriteJSON(w, 200, health)
}

// deepHealthCheck performs detailed health checks on dependencies.
func (h *Handler) deepHealthCheck(ctx context.Context) map[string]any {
	checks := make(map[string]any)

	// PostgreSQL check
	checks["postgres"] = h.checkPostgres(ctx)

	// Redis check
	checks["redis"] = h.checkRedis(ctx)

	// Storage check
	checks["storage"] = h.checkStorage(ctx)

	return checks
}

func (h *Handler) checkPostgres(ctx context.Context) map[string]any {
	start := time.Now()
	result := map[string]any{
		"status": "ok",
	}

	// Create a context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Ping the database
	if err := h.pool.Ping(checkCtx); err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
	} else {
		// Get pool stats
		stats := h.pool.Stat()
		result["total_conns"] = stats.TotalConns()
		result["idle_conns"] = stats.IdleConns()
		result["acquired_conns"] = stats.AcquiredConns()
	}

	result["latency_ms"] = time.Since(start).Milliseconds()
	return result
}

func (h *Handler) checkRedis(ctx context.Context) map[string]any {
	start := time.Now()
	result := map[string]any{
		"status": "ok",
	}

	// Create a context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Ping Redis
	if err := h.rdb.Ping(checkCtx).Err(); err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
	}

	result["latency_ms"] = time.Since(start).Milliseconds()
	return result
}

func (h *Handler) checkStorage(_ context.Context) map[string]any {
	result := map[string]any{
		"status":   "ok",
		"provider": h.sp.Provider(),
	}

	// For now, just report the provider type
	// In the future, we could add actual connectivity checks
	return result
}

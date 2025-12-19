package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/httpapi"
	"gala/internal/pkg/logger"
	"gala/internal/pkg/shutdown"
	"gala/internal/storage"
)

func main() {
	// Initialize logger
	log := logger.New(logger.Config{
		Level:       getEnv("LOG_LEVEL", "info"),
		Format:      getEnv("LOG_FORMAT", "json"),
		ServiceName: "gala-api",
		AddSource:   getEnv("LOG_SOURCE", "false") == "true",
	})

	log.Info("starting GALA API",
		"version", "0.1.0",
	)

	// Load configuration
	httpPort := getEnv("HTTP_PORT", "8080")
	dbURL := mustEnv(log, "DATABASE_URL")
	redisAddr := mustEnv(log, "REDIS_ADDR")

	ctx := context.Background()

	// Initialize shutdown manager
	shutdownMgr := shutdown.NewManager(log, 30*time.Second)

	// Connect to PostgreSQL
	log.Info("connecting to PostgreSQL")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.LogFatal("failed to connect to PostgreSQL", err)
	}
	shutdownMgr.Register("postgres", func(ctx context.Context) error {
		pool.Close()
		return nil
	})

	// Verify PostgreSQL connection
	if err := pool.Ping(ctx); err != nil {
		log.LogFatal("failed to ping PostgreSQL", err)
	}
	log.Info("PostgreSQL connected")

	// Connect to Redis
	log.Info("connecting to Redis")
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	shutdownMgr.Register("redis", func(ctx context.Context) error {
		return rdb.Close()
	})

	// Verify Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.LogFatal("failed to ping Redis", err)
	}
	log.Info("Redis connected")

	// Initialize storage provider
	log.Info("initializing storage provider")
	sp, err := storage.NewProvider()
	if err != nil {
		log.LogFatal("failed to initialize storage provider", err)
	}
	log.Info("storage provider initialized", "provider", sp.Provider())

	// Create HTTP router
	deps := httpapi.Deps{
		Pool: pool,
		RDB:  rdb,
		SP:   sp,
		Log:  log,
	}
	router := httpapi.NewRouter(deps)

	// Create HTTP server
	server := &http.Server{
		Addr:         "0.0.0.0:" + httpPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Register server shutdown
	shutdownMgr.Register("http-server", func(ctx context.Context) error {
		log.Info("shutting down HTTP server")
		return server.Shutdown(ctx)
	})

	// Start server in goroutine
	go func() {
		log.Info("HTTP server listening",
			"addr", server.Addr,
			"port", httpPort,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.LogFatal("HTTP server failed", err)
		}
	}()

	// Wait for shutdown signal
	shutdownMgr.Wait()
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	return v
}

// mustEnv gets a required environment variable or exits.
func mustEnv(log *logger.Logger, key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Error("missing required environment variable", "key", key)
		os.Exit(1)
	}
	return v
}

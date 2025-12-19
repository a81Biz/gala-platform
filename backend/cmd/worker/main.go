package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/pkg/logger"
	"gala/internal/pkg/shutdown"
	"gala/internal/storage"
	"gala/internal/worker"
)

func main() {
	// Initialize logger
	log := logger.New(logger.Config{
		Level:       getEnv("LOG_LEVEL", "info"),
		Format:      getEnv("LOG_FORMAT", "json"),
		ServiceName: "gala-worker",
		AddSource:   getEnv("LOG_SOURCE", "false") == "true",
	})

	log.Info("starting GALA Worker",
		"version", "0.1.0",
	)

	// Load configuration
	dbURL := mustEnv(log, "DATABASE_URL")
	redisAddr := mustEnv(log, "REDIS_ADDR")
	rendererBaseURL := mustEnv(log, "RENDERER_HTTP_BASEURL")
	storageRoot := getEnv("STORAGE_LOCAL_ROOT", "/data")
	queueName := getEnv("JOB_QUEUE_NAME", "gala:jobs")
	cleanupLocal := boolEnv("WORKER_CLEANUP_LOCAL", false)

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

	// Create worker dependencies
	deps := worker.Deps{
		Pool:            pool,
		RDB:             rdb,
		RendererBaseURL: rendererBaseURL,
		StorageRoot:     storageRoot,
		QueueName:       queueName,
		CleanupLocal:    cleanupLocal,
		SP:              sp,
		Log:             log,
	}

	log.Info("worker configuration",
		"queue", queueName,
		"renderer_url", rendererBaseURL,
		"storage_root", storageRoot,
		"cleanup_local", cleanupLocal,
	)

	// Create cancellable context for the worker
	workerCtx, cancelWorker := context.WithCancel(ctx)

	// Register worker shutdown
	shutdownMgr.Register("worker", func(ctx context.Context) error {
		log.Info("stopping worker")
		cancelWorker()
		// Give worker time to finish current job
		time.Sleep(1 * time.Second)
		return nil
	})

	// Start worker in goroutine
	go func() {
		log.Info("worker started, waiting for jobs")
		if err := worker.Run(workerCtx, deps); err != nil {
			if err != context.Canceled {
				log.Error("worker error", "error", err.Error())
			}
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

// boolEnv gets a boolean environment variable.
func boolEnv(key string, defaultValue bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return defaultValue
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/storage"
	"gala/internal/worker"
	"gala/internal/worker/util"
)

func main() {
	ctx := context.Background()

	dbURL := util.MustEnv("DATABASE_URL")
	redisAddr := util.MustEnv("REDIS_ADDR")
	rendererBaseURL := util.MustEnv("RENDERER_HTTP_BASEURL")
	storageRoot := util.Env("STORAGE_LOCAL_ROOT", "/data")
	queueName := util.Env("JOB_QUEUE_NAME", "gala:jobs")
	cleanupLocal := util.BoolEnv("WORKER_CLEANUP_LOCAL", false)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	sp, err := storage.NewProvider()
	if err != nil {
		panic(err)
	}

	deps := worker.Deps{
		Pool:            pool,
		RDB:             rdb,
		RendererBaseURL: rendererBaseURL,
		StorageRoot:     storageRoot,
		QueueName:       queueName,
		CleanupLocal:    cleanupLocal,
		SP:              sp,
	}

	fmt.Println("GALA Worker started")
	if err := worker.Run(ctx, deps); err != nil {
		panic(err)
	}
}

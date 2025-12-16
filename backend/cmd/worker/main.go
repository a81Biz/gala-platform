package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

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

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	deps := worker.Deps{
		Pool:            pool,
		RDB:             rdb,
		RendererBaseURL: rendererBaseURL,
		StorageRoot:     storageRoot,
		QueueName:       queueName,
	}

	fmt.Println("GALA Worker started")
	if err := worker.Run(ctx, deps); err != nil {
		panic(err)
	}
}

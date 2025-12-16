package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/httpapi"
	"gala/internal/httpapi/util"
	"gala/internal/storage"
)

func main() {
	httpPort := util.Env("HTTP_PORT", "8080")
	dbURL := util.MustEnv("DATABASE_URL")
	redisAddr := util.MustEnv("REDIS_ADDR")

	ctx := context.Background()

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

	deps := httpapi.Deps{
		Pool: pool,
		RDB:  rdb,
		SP:   sp,
	}

	r := httpapi.NewRouter(deps)

	fmt.Printf("GALA API listening on :%s\n", httpPort)
	if err := http.ListenAndServe("0.0.0.0:"+httpPort, r); err != nil {
		panic(err)
	}
}

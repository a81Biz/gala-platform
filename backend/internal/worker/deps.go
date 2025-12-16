package worker

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Pool            *pgxpool.Pool
	RDB             *redis.Client
	RendererBaseURL string
	StorageRoot     string
	QueueName       string
}

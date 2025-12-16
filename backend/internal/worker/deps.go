package worker

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/ports"
)

type Deps struct {
	Pool            *pgxpool.Pool
	RDB             *redis.Client
	RendererBaseURL string
	StorageRoot     string
	QueueName       string

	// Feature flag: if true, the worker will delete local render staging under StorageRoot
	// after (1) upload OK and (2) DB insert OK. See README Punto 3.
	CleanupLocal bool

	SP ports.StorageProvider
}

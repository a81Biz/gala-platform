package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/ports"
)

type Deps struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
	SP   ports.StorageProvider
}

type Handler struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	sp   ports.StorageProvider
}

func New(d Deps) *Handler {
	return &Handler{
		pool: d.Pool,
		rdb:  d.RDB,
		sp:   d.SP,
	}
}

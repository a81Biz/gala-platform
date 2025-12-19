package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/pkg/logger"
	"gala/internal/ports"
)

type Deps struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
	SP   ports.StorageProvider
	Log  *logger.Logger
}

type Handler struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	sp   ports.StorageProvider
	log  *logger.Logger
}

func New(d Deps) *Handler {
	// Create a component-specific logger
	handlerLog := d.Log
	if handlerLog != nil {
		handlerLog = handlerLog.WithComponent("handlers")
	}

	return &Handler{
		pool: d.Pool,
		rdb:  d.RDB,
		sp:   d.SP,
		log:  handlerLog,
	}
}

// getLogger returns a logger enriched with request context.
func (h *Handler) getLogger(_ interface{ Context() interface{} }) *logger.Logger {
	if h.log == nil {
		return logger.NewDefault()
	}
	return h.log
}

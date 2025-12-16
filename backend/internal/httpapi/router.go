package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/httpapi/handlers"
	"gala/internal/ports"
)

type Deps struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
	SP   ports.StorageProvider
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	h := handlers.New(handlers.Deps{
		Pool: d.Pool,
		RDB:  d.RDB,
		SP:   d.SP,
	})

	// HEALTH
	r.Get("/health", h.Health)

	// ASSETS
	r.Post("/assets", h.PostAsset)
	r.Get("/assets/{assetId}", h.GetAsset)
	r.Get("/assets/{assetId}/url", h.GetAssetURL)
	r.Get("/assets/{assetId}/content", h.StreamAsset)
	r.Delete("/assets/{assetId}", h.DeleteAsset)

	// JOBS
	r.Post("/jobs", h.PostJob)
	r.Get("/jobs", h.ListJobs)
	r.Get("/jobs/{jobId}", h.GetJob)

	return r
}

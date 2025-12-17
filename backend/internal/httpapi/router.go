package httpapi

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"gala/internal/httpapi/handlers"
	"gala/internal/httpkit"
	"gala/internal/ports"
)

type Deps struct {
	Pool *pgxpool.Pool
	RDB  *redis.Client
	SP   ports.StorageProvider
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// ---- CORS (Swagger UI + Frontend futuro) ----
	allowedOrigins := envCSV("CORS_ALLOWED_ORIGINS", []string{
		"http://localhost:8081",
		"http://localhost:5173",
	})
	r.Use(httpkit.CORS(httpkit.CORSOptions{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAgeSeconds:    600,
	}))

	h := handlers.New(handlers.Deps{
		Pool: d.Pool,
		RDB:  d.RDB,
		SP:   d.SP,
	})

	// ---- HEALTH ----
	r.Get("/health", h.Health)

	// ---- ASSETS ----
	r.Post("/assets", h.PostAsset)
	r.Get("/assets/{assetId}", h.GetAsset)
	r.Get("/assets/{assetId}/url", h.GetAssetURL)
	r.Get("/assets/{assetId}/content", h.StreamAsset)
	r.Delete("/assets/{assetId}", h.DeleteAsset)

	// ---- TEMPLATES ----
	r.Post("/templates", h.PostTemplate)
	r.Get("/templates", h.ListTemplates)
	r.Get("/templates/{templateId}", h.GetTemplate)
	r.Patch("/templates/{templateId}", h.PatchTemplate)
	r.Delete("/templates/{templateId}", h.DeleteTemplate)

	// ---- JOBS ----
	r.Post("/jobs", h.PostJob)
	r.Get("/jobs", h.ListJobs)
	r.Get("/jobs/{jobId}", h.GetJob)

	return r
}

func envCSV(key string, def []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

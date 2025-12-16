package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/httpapi/util"
	"gala/internal/httpkit"
)

type CreateJobRequest struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

func (h *Handler) PostJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateJobRequest
	if err := httpkit.DecodeJSON(r, &req); err != nil {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "invalid json body", nil)
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}

	if _, ok := req.Params["text"]; !ok {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "params.text is required", map[string]any{"field": "params.text"})
		return
	}

	jobID := util.NewID("job")
	paramsBytes, _ := json.Marshal(req.Params)

	createdAt := time.Now().UTC()
	_, err := h.pool.Exec(ctx,
		`INSERT INTO jobs (id, name, status, params_json, created_at)
		 VALUES ($1,$2,'QUEUED',$3,$4)`,
		jobID, nullIfEmpty(strings.TrimSpace(req.Name)), string(paramsBytes), createdAt,
	)
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db insert failed", nil)
		return
	}

	if err := h.rdb.LPush(ctx, "gala:jobs", jobID).Err(); err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "queue push failed", nil)
		return
	}

	httpkit.WriteJSON(w, 201, map[string]any{
		"job": map[string]any{
			"id":         jobID,
			"name":       req.Name,
			"status":     "QUEUED",
			"params":     req.Params,
			"created_at": createdAt,
		},
	})
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	var (
		rows pgxRows
		err  error
	)

	if status != "" {
		rows, err = h.pool.Query(ctx,
			`SELECT id, COALESCE(name,''), status, created_at
			 FROM jobs WHERE status=$1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			status, limit,
		)
	} else {
		rows, err = h.pool.Query(ctx,
			`SELECT id, COALESCE(name,''), status, created_at
			 FROM jobs
			 ORDER BY created_at DESC
			 LIMIT $1`,
			limit,
		)
	}
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db query failed", nil)
		return
	}
	defer rows.Close()

	type item struct {
		ID        string    `json:"id"`
		Name      string    `json:"name,omitempty"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}

	out := make([]item, 0, limit)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Name, &it.Status, &it.CreatedAt); err != nil {
			httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "row scan failed", nil)
			return
		}
		out = append(out, it)
	}

	httpkit.WriteJSON(w, 200, map[string]any{"jobs": out})
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := chi.URLParam(r, "jobId")

	var (
		id, name, status, paramsJSON string
		createdAt                    time.Time
		startedAt, finishedAt        *time.Time
	)

	err := h.pool.QueryRow(ctx,
		`SELECT id, COALESCE(name,''), status, params_json, created_at, started_at, finished_at
		 FROM jobs WHERE id=$1`,
		jobID,
	).Scan(&id, &name, &status, &paramsJSON, &createdAt, &startedAt, &finishedAt)
	if err != nil {
		httpkit.WriteErr(w, 404, "JOB_NOT_FOUND", "job not found", map[string]any{"job_id": jobID})
		return
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(paramsJSON), &params)

	type outItem struct {
		Variant           int    `json:"variant"`
		VideoAssetID      string `json:"video_asset_id"`
		ThumbnailAssetID  string `json:"thumbnail_asset_id,omitempty"`
		CaptionsAssetID   string `json:"captions_asset_id,omitempty"`
		VideoObjectKey    string `json:"video_object_key,omitempty"`
		ThumbObjectKey    string `json:"thumb_object_key,omitempty"`
		CaptionsObjectKey string `json:"captions_object_key,omitempty"`
	}

	outs := []outItem{}

	rows, err := h.pool.Query(ctx,
		`SELECT variant, video_asset_id, COALESCE(thumbnail_asset_id,''), COALESCE(captions_asset_id,'')
		 FROM job_outputs WHERE job_id=$1 ORDER BY variant ASC`,
		jobID,
	)
	if err != nil {
		if !httpkit.IsUndefinedTable(err) {
			httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db outputs query failed", nil)
			return
		}
	} else {
		defer rows.Close()
		for rows.Next() {
			var it outItem
			var thumbID, capID string
			if err := rows.Scan(&it.Variant, &it.VideoAssetID, &thumbID, &capID); err != nil {
				httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "outputs scan failed", nil)
				return
			}
			if thumbID != "" {
				it.ThumbnailAssetID = thumbID
			}
			if capID != "" {
				it.CaptionsAssetID = capID
			}

			it.VideoObjectKey = lookupObjectKey(ctx, h.pool, it.VideoAssetID)
			if it.ThumbnailAssetID != "" {
				it.ThumbObjectKey = lookupObjectKey(ctx, h.pool, it.ThumbnailAssetID)
			}
			if it.CaptionsAssetID != "" {
				it.CaptionsObjectKey = lookupObjectKey(ctx, h.pool, it.CaptionsAssetID)
			}

			outs = append(outs, it)
		}
	}

	httpkit.WriteJSON(w, 200, map[string]any{
		"job": map[string]any{
			"id":          id,
			"name":        name,
			"status":      status,
			"params":      params,
			"created_at":  createdAt,
			"started_at":  startedAt,
			"finished_at": finishedAt,
			"outputs":     outs,
		},
	})
}

func lookupObjectKey(ctx context.Context, pool *pgxpool.Pool, assetID string) string {
	if assetID == "" {
		return ""
	}
	var objectKey string
	_ = pool.QueryRow(ctx, `SELECT object_key FROM assets WHERE id=$1`, assetID).Scan(&objectKey)
	return objectKey
}

type pgxRows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
}


package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"gala/internal/httpapi/util"
	"gala/internal/httpkit"
	"gala/internal/ports"
)

func (h *Handler) PostAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(512 << 20); err != nil {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "invalid multipart form", nil)
		return
	}

	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "kind is required", map[string]any{"field": "kind"})
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))

	file, header, err := r.FormFile("file")
	if err != nil {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "file is required", map[string]any{"field": "file"})
		return
	}
	defer file.Close()

	assetID := util.NewID("ast")
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = guessExt(header.Header.Get("Content-Type"))
		if ext == "" {
			ext = ".bin"
		}
	}

	objectKey := fmt.Sprintf("assets/%s/original%s", assetID, ext)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(ext)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	out, err := h.sp.PutObject(ctx, ports.PutObjectInput{
		ObjectKey:   objectKey,
		ContentType: contentType,
		Reader:      file,
		Size:        header.Size,
	})
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "storage put failed", nil)
		return
	}

	createdAt := time.Now().UTC()
	provider := h.sp.Provider()
	_, err = h.pool.Exec(ctx,
		`INSERT INTO assets (id, kind, provider, object_key, mime, size_bytes, label, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		assetID, kind, provider, out.ObjectKey, contentType, out.Size, nullIfEmpty(label), createdAt,
	)
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db insert asset failed", nil)
		return
	}

	httpkit.WriteJSON(w, 201, map[string]any{
		"asset": map[string]any{
			"id":         assetID,
			"kind":       kind,
			"provider":   provider,
			"object_key": out.ObjectKey,
			"mime":       contentType,
			"size_bytes": out.Size,
			"label":      label,
			"created_at": createdAt,
		},
	})
}

func (h *Handler) GetAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assetID := chi.URLParam(r, "assetId")

	var (
		id, kind, provider, objectKey, mimeType string
		sizeBytes                               int64
		label                                   sql.NullString
		createdAt                               time.Time
	)

	err := h.pool.QueryRow(ctx,
		`SELECT id, kind, provider, object_key, mime, size_bytes, label, created_at
		 FROM assets WHERE id=$1`, assetID,
	).Scan(&id, &kind, &provider, &objectKey, &mimeType, &sizeBytes, &label, &createdAt)
	if err != nil {
		httpkit.WriteErr(w, 404, "ASSET_NOT_FOUND", "asset not found", map[string]any{"asset_id": assetID})
		return
	}

	httpkit.WriteJSON(w, 200, map[string]any{
		"asset": map[string]any{
			"id":         id,
			"kind":       kind,
			"provider":   provider,
			"object_key": objectKey,
			"mime":       mimeType,
			"size_bytes": sizeBytes,
			"label":      label.String,
			"created_at": createdAt,
		},
	})
}

func (h *Handler) GetAssetURL(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "assetId")
	expiresAt := time.Now().UTC().Add(30 * time.Minute)

	httpkit.WriteJSON(w, 200, map[string]any{
		"asset_id":   assetID,
		"url":        fmt.Sprintf("http://localhost:%s/assets/%s/content", util.Env("HTTP_PORT", "8080"), assetID),
		"expires_at": expiresAt,
	})
}

func (h *Handler) StreamAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assetID := chi.URLParam(r, "assetId")

	var objectKey, mimeType string
	var sizeBytes int64

	err := h.pool.QueryRow(ctx,
		`SELECT object_key, mime, size_bytes FROM assets WHERE id=$1`, assetID,
	).Scan(&objectKey, &mimeType, &sizeBytes)
	if err != nil {
		httpkit.WriteErr(w, 404, "ASSET_NOT_FOUND", "asset not found", map[string]any{"asset_id": assetID})
		return
	}

	rc, ct, _, err := h.sp.GetObject(ctx, objectKey)
	if err != nil {
		httpkit.WriteErr(w, 404, "ASSET_FILE_MISSING", "asset file missing", map[string]any{"object_key": objectKey})
		return
	}
	defer rc.Close()

	if ct == "" {
		ct = mimeType
	}
	w.Header().Set("Content-Type", ct)
	if sizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(sizeBytes, 10))
	}
	_, _ = io.Copy(w, rc)
}

func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assetID := chi.URLParam(r, "assetId")

	var objectKey string
	err := h.pool.QueryRow(ctx, `SELECT object_key FROM assets WHERE id=$1`, assetID).Scan(&objectKey)
	if err != nil {
		httpkit.WriteErr(w, 404, "ASSET_NOT_FOUND", "asset not found", map[string]any{"asset_id": assetID})
		return
	}

	var cnt int
	if err := h.pool.QueryRow(ctx,
		`SELECT COUNT(1)
		 FROM job_outputs
		 WHERE video_asset_id=$1 OR thumbnail_asset_id=$1 OR captions_asset_id=$1`,
		assetID,
	).Scan(&cnt); err != nil {
		if !httpkit.IsUndefinedTable(err) {
			httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db query failed", nil)
			return
		}
		cnt = 0
	}

	if cnt > 0 {
		httpkit.WriteErr(w, 409, "ASSET_IN_USE", "asset is referenced by job outputs", map[string]any{"asset_id": assetID})
		return
	}

	if err := h.sp.DeleteObject(ctx, objectKey); err != nil && !errors.Is(err, os.ErrNotExist) {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "storage delete failed", map[string]any{"object_key": objectKey})
		return
	}

	_, err = h.pool.Exec(ctx, `DELETE FROM assets WHERE id=$1`, assetID)
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db delete failed", nil)
		return
	}

	w.WriteHeader(204)
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func guessExt(contentType string) string {
	if contentType == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

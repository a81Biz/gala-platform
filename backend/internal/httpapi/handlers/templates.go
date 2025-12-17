package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gala/internal/httpapi/util"
	"gala/internal/httpkit"
)

type TemplateFormat struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

type CreateTemplateRequest struct {
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	DurationMs   *int            `json:"duration_ms,omitempty"`
	Format       *TemplateFormat `json:"format,omitempty"`
	ParamsSchema map[string]any  `json:"params_schema,omitempty"`
	Defaults     map[string]any  `json:"defaults,omitempty"`
}

type UpdateTemplateRequest struct {
	Type         *string         `json:"type,omitempty"`
	Name         *string         `json:"name,omitempty"`
	DurationMs   *int            `json:"duration_ms,omitempty"`
	Format       *TemplateFormat `json:"format,omitempty"`
	ParamsSchema *map[string]any `json:"params_schema,omitempty"`
	Defaults     *map[string]any `json:"defaults,omitempty"`
}

func (h *Handler) PostTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateTemplateRequest
	if err := httpkit.DecodeJSON(r, &req); err != nil {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "invalid json body", nil)
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	req.Name = strings.TrimSpace(req.Name)

	if req.Type == "" {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "type is required", map[string]any{"field": "type"})
		return
	}
	if req.Name == "" {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "name is required", map[string]any{"field": "name"})
		return
	}

	// JSONB payloads
	var (
		formatJSON, paramsSchemaJSON, defaultsJSON any
	)

	if req.Format != nil {
		b, _ := json.Marshal(req.Format)
		formatJSON = b
	} else {
		formatJSON = nil
	}

	if req.ParamsSchema != nil {
		b, _ := json.Marshal(req.ParamsSchema)
		paramsSchemaJSON = b
	} else {
		paramsSchemaJSON = nil
	}

	if req.Defaults != nil {
		b, _ := json.Marshal(req.Defaults)
		defaultsJSON = b
	} else {
		defaultsJSON = nil
	}

	id := util.NewID("tpl")
	createdAt := time.Now().UTC()

	_, err := h.pool.Exec(ctx, `
		INSERT INTO templates (id, type, name, duration_ms, format, params_schema, defaults, created_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8)
	`, id, req.Type, req.Name, req.DurationMs, formatJSON, paramsSchemaJSON, defaultsJSON, createdAt)

	if err != nil {
		if isUniqueViolation(err) {
			httpkit.WriteErr(w, 409, "TEMPLATE_NAME_EXISTS", "template name already exists", map[string]any{"field": "name"})
			return
		}
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db insert failed", nil)
		return
	}

	resp := map[string]any{
		"template": map[string]any{
			"id":            id,
			"type":          req.Type,
			"name":          req.Name,
			"duration_ms":   req.DurationMs,
			"format":        req.Format,
			"params_schema": req.ParamsSchema,
			"defaults":      req.Defaults,
			"created_at":    createdAt,
		},
	}
	httpkit.WriteJSON(w, 201, resp)
}

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pool.Query(ctx, `
		SELECT id, type, name, duration_ms, format, params_schema, defaults, created_at
		FROM templates
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db query failed", nil)
		return
	}
	defer rows.Close()

	templates := []map[string]any{}

	for rows.Next() {
		var (
			id, typ, name                           string
			durationMs                              *int
			formatBytes, paramsBytes, defaultsBytes []byte
			createdAt                               time.Time
		)

		if err := rows.Scan(&id, &typ, &name, &durationMs, &formatBytes, &paramsBytes, &defaultsBytes, &createdAt); err != nil {
			httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "row scan failed", nil)
			return
		}

		var format any
		var params any
		var defaults any
		_ = json.Unmarshal(formatBytes, &format)
		_ = json.Unmarshal(paramsBytes, &params)
		_ = json.Unmarshal(defaultsBytes, &defaults)

		templates = append(templates, map[string]any{
			"id":            id,
			"type":          typ,
			"name":          name,
			"duration_ms":   durationMs,
			"format":        format,
			"params_schema": params,
			"defaults":      defaults,
			"created_at":    createdAt,
		})
	}

	httpkit.WriteJSON(w, 200, map[string]any{"templates": templates})
}

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	templateID := chi.URLParam(r, "templateId")

	var (
		id, typ, name                           string
		durationMs                              *int
		formatBytes, paramsBytes, defaultsBytes []byte
		createdAt                               time.Time
	)

	err := h.pool.QueryRow(ctx, `
		SELECT id, type, name, duration_ms, format, params_schema, defaults, created_at
		FROM templates
		WHERE id=$1 AND deleted_at IS NULL
	`, templateID).Scan(&id, &typ, &name, &durationMs, &formatBytes, &paramsBytes, &defaultsBytes, &createdAt)

	if err != nil {
		httpkit.WriteErr(w, 404, "TEMPLATE_NOT_FOUND", "template not found", map[string]any{"template_id": templateID})
		return
	}

	var format any
	var params any
	var defaults any
	_ = json.Unmarshal(formatBytes, &format)
	_ = json.Unmarshal(paramsBytes, &params)
	_ = json.Unmarshal(defaultsBytes, &defaults)

	httpkit.WriteJSON(w, 200, map[string]any{
		"template": map[string]any{
			"id":            id,
			"type":          typ,
			"name":          name,
			"duration_ms":   durationMs,
			"format":        format,
			"params_schema": params,
			"defaults":      defaults,
			"created_at":    createdAt,
		},
	})
}

func (h *Handler) PatchTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	templateID := chi.URLParam(r, "templateId")

	// read existing first
	var (
		id, typ, name                           string
		durationMs                              *int
		formatBytes, paramsBytes, defaultsBytes []byte
		createdAt                               time.Time
	)

	err := h.pool.QueryRow(ctx, `
		SELECT id, type, name, duration_ms, format, params_schema, defaults, created_at
		FROM templates
		WHERE id=$1 AND deleted_at IS NULL
	`, templateID).Scan(&id, &typ, &name, &durationMs, &formatBytes, &paramsBytes, &defaultsBytes, &createdAt)

	if err != nil {
		httpkit.WriteErr(w, 404, "TEMPLATE_NOT_FOUND", "template not found", map[string]any{"template_id": templateID})
		return
	}

	var req UpdateTemplateRequest
	if err := httpkit.DecodeJSON(r, &req); err != nil {
		httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "invalid json body", nil)
		return
	}

	if req.Type != nil {
		typ = strings.TrimSpace(*req.Type)
		if typ == "" {
			httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "type cannot be empty", map[string]any{"field": "type"})
			return
		}
	}
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			httpkit.WriteErr(w, 400, "VALIDATION_ERROR", "name cannot be empty", map[string]any{"field": "name"})
			return
		}
	}
	if req.DurationMs != nil {
		durationMs = req.DurationMs
	}

	// JSONB payloads
	var formatJSON, paramsSchemaJSON, defaultsJSON any

	if req.Format != nil {
		b, _ := json.Marshal(req.Format)
		formatJSON = b
	} else {
		// keep existing
		formatJSON = formatBytes
	}

	if req.ParamsSchema != nil {
		b, _ := json.Marshal(*req.ParamsSchema)
		paramsSchemaJSON = b
	} else {
		paramsSchemaJSON = paramsBytes
	}

	if req.Defaults != nil {
		b, _ := json.Marshal(*req.Defaults)
		defaultsJSON = b
	} else {
		defaultsJSON = defaultsBytes
	}

	_, err = h.pool.Exec(ctx, `
		UPDATE templates
		SET type=$2, name=$3, duration_ms=$4, format=$5::jsonb, params_schema=$6::jsonb, defaults=$7::jsonb
		WHERE id=$1 AND deleted_at IS NULL
	`, templateID, typ, name, durationMs, formatJSON, paramsSchemaJSON, defaultsJSON)

	if err != nil {
		if isUniqueViolation(err) {
			httpkit.WriteErr(w, 409, "TEMPLATE_NAME_EXISTS", "template name already exists", map[string]any{"field": "name"})
			return
		}
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db update failed", nil)
		return
	}

	// return fresh
	h.GetTemplate(w, r)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	templateID := chi.URLParam(r, "templateId")

	cmd, err := h.pool.Exec(ctx, `
		UPDATE templates
		SET deleted_at=NOW()
		WHERE id=$1 AND deleted_at IS NULL
	`, templateID)
	if err != nil {
		httpkit.WriteErr(w, 500, "INTERNAL_ERROR", "db delete failed", nil)
		return
	}
	if cmd.RowsAffected() == 0 {
		httpkit.WriteErr(w, 404, "TEMPLATE_NOT_FOUND", "template not found", map[string]any{"template_id": templateID})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 23505 = unique_violation
		return pgErr.Code == "23505"
	}
	return false
}

// keep goimports from deleting util if your IDE complains in this file (rare)
var _ = util.NewID

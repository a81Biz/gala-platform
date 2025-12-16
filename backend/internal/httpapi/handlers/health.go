package handlers

import (
	"net/http"

	"gala/internal/httpkit"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	httpkit.WriteJSON(w, 200, map[string]any{
		"status":  "ok",
		"service": "gala-api",
		"version": "0.1.0",
	})
}

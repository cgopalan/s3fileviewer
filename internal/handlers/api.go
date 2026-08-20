package handlers

import (
	"net/http"
)

func (h *Handler) Schema(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")
	member := r.URL.Query().Get("member")

	if err := h.validateBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	cols, err := h.duckdb.Schema(r.Context(), bucket, key, member)
	if err != nil {
		h.renderPartial(w, "partials/error.html", map[string]any{"Error": err.Error()})
		return
	}

	h.renderPartial(w, "partials/columns.html", map[string]any{"Columns": cols})
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	bucket := r.FormValue("bucket")
	key := r.FormValue("key")
	member := r.FormValue("member")
	query := r.FormValue("query")

	if err := h.validateBucket(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	result, err := h.duckdb.Query(r.Context(), bucket, key, member, query)
	if err != nil {
		h.renderPartial(w, "partials/error.html", map[string]any{"Error": err.Error()})
		return
	}

	h.renderPartial(w, "partials/results.html", map[string]any{
		"Columns":   result.Columns,
		"Rows":      result.Rows,
		"RowCount":  result.RowCount,
		"Truncated": result.Truncated,
		"MaxRows":   h.cfg.MaxQueryRows,
	})
}

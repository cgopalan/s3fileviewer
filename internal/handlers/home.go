package handlers

import (
	"net/http"
)

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	h.render(w, "home_content", map[string]any{
		"Title": "S3 File Viewer",
	})
}

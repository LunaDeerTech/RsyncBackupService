package handler

import "net/http"

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

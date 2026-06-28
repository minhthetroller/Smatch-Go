package handler

import (
	"net/http"
	"strconv"

	"github.com/smatch/badminton-backend/internal/service"
)

type SearchHandler struct {
	svc *service.SearchService
}

func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// GET /api/search/autocomplete?q=...
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	resp, err := h.svc.Autocomplete(r.Context(), q)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// GET /api/search/courts?q=...&page=1&limit=10
func (h *SearchHandler) SearchCourts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := 1
	limit := 10
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	result, err := h.svc.SearchCourts(r.Context(), q, page, limit)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendPaginated(w, result.Courts, page, limit, result.Total)
}

// GET /api/search/popular
func (h *SearchHandler) Popular(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Popular(r.Context())
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// POST /api/admin/search/reindex
func (h *SearchHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.Reindex(r.Context())
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]int{"indexed": count}, 200)
}

// GET /api/admin/search/stats
func (h *SearchHandler) Stats(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.Stats(r.Context())
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]int64{"autocompleteEntries": count}, 200)
}

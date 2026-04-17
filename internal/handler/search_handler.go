package handler

import (
	"net/http"
	"strconv"

	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
)

type SearchHandler struct {
	redis      *service.RedisService
	searchRepo *repository.SearchRepository
	courtRepo  *repository.CourtRepository
}

func NewSearchHandler(redis *service.RedisService, sr *repository.SearchRepository, cr *repository.CourtRepository) *SearchHandler {
	return &SearchHandler{redis: redis, searchRepo: sr, courtRepo: cr}
}

// GET /api/search/autocomplete?q=...
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	suggestions, err := h.redis.SearchAutocomplete(r.Context(), q, 10)
	if err != nil {
		sendError(w, "Search failed", "INTERNAL_ERROR", 500)
		return
	}
	var dtos []dto.AutocompleteSuggestion
	for _, s := range suggestions {
		dtos = append(dtos, dto.AutocompleteSuggestion{ID: s.ID, Text: s.Text, Score: s.Score})
	}
	if dtos == nil {
		dtos = []dto.AutocompleteSuggestion{}
	}
	sendSuccess(w, dto.AutocompleteResponse{Suggestions: dtos}, 200)
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
	courts, total, err := h.searchRepo.SearchCourts(r.Context(), q, page, limit)
	if err != nil {
		sendError(w, "Search failed", "INTERNAL_ERROR", 500)
		return
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		resp[i] = mapCourtToDTO(c)
	}
	// Track search query in Redis.
	h.redis.TrackSearch(r.Context(), q) //nolint:errcheck
	sendPaginated(w, resp, page, limit, total)
}

// GET /api/search/popular
func (h *SearchHandler) Popular(w http.ResponseWriter, r *http.Request) {
	queries, err := h.redis.GetPopularSearches(r.Context(), 10)
	if err != nil {
		sendError(w, "Failed to get popular searches", "INTERNAL_ERROR", 500)
		return
	}
	if queries == nil {
		queries = []string{}
	}
	sendSuccess(w, dto.PopularSearchesResponse{Queries: queries}, 200)
}

// POST /api/admin/search/reindex
func (h *SearchHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	courts, err := h.searchRepo.GetAllCourtNames(r.Context())
	if err != nil {
		sendError(w, "Reindex failed", "INTERNAL_ERROR", 500)
		return
	}
	h.redis.ClearAutocomplete(r.Context()) //nolint:errcheck
	for _, c := range courts {
		terms := []string{c.Name}
		if c.District != "" {
			terms = append(terms, c.District)
		}
		h.redis.AddToAutocomplete(r.Context(), c.ID, c.Name, terms, 0) //nolint:errcheck
	}
	sendSuccess(w, map[string]int{"indexed": len(courts)}, 200)
}

// GET /api/admin/search/stats
func (h *SearchHandler) Stats(w http.ResponseWriter, r *http.Request) {
	count, err := h.redis.GetAutocompleteCount(r.Context())
	if err != nil {
		sendError(w, "Failed to get stats", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, map[string]int64{"autocompleteEntries": count}, 200)
}

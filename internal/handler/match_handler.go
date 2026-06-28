package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
)

type MatchHandler struct {
	svc *service.MatchService
}

func NewMatchHandler(svc *service.MatchService) *MatchHandler {
	return &MatchHandler{svc: svc}
}

// GetAllMatches GET /api/matches
func (h *MatchHandler) GetAllMatches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	filter := repository.MatchFilter{
		Page:  page,
		Limit: limit,
	}
	if v := q.Get("courtId"); v != "" {
		filter.CourtID = &v
	}
	if v := q.Get("skillLevel"); v != "" {
		filter.SkillLevel = &v
	}
	if v := q.Get("playerFormat"); v != "" {
		filter.PlayerFormat = &v
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}
	if v := q.Get("date"); v != "" {
		filter.Date = &v
	}
	if v := q.Get("dateFrom"); v != "" {
		filter.DateFrom = &v
	}
	if v := q.Get("dateTo"); v != "" {
		filter.DateTo = &v
	}
	filter.IncludeExpired = q.Get("includeExpired") == "true"

	resp, total, err := h.svc.GetAllMatches(r.Context(), filter)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendPaginated(w, resp, page, limit, total)
}

// GET /api/matches/hosted
func (h *MatchHandler) GetHostedMatches(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	includeExpired := r.URL.Query().Get("includeExpired") == "true"
	resp, err := h.svc.GetHostedMatches(r.Context(), user.ID, includeExpired)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// GET /api/matches/joined
func (h *MatchHandler) GetJoinedMatches(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	includeExpired := r.URL.Query().Get("includeExpired") == "true"
	resp, err := h.svc.GetJoinedMatches(r.Context(), user.ID, includeExpired)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// GET /api/matches/:id
func (h *MatchHandler) GetMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var currentID *string
	if user := middleware.UserFromContext(r.Context()); user != nil {
		currentID = &user.ID
	}
	resp, err := h.svc.GetMatch(r.Context(), id, currentID)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// POST /api/matches
func (h *MatchHandler) CreateMatch(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req dto.CreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.CreateMatch(r.Context(), &req, user.ID)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 201)
}

// UpdateMatch PUT /api/matches/:id
func (h *MatchHandler) UpdateMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())
	var req dto.UpdateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.UpdateMatch(r.Context(), id, &req, user.ID)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// CancelMatch DELETE /api/matches/:id - sets status=CANCELLED
func (h *MatchHandler) CancelMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())
	if err := h.svc.CancelMatch(r.Context(), id, user.ID); err != nil {
		sendAppError(w, err)
		return
	}
	w.WriteHeader(204)
}

// JoinMatch POST /api/matches/:id/join
func (h *MatchHandler) JoinMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())
	var req dto.JoinMatchRequest
	_ = json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck

	resp, err := h.svc.JoinMatch(r.Context(), id, user.ID, req.Message)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 201)
}

// LeaveMatch DELETE /api/matches/:id/leave
func (h *MatchHandler) LeaveMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())
	if err := h.svc.LeaveMatch(r.Context(), id, user.ID); err != nil {
		sendAppError(w, err)
		return
	}
	w.WriteHeader(204)
}

// GetJoinRequests GET /api/matches/:id/requests
func (h *MatchHandler) GetJoinRequests(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())
	resp, err := h.svc.GetJoinRequests(r.Context(), id, user.ID)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// RespondToJoinRequest PUT /api/matches/:id/requests/:playerId/respond
func (h *MatchHandler) RespondToJoinRequest(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "id")
	playerID := chi.URLParam(r, "playerId")
	user := middleware.UserFromContext(r.Context())
	var req dto.RespondToJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.RespondToJoinRequest(r.Context(), matchID, playerID, user.ID, req.Status)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

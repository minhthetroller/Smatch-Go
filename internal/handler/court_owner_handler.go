package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
)

type CourtOwnerHandler struct {
	service *service.CourtOwnerService
}

func NewCourtOwnerHandler(s *service.CourtOwnerService) *CourtOwnerHandler {
	return &CourtOwnerHandler{service: s}
}

func (h *CourtOwnerHandler) ListCourts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courts, err := h.service.ListMyCourts(r.Context(), user.ID)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, courts, 200)
}

func (h *CourtOwnerHandler) GetCourtStats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courtID := chi.URLParam(r, "id")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	stats, err := h.service.GetCourtStats(r.Context(), user.ID, courtID, period)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, stats, 200)
}

func (h *CourtOwnerHandler) CloseCourt(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courtID := chi.URLParam(r, "id")

	var req dto.CloseCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request", "BAD_REQUEST", 400)
		return
	}

	if err := h.service.CloseCourt(r.Context(), user.ID, courtID, req); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]string{"status": "closed"}, 200)
}

func (h *CourtOwnerHandler) OpenCourt(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courtID := chi.URLParam(r, "id")
	date := r.URL.Query().Get("date")

	if err := h.service.OpenCourt(r.Context(), user.ID, courtID, date); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]string{"status": "opened"}, 200)
}

func (h *CourtOwnerHandler) CloseSubCourt(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courtID := chi.URLParam(r, "id")
	subCourtID := chi.URLParam(r, "subId")

	var req dto.CloseSubCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request", "BAD_REQUEST", 400)
		return
	}

	if err := h.service.CloseSubCourt(r.Context(), user.ID, courtID, subCourtID, req); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]string{"status": "closed"}, 200)
}

func (h *CourtOwnerHandler) OpenSubCourt(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	courtID := chi.URLParam(r, "id")
	subCourtID := chi.URLParam(r, "subId")
	date := r.URL.Query().Get("date")

	if err := h.service.OpenSubCourt(r.Context(), user.ID, courtID, subCourtID, date); err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, map[string]string{"status": "opened"}, 200)
}

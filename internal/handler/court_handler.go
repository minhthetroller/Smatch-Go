package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/service"
)

type CourtHandler struct {
	svc *service.CourtService
}

func NewCourtHandler(svc *service.CourtService) *CourtHandler {
	return &CourtHandler{svc: svc}
}

// GET /api/courts - List all courts.
func (h *CourtHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.List(r.Context())
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// GET /api/courts/nearby - Courts within radius.
// radius param: required to include "km" suffix (e.g. radius=10km). Range: [5, 50] km. Default: 5km.
func (h *CourtHandler) Nearby(w http.ResponseWriter, r *http.Request) {
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	if err1 != nil || err2 != nil {
		sendError(w, "lat and lng are required", "BAD_REQUEST", 400)
		return
	}

	radiusKm, err := service.ParseNearbyRadius(r.URL.Query().Get("radius"))
	if err != nil {
		sendAppError(w, err)
		return
	}

	resp, err := h.svc.Nearby(r.Context(), service.NearbyQuery{Lat: lat, Lng: lng, RadiusKm: radiusKm})
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// GET /api/courts/:id - Single court.
func (h *CourtHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// POST /api/courts - Create court (admin only).
func (h *CourtHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 201)
}

// PUT /api/courts/:id - Update court (admin only).
func (h *CourtHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dto.UpdateCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.Update(r.Context(), id, &req)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// DELETE /api/courts/:id - Delete court (admin only).
func (h *CourtHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		sendAppError(w, err)
		return
	}
	w.WriteHeader(204)
}

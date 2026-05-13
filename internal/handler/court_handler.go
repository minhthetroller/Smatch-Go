package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
)

type CourtHandler struct {
	courtRepo *repository.CourtRepository
}

func NewCourtHandler(cr *repository.CourtRepository) *CourtHandler {
	return &CourtHandler{courtRepo: cr}
}

// GET /api/courts - List all courts.
func (h *CourtHandler) List(w http.ResponseWriter, r *http.Request) {
	courts, err := h.courtRepo.FindAll(r.Context())
	if err != nil {
		sendError(w, "Failed to get courts", "INTERNAL_ERROR", 500)
		return
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		resp[i] = mapCourtToDTO(c)
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

	const minRadiusKm = 5.0
	const maxRadiusKm = 50.0

	radiusKm := minRadiusKm
	if raw := r.URL.Query().Get("radius"); raw != "" {
		if !strings.HasSuffix(raw, "km") {
			sendError(w, fmt.Sprintf("radius must include unit suffix, e.g. radius=%.0fkm (accepted range: %.0f–%.0fkm)", minRadiusKm, minRadiusKm, maxRadiusKm), "BAD_REQUEST", 400)
			return
		}
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(raw, "km"), 64)
		if err != nil {
			sendError(w, "radius value must be a number in km, e.g. radius=10km", "BAD_REQUEST", 400)
			return
		}
		radiusKm = parsed
	}
	if radiusKm < minRadiusKm || radiusKm > maxRadiusKm {
		sendError(w, fmt.Sprintf("radius must be between %.0fkm and %.0fkm", minRadiusKm, maxRadiusKm), "BAD_REQUEST", 400)
		return
	}

	courts, distances, err := h.courtRepo.FindNearby(r.Context(), lat, lng, radiusKm*1000)
	if err != nil {
		sendError(w, "Failed to get courts", "INTERNAL_ERROR", 500)
		return
	}
	resp := make([]dto.CourtResponse, len(courts))
	for i, c := range courts {
		cr := mapCourtToDTO(c)
		if i < len(distances) {
			distMeters := distances[i]
			distKm := distMeters / 1000
			cr.Distance = &distMeters
			cr.DistanceKm = &distKm
		}
		resp[i] = cr
	}
	sendSuccess(w, resp, 200)
}

// GET /api/courts/:id - Single court.
func (h *CourtHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	court, err := h.courtRepo.FindByID(r.Context(), id)
	if err != nil {
		sendError(w, "Failed to get court", "INTERNAL_ERROR", 500)
		return
	}
	if court == nil {
		sendError(w, "Court not found", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, mapCourtToDTO(court), 200)
}

// POST /api/courts - Create court (admin only).
func (h *CourtHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	c := &domain.Court{
		Name:            req.Name,
		Description:     req.Description,
		PhoneNumbers:    req.PhoneNumbers,
		AddressStreet:   req.AddressStreet,
		AddressWard:     req.AddressWard,
		AddressDistrict: req.AddressDistrict,
		AddressCity:     req.AddressCity,
		Details:         req.Details,
		OpeningHours:    req.OpeningHours,
		Lat:             req.Lat,
		Lng:             req.Lng,
	}
	created, err := h.courtRepo.Create(r.Context(), c)
	if err != nil {
		sendError(w, "Failed to create court", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, mapCourtToDTO(created), 201)
}

// PUT /api/courts/:id - Update court (admin only).
func (h *CourtHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req dto.UpdateCourtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	fields := map[string]interface{}{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.PhoneNumbers != nil {
		fields["phone_numbers"] = req.PhoneNumbers
	}
	if req.Details != nil {
		fields["details"] = req.Details
	}
	if req.OpeningHours != nil {
		fields["opening_hours"] = req.OpeningHours
	}

	updated, err := h.courtRepo.Update(r.Context(), id, fields)
	if err != nil {
		sendError(w, "Failed to update court", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, mapCourtToDTO(updated), 200)
}

// DELETE /api/courts/:id - Delete court (admin only).
func (h *CourtHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.courtRepo.Delete(r.Context(), id); err != nil {
		sendError(w, "Failed to delete court", "INTERNAL_ERROR", 500)
		return
	}
	w.WriteHeader(204)
}

func mapCourtToDTO(c *domain.Court) dto.CourtResponse {
	resp := dto.CourtResponse{
		ID:              c.ID,
		Name:            c.Name,
		Description:     c.Description,
		PhoneNumbers:    c.PhoneNumbers,
		AddressStreet:   c.AddressStreet,
		AddressWard:     c.AddressWard,
		AddressDistrict: c.AddressDistrict,
		AddressCity:     c.AddressCity,
		Details:         c.Details,
		OpeningHours:    c.OpeningHours,
		Lat:             c.Lat,
		Lng:             c.Lng,
		CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if resp.PhoneNumbers == nil {
		resp.PhoneNumbers = []string{}
	}
	return resp
}

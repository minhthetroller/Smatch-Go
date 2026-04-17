package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
)

type AvailabilityHandler struct {
	svc *service.AvailabilityService
}

func NewAvailabilityHandler(svc *service.AvailabilityService) *AvailabilityHandler {
	return &AvailabilityHandler{svc: svc}
}

// GET /api/courts/:courtId/availability?date=YYYY-MM-DD
func (h *AvailabilityHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	courtID := chi.URLParam(r, "courtId")
	date := r.URL.Query().Get("date")
	if date == "" {
		sendError(w, "date is required", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.GetCourtAvailability(r.Context(), courtID, date)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, resp, 200)
}

// POST /api/bookings
func (h *AvailabilityHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req dto.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}

	userID := ""
	if user != nil {
		userID = user.ID
	}

	bookings, err := h.svc.CreateBooking(r.Context(), &req, userID)
	if err != nil {
		sendAppError(w, err)
		return
	}

	// Return array if multiple bookings, single object if one.
	if len(bookings) == 1 {
		sendSuccess(w, bookings[0], 201)
	} else {
		sendSuccess(w, bookings, 201)
	}
}

// GET /api/bookings/:id
func (h *AvailabilityHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	booking, err := h.svc.GetBookingByID(r.Context(), id)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, booking, 200)
}

// DELETE /api/bookings/:id
func (h *AvailabilityHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	booking, err := h.svc.CancelBooking(r.Context(), id)
	if err != nil {
		sendAppError(w, err)
		return
	}
	sendSuccess(w, booking, 200)
}

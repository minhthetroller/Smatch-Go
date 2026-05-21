package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
	"go.uber.org/zap"
)

type AvailabilityHandler struct {
	svc    *service.AvailabilityService
	logger *zap.Logger
}

func NewAvailabilityHandler(svc *service.AvailabilityService, logger ...*zap.Logger) *AvailabilityHandler {
	l := zap.NewNop()
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &AvailabilityHandler{svc: svc, logger: l}
}

func (h *AvailabilityHandler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

func bookingRequestItems(req dto.CreateBookingRequest) []dto.SingleBookingItem {
	items := req.Bookings
	if len(items) == 0 && req.SubCourtID != "" {
		items = []dto.SingleBookingItem{{
			SubCourtID: req.SubCourtID,
			Date:       req.Date,
			StartTime:  req.StartTime,
			EndTime:    req.EndTime,
		}}
	}
	return items
}

func bookingItemLogFields(items []dto.SingleBookingItem) []zap.Field {
	fields := []zap.Field{zap.Int("booking_item_count", len(items))}
	if len(items) == 1 {
		fields = append(fields,
			zap.String("sub_court_id", items[0].SubCourtID),
			zap.String("date", items[0].Date),
			zap.String("start_time", items[0].StartTime),
			zap.String("end_time", items[0].EndTime),
		)
	}
	return fields
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
		h.log().Warn("booking create invalid request body", append(requestLogFields(r), zap.Error(err))...)
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	items := bookingRequestItems(req)
	h.log().Info("booking create request received",
		append(append(requestLogFields(r), bookingItemLogFields(items)...),
			zap.Bool("has_guest_name", req.GuestName != ""),
			zap.Bool("has_guest_phone", req.GuestPhone != ""),
			zap.Bool("has_guest_email", req.GuestEmail != nil && *req.GuestEmail != ""),
			zap.Bool("has_notes", req.Notes != nil && *req.Notes != ""),
		)...,
	)

	userID := ""
	if user != nil {
		userID = user.ID
	}

	bookings, err := h.svc.CreateBooking(r.Context(), &req, userID)
	if err != nil {
		fields := append(append(requestLogFields(r), bookingItemLogFields(items)...), zap.Error(err))
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			fields = append(fields,
				zap.String("error_code", appErr.Code),
				zap.Int("error_status", appErr.Status),
			)
		}
		h.log().Warn("booking create failed", fields...)
		sendAppError(w, err)
		return
	}
	h.log().Info("booking create succeeded",
		append(append(requestLogFields(r), bookingItemLogFields(items)...),
			zap.Int("booking_count", len(bookings)),
		)...,
	)

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

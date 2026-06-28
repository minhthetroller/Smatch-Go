package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/service"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	svc                *service.PaymentService
	logger             *zap.Logger
	port               int
	nodeEnv            string
	paymentWSTicketTTL int
}

func NewPaymentHandler(
	svc *service.PaymentService,
	logger *zap.Logger,
	paymentWSTicketTTL, port int,
	nodeEnv string,
) *PaymentHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	svc.SetPaymentWSTicketTTL(paymentWSTicketTTL)
	return &PaymentHandler{
		svc:                svc,
		logger:             logger,
		port:               port,
		nodeEnv:            nodeEnv,
		paymentWSTicketTTL: paymentWSTicketTTL,
	}
}

func (h *PaymentHandler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

func requestLogFields(r *http.Request) []zap.Field {
	user := middleware.UserFromContext(r.Context())
	fields := []zap.Field{
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote_addr", r.RemoteAddr),
		zap.Bool("has_authorization", r.Header.Get("Authorization") != ""),
		zap.Bool("authenticated", user != nil),
	}
	if user != nil {
		fields = append(fields,
			zap.String("user_id", user.ID),
			zap.Bool("user_is_anonymous", user.IsAnonymous),
		)
	}
	return fields
}

func (h *PaymentHandler) wsURLParams(r *http.Request) service.WSURLParams {
	return service.WSURLParams{
		ForwardedProto: r.Header.Get("X-Forwarded-Proto"),
		ForwardedHost:  r.Header.Get("X-Forwarded-Host"),
		Host:           r.Host,
		TLS:            r.TLS != nil,
		Port:           h.port,
		NodeEnv:        h.nodeEnv,
	}
}

// CreatePayment POST /api/payments/create
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log().Warn("payment create invalid request body", append(requestLogFields(r), zap.Error(err))...)
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.CreatePayment(r.Context(), req.BookingID, h.wsURLParams(r))
	if err != nil {
		h.logPaymentErr(r, "payment create", err)
		sendAppError(w, service.AppErrorFromPaymentErr(err))
		return
	}
	sendSuccess(w, resp, 201)
}

// Callback POST /api/payments/callback - ZaloPay webhook.
func (h *PaymentHandler) Callback(w http.ResponseWriter, r *http.Request) {
	var req dto.ZaloPayCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log().Warn("payment callback invalid request body", append(requestLogFields(r), zap.Error(err))...)
		writeZaloPayCallback(w, dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "invalid request"})
		return
	}
	result := h.svc.Callback(r.Context(), req.Data, req.MAC)
	writeZaloPayCallback(w, dto.ZaloPayCallbackResponse{ReturnCode: result.ReturnCode, ReturnMessage: result.ReturnMessage})
}

// GetPayment GET /api/payments/:id
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	resp, err := h.svc.GetPayment(r.Context(), id)
	if err != nil {
		h.logPaymentErr(r, "payment detail", err)
		sendAppError(w, service.AppErrorFromPaymentErr(err))
		return
	}
	sendSuccess(w, resp, 200)
}

// CancelPayment POST /api/payments/:id/cancel
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	payment, err := h.svc.CancelPayment(r.Context(), id, "Payment was cancelled")
	if err != nil || payment == nil {
		h.logPaymentErr(r, "payment cancel", err)
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, service.MapPaymentToDTO(payment), 200)
}

// PaymentStatusNotification exposes the service method to the WS hub wiring.
func (h *PaymentHandler) PaymentStatusNotification(ctx context.Context, paymentID string) (*ws.PaymentNotification, error) {
	return h.svc.PaymentStatusNotification(ctx, paymentID)
}

// CancelPaymentByID is the WS-disconnect hook used by the WS hub wiring.
func (h *PaymentHandler) CancelPaymentByID(ctx context.Context, paymentID string) {
	h.svc.CancelPaymentByID(ctx, paymentID)
}

// CreateMatchPayment POST /api/matches/:matchId/payment - Create payment for match join.
func (h *PaymentHandler) CreateMatchPayment(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	user := middleware.UserFromContext(r.Context())

	var req dto.CreateMatchPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	resp, err := h.svc.CreateMatchPayment(r.Context(), matchID, user.ID, req.MatchPlayerID, h.wsURLParams(r))
	if err != nil {
		sendAppError(w, service.AppErrorFromPaymentErr(err))
		return
	}
	sendSuccess(w, resp, 201)
}

// GetMatchPaymentStatus GET /api/matches/:matchId/payment/:paymentId/status
func (h *PaymentHandler) GetMatchPaymentStatus(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentId")
	resp, err := h.svc.GetMatchPaymentStatus(r.Context(), paymentID)
	if err != nil {
		sendAppError(w, service.AppErrorFromPaymentErr(err))
		return
	}
	sendSuccess(w, resp, 200)
}

// GetBookingPayment GET /api/bookings/:id/payment — returns the latest payment for a booking
func (h *PaymentHandler) GetBookingPayment(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "id")
	resp, err := h.svc.GetBookingPayment(r.Context(), bookingID)
	if err != nil {
		sendAppError(w, service.AppErrorFromPaymentErr(err))
		return
	}
	sendSuccess(w, resp, 200)
}

// ==================== Helpers ====================

func (h *PaymentHandler) logPaymentErr(r *http.Request, op string, err error) {
	if err == nil {
		return
	}
	fields := append(requestLogFields(r), zap.Error(err))
	h.log().Warn(op+" failed", fields...)
}

func writeZaloPayCallback(w http.ResponseWriter, resp dto.ZaloPayCallbackResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

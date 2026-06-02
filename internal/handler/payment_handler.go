package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	paymentRepo        paymentStore
	availRepo          availabilityStore
	matchRepo          matchPaymentStore
	redis              paymentRedis
	zalopay            paymentGateway
	paymentSvc         *service.PaymentService
	hub                *ws.Hub
	logger             *zap.Logger
	paymentWSTicketTTL int
	port               int
	nodeEnv            string
}

type paymentStore interface {
	FindByID(ctx context.Context, id string) (*domain.Payment, error)
	FindByAppTransID(ctx context.Context, appTransID string) (*domain.Payment, error)
	FindLatestPendingByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error)
	Create(ctx context.Context, bookingID *string, matchPlayerID *string, paymentType domain.PaymentType, appTransID string, amount int) (*domain.Payment, error)
	UpdateOrderURL(ctx context.Context, id, orderURL, zpTransToken string) error
	UpdatePendingStatus(ctx context.Context, id string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error)
	UpdatePendingStatusByAppTransID(ctx context.Context, appTransID string, status domain.PaymentStatus, zpTransID *string, callbackData json.RawMessage) (*domain.Payment, error)
	FindPendingPayments(ctx context.Context) ([]*domain.Payment, error)
}

type availabilityStore interface {
	GetBookingByID(ctx context.Context, id string) (*repository.BookingRow, error)
	GetBookingsByGroupID(ctx context.Context, groupID string) ([]*repository.RawBooking, error)
	UpdateBookingStatus(ctx context.Context, id, status string) error
	MarkExpiredPendingBookings(ctx context.Context, timeoutSec int) error
}

type matchPaymentStore interface {
	FindByID(ctx context.Context, id string) (*repository.MatchRow, error)
	FindPlayerByID(ctx context.Context, playerID string) (*repository.MatchPlayerRow, error)
	GetNextPosition(ctx context.Context, matchID string) (int, error)
	UpdatePlayerStatus(ctx context.Context, playerID string, status domain.MatchPlayerStatus, position *int) (*repository.MatchPlayerRow, error)
	CountAcceptedPlayers(ctx context.Context, matchID string) (int, error)
	UpdateStatus(ctx context.Context, id string, status domain.MatchStatus) error
	MarkExpiredMatchPlayers(ctx context.Context, playerIDs []string) error
}

type paymentRedis interface {
	AcquireSlotLocks(ctx context.Context, slots []service.SlotLockSpec) (bool, error)
	ReleaseSlotLocks(ctx context.Context, slots []service.SlotLockSpec)
	CreatePaymentWSTicket(ctx context.Context, paymentID string, ttl time.Duration) (string, error)
}

type paymentGateway interface {
	GenerateAppTransID(bookingID string) string
	CreateOrder(ctx context.Context, input zalopay.CreateOrderInput) (*zalopay.CreateOrderResponse, error)
	QueryOrder(ctx context.Context, appTransID string) (*zalopay.QueryOrderResponse, error)
	VerifyCallback(data, mac string) (*zalopay.CallbackData, bool)
}

func NewPaymentHandler(
	pr *repository.PaymentRepository,
	ar *repository.AvailabilityRepository,
	mr *repository.MatchRepository,
	rs *service.RedisService,
	zp *zalopay.Client,
	hub *ws.Hub,
	logger *zap.Logger,
	paymentWSTicketTTL, port int,
	nodeEnv string,
) *PaymentHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	var redis paymentRedis
	if rs != nil {
		redis = rs
	}
	paymentSvc := service.NewPaymentService(pr, ar, mr, redis, zp, hub, logger)
	return &PaymentHandler{
		paymentRepo:        pr,
		availRepo:          ar,
		matchRepo:          mr,
		redis:              redis,
		zalopay:            zp,
		paymentSvc:         paymentSvc,
		hub:                hub,
		logger:             logger,
		paymentWSTicketTTL: paymentWSTicketTTL,
		port:               port,
		nodeEnv:            nodeEnv,
	}
}

func (h *PaymentHandler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

func (h *PaymentHandler) payments() *service.PaymentService {
	if h.paymentSvc != nil {
		return h.paymentSvc
	}
	h.paymentSvc = service.NewPaymentService(h.paymentRepo, h.availRepo, h.matchRepo, h.redis, h.zalopay, h.hub, h.log())
	return h.paymentSvc
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

func phoneLast4(phone string) string {
	if len(phone) < 4 {
		return ""
	}
	return phone[len(phone)-4:]
}

func (h *PaymentHandler) acquireSlotLocks(ctx context.Context, slots []service.SlotLockSpec) (bool, error) {
	if h.redis == nil || len(slots) == 0 {
		return true, nil
	}
	return h.redis.AcquireSlotLocks(ctx, slots)
}

func (h *PaymentHandler) releaseSlotLocks(ctx context.Context, slots []service.SlotLockSpec) {
	if h.redis == nil || len(slots) == 0 {
		return
	}
	h.redis.ReleaseSlotLocks(ctx, slots)
}

func (h *PaymentHandler) createPaymentWSTicket(ctx context.Context, payment *domain.Payment) (string, error) {
	if h.redis == nil {
		return "", fmt.Errorf("redis unavailable for payment websocket ticket")
	}
	ttl := h.paymentWSTicketTTL
	if ttl <= 0 {
		ttl = 60
	}
	duration := time.Duration(ttl) * time.Second
	remaining := h.payments().RemainingValidity(payment)
	if remaining <= 0 {
		return "", fmt.Errorf("payment transaction expired")
	}
	if duration > remaining {
		duration = remaining
	}
	return h.redis.CreatePaymentWSTicket(ctx, payment.ID, duration)
}

func (h *PaymentHandler) failPaymentWSTicket(w http.ResponseWriter, r *http.Request, paymentID string, slots []service.SlotLockSpec, err error) {
	if paymentID != "" {
		if _, updateErr := h.paymentRepo.UpdatePendingStatus(r.Context(), paymentID, domain.PaymentStatusFailed, nil, nil); updateErr != nil {
			h.log().Warn("payment websocket ticket status update failed",
				append(requestLogFields(r), zap.String("payment_id", paymentID), zap.Error(updateErr))...,
			)
		}
	}
	h.releaseSlotLocks(r.Context(), slots)
	h.log().Error("payment websocket ticket creation failed",
		append(requestLogFields(r), zap.String("payment_id", paymentID), zap.Error(err))...,
	)
	sendError(w, "Payment notification channel unavailable. Please try again.", "PAYMENT_WS_TICKET_ERROR", http.StatusServiceUnavailable)
}

// CreatePayment POST /api/payments/create
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	_ = middleware.UserFromContext(r.Context()) // auth enforced by middleware; user context not needed here

	var req dto.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log().Warn("payment create invalid request body", append(requestLogFields(r), zap.Error(err))...)
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	h.log().Info("payment create request received", append(requestLogFields(r), zap.String("booking_id", req.BookingID))...)
	if req.BookingID == "" {
		h.log().Warn("payment create missing booking id", requestLogFields(r)...)
		sendError(w, "bookingId is required", "BAD_REQUEST", 400)
		return
	}

	// 1. Get booking and validate.
	booking, err := h.availRepo.GetBookingByID(r.Context(), req.BookingID)
	if err != nil || booking == nil {
		fields := append(requestLogFields(r), zap.String("booking_id", req.BookingID))
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Warn("payment create booking not found", fields...)
		sendError(w, "Booking not found", "NOT_FOUND", 404)
		return
	}
	h.log().Info("payment create booking loaded",
		append(requestLogFields(r),
			zap.String("booking_id", booking.ID),
			zap.String("booking_status", booking.Status),
			zap.Int("amount", booking.TotalPrice),
			zap.Bool("has_guest_name", booking.GuestName != ""),
			zap.Bool("has_guest_phone", booking.GuestPhone != ""),
			zap.String("guest_phone_last4", phoneLast4(booking.GuestPhone)),
			zap.Bool("has_guest_email", booking.GuestEmail != nil && *booking.GuestEmail != ""),
			zap.Bool("has_user_id", booking.UserID != nil),
			zap.String("court_id", booking.CourtID),
			zap.String("sub_court_id", booking.SubCourtID),
			zap.String("date", booking.Date),
			zap.String("start_time", booking.StartTime),
			zap.String("end_time", booking.EndTime),
		)...,
	)
	if booking.Status != "pending" {
		h.log().Warn("payment create booking invalid status",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.String("booking_status", booking.Status),
			)...,
		)
		sendError(w, "Booking is not in pending status", "INVALID_STATUS", 400)
		return
	}

	// 2. Check for group bookings (same groupId).
	var groupBookingIDs []string
	if booking.GroupID != nil {
		groupBookings, err := h.availRepo.GetBookingsByGroupID(r.Context(), *booking.GroupID)
		if err == nil {
			for _, gb := range groupBookings {
				groupBookingIDs = append(groupBookingIDs, gb.ID)
			}
		} else {
			h.log().Warn("payment create group bookings lookup failed",
				append(requestLogFields(r),
					zap.String("booking_id", req.BookingID),
					zap.String("group_id", *booking.GroupID),
					zap.Error(err),
				)...,
			)
		}
	}
	if len(groupBookingIDs) == 0 {
		groupBookingIDs = []string{req.BookingID}
	}
	h.log().Info("payment create group resolved",
		append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.Int("group_booking_count", len(groupBookingIDs)),
			zap.Bool("has_group_id", booking.GroupID != nil),
		)...,
	)

	// 3. Check for existing pending payment.
	existingPayment, err := h.paymentRepo.FindLatestPendingByBookingID(r.Context(), req.BookingID)
	if err == nil && existingPayment != nil {
		existingPayment, err = h.payments().EnsurePaymentCurrent(r.Context(), existingPayment)
		if err != nil {
			fields := append(requestLogFields(r), zap.String("booking_id", req.BookingID), zap.Error(err))
			if existingPayment != nil {
				fields = append(fields, zap.String("payment_id", existingPayment.ID))
			}
			h.log().Warn("payment create existing pending settlement failed", fields...)
			sendError(w, "Failed to refresh payment status", "INTERNAL_ERROR", 500)
			return
		}
		if existingPayment == nil || existingPayment.Status != domain.PaymentStatusPending {
			h.log().Info("payment create existing payment no longer pending",
				append(requestLogFields(r),
					zap.String("booking_id", req.BookingID),
				)...,
			)
			sendError(w, "Payment is no longer pending. Please refresh booking status.", "PAYMENT_NOT_PENDING", http.StatusConflict)
			return
		}
		h.log().Info("payment create returning existing pending payment",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.String("payment_id", existingPayment.ID),
				zap.String("payment_status", string(existingPayment.Status)),
			)...,
		)
		// Return existing payment with a new QR code.
		qrResp, err := generateQRCode(existingPayment.OrderURL)
		if err != nil {
			h.log().Error("payment create existing payment qr generation failed",
				append(requestLogFields(r),
					zap.String("booking_id", req.BookingID),
					zap.String("payment_id", existingPayment.ID),
					zap.Error(err),
				)...,
			)
			sendError(w, "Failed to generate QR code", "INTERNAL_ERROR", 500)
			return
		}
		expireAt := formatPaymentExpireAt(existingPayment)
		ticket, err := h.createPaymentWSTicket(r.Context(), existingPayment)
		if err != nil {
			h.failPaymentWSTicket(w, r, "", nil, err)
			return
		}
		wsURL := h.buildWSSubscribeURL(r, existingPayment.ID, ticket)
		orderURL := ""
		if existingPayment.OrderURL != nil {
			orderURL = *existingPayment.OrderURL
		}
		sendSuccess(w, dto.CreatePaymentResponse{
			Payment:        mapPaymentToDTO(existingPayment),
			OrderURL:       orderURL,
			QRCode:         *qrResp,
			ZPTransToken:   existingPayment.ZPTransToken,
			ExpireAt:       expireAt,
			WsSubscribeURL: wsURL,
		}, 200)
		return
	} else if err != nil {
		h.log().Warn("payment create pending payment lookup failed",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.Error(err),
			)...,
		)
	}

	// 4. Acquire Redis slot locks for all bookings in group.
	var slots []service.SlotLockSpec
	for _, bID := range groupBookingIDs {
		b, err := h.availRepo.GetBookingByID(r.Context(), bID)
		if err != nil || b == nil {
			continue
		}
		slots = append(slots, service.SlotLockSpec{
			SubCourtID: b.SubCourtID,
			Date:       b.Date,
			StartTime:  b.StartTime,
			EndTime:    b.EndTime,
			BookingID:  bID,
		})
	}

	ok, err := h.acquireSlotLocks(r.Context(), slots)
	if err != nil || !ok {
		fields := append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.Int("slot_count", len(slots)),
			zap.Bool("slot_lock_acquired", ok),
			zap.Bool("redis_configured", h.redis != nil),
		)
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Warn("payment create slot lock failed", fields...)
		sendError(w, "Time slot is currently being processed by another user. Please try again.", "SLOT_LOCKED", 409)
		return
	}
	h.log().Info("payment create slot lock acquired",
		append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.Int("slot_count", len(slots)),
			zap.Bool("redis_configured", h.redis != nil),
		)...,
	)

	// 5. Create payment record.
	appTransID := h.zalopay.GenerateAppTransID(req.BookingID)
	payment, err := h.paymentRepo.Create(r.Context(), &req.BookingID, nil, domain.PaymentTypeBooking, appTransID, booking.TotalPrice)
	if err != nil {
		h.releaseSlotLocks(r.Context(), slots)
		h.log().Error("payment create record insert failed",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.String("app_trans_id", appTransID),
				zap.Int("amount", booking.TotalPrice),
				zap.Error(err),
			)...,
		)
		sendError(w, "Failed to create payment", "INTERNAL_ERROR", 500)
		return
	}
	h.log().Info("payment create record inserted",
		append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.String("payment_id", payment.ID),
			zap.String("app_trans_id", appTransID),
			zap.Int("amount", booking.TotalPrice),
		)...,
	)

	ticket, err := h.createPaymentWSTicket(r.Context(), payment)
	if err != nil {
		h.failPaymentWSTicket(w, r, payment.ID, slots, err)
		return
	}

	// 6. Call ZaloPay createOrder.
	embedData := zalopay.EmbedData{BookingID: req.BookingID}
	description := fmt.Sprintf("Booking %s at %s", req.BookingID[:8], booking.CourtName)
	guestName := booking.GuestName
	guestPhone := booking.GuestPhone

	zpResp, err := h.zalopay.CreateOrder(r.Context(), zalopay.CreateOrderInput{
		AppTransID:            appTransID,
		Description:           description,
		GuestName:             guestName,
		GuestPhone:            guestPhone,
		Amount:                booking.TotalPrice,
		EmbedData:             embedData,
		ExpireDurationSeconds: int64(service.PaymentValiditySeconds),
	})
	if err != nil || zpResp == nil || zpResp.ReturnCode != 1 {
		// 7. ZaloPay failed: release locks and return error.
		h.releaseSlotLocks(r.Context(), slots)
		msg := "Failed to create payment order"
		fields := append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.String("payment_id", payment.ID),
			zap.String("app_trans_id", appTransID),
		)
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		if zpResp != nil {
			msg = zpResp.ReturnMessage
			fields = append(fields,
				zap.Int("zalopay_return_code", zpResp.ReturnCode),
				zap.Int("zalopay_sub_return_code", zpResp.SubReturnCode),
				zap.String("zalopay_return_message", zpResp.ReturnMessage),
			)
		}
		h.log().Warn("payment create gateway order failed", fields...)
		sendError(w, msg, "PAYMENT_GATEWAY_ERROR", 502)
		return
	}
	h.log().Info("payment create gateway order succeeded",
		append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.String("payment_id", payment.ID),
			zap.String("app_trans_id", appTransID),
			zap.Int("zalopay_return_code", zpResp.ReturnCode),
		)...,
	)

	// Update payment with order URL and trans token.
	if err := h.paymentRepo.UpdateOrderURL(r.Context(), payment.ID, zpResp.OrderURL, zpResp.ZPTransToken); err != nil {
		h.log().Warn("payment create update order url failed",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.String("payment_id", payment.ID),
				zap.Error(err),
			)...,
		)
	}

	// Refresh payment.
	paymentID := payment.ID
	payment, err = h.paymentRepo.FindByID(r.Context(), paymentID)
	if err != nil || payment == nil {
		fields := append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.String("payment_id", paymentID),
		)
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Error("payment create refresh failed", fields...)
		sendError(w, "Failed to retrieve payment", "INTERNAL_ERROR", 500)
		return
	}

	// Generate QR code.
	qrResp, err := generateQRCode(&zpResp.OrderURL)
	if err != nil {
		h.log().Error("payment create qr generation failed",
			append(requestLogFields(r),
				zap.String("booking_id", req.BookingID),
				zap.String("payment_id", payment.ID),
				zap.Error(err),
			)...,
		)
		sendError(w, "Failed to generate QR code", "INTERNAL_ERROR", 500)
		return
	}

	expireAt := formatPaymentExpireAt(payment)
	wsURL := h.buildWSSubscribeURL(r, payment.ID, ticket)

	sendSuccess(w, dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       zpResp.OrderURL,
		QRCode:         *qrResp,
		ZPTransToken:   &zpResp.ZPTransToken,
		ExpireAt:       expireAt,
		WsSubscribeURL: wsURL,
	}, 201)
	h.log().Info("payment create response sent",
		append(requestLogFields(r),
			zap.String("booking_id", req.BookingID),
			zap.String("payment_id", payment.ID),
			zap.String("payment_status", string(payment.Status)),
			zap.String("expire_at", expireAt),
		)...,
	)
}

// Callback POST /api/payments/callback - ZaloPay webhook.
func (h *PaymentHandler) Callback(w http.ResponseWriter, r *http.Request) {
	var req dto.ZaloPayCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log().Warn("payment callback invalid request body", append(requestLogFields(r), zap.Error(err))...)
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "invalid request"}) //nolint:errcheck
		return
	}
	h.log().Info("payment callback received",
		append(requestLogFields(r),
			zap.Int("callback_type", req.Type),
			zap.Int("data_length", len(req.Data)),
			zap.Bool("has_mac", req.MAC != ""),
		)...,
	)

	// 1. Verify MAC.
	cbData, valid := h.zalopay.VerifyCallback(req.Data, req.MAC)
	if !valid || cbData == nil {
		h.log().Warn("payment callback mac verification failed",
			append(requestLogFields(r),
				zap.Int("callback_type", req.Type),
				zap.Int("data_length", len(req.Data)),
			)...,
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "mac not equal"}) //nolint:errcheck
		return
	}
	h.log().Info("payment callback verified",
		append(requestLogFields(r),
			zap.String("app_trans_id", cbData.AppTransID),
			zap.Int("amount", cbData.Amount),
			zap.Int64("zp_trans_id", cbData.ZPTransID),
		)...,
	)

	// 2. Settle through shared payment service.
	updatedPayment, err := h.payments().SettleCallback(r.Context(), cbData)
	if err != nil || updatedPayment == nil {
		fields := append(requestLogFields(r), zap.String("app_trans_id", cbData.AppTransID))
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Warn("payment callback settlement failed", fields...)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "payment not found"}) //nolint:errcheck
		return
	}
	h.log().Info("payment callback settled",
		append(requestLogFields(r),
			zap.String("payment_id", updatedPayment.ID),
			zap.String("app_trans_id", cbData.AppTransID),
			zap.String("payment_status", string(updatedPayment.Status)),
			zap.Stringp("booking_id", updatedPayment.BookingID),
			zap.Stringp("match_player_id", updatedPayment.MatchPlayerID),
		)...,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: 1, ReturnMessage: "success"}) //nolint:errcheck
	h.log().Info("payment callback response sent",
		append(requestLogFields(r),
			zap.String("payment_id", updatedPayment.ID),
			zap.String("app_trans_id", cbData.AppTransID),
		)...,
	)
}

// GetPayment GET /api/payments/:id
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.log().Info("payment detail request received", append(requestLogFields(r), zap.String("payment_id", id))...)
	payment, err := h.payments().EnsureCurrentStatus(r.Context(), id)
	if err != nil || payment == nil {
		fields := append(requestLogFields(r), zap.String("payment_id", id))
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Warn("payment detail not found", fields...)
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, mapPaymentToDTO(payment), 200)
	h.log().Info("payment detail response sent",
		append(requestLogFields(r),
			zap.String("payment_id", payment.ID),
			zap.Stringp("booking_id", payment.BookingID),
			zap.Stringp("match_player_id", payment.MatchPlayerID),
			zap.String("payment_type", string(payment.PaymentType)),
			zap.String("payment_status", string(payment.Status)),
			zap.Int("amount", payment.Amount),
		)...,
	)
}

func (h *PaymentHandler) PaymentStatusNotification(ctx context.Context, paymentID string) (*ws.PaymentNotification, error) {
	payment, err := h.payments().EnsureCurrentStatus(ctx, paymentID)
	if err != nil || payment == nil {
		return nil, err
	}
	notification := h.payments().Notification(payment, "")
	return &notification, nil
}

// CancelPayment POST /api/payments/:id/cancel
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.log().Info("payment cancel request received", append(requestLogFields(r), zap.String("payment_id", id))...)
	payment, err := h.payments().CancelPayment(r.Context(), id, "Payment was cancelled")
	if err != nil || payment == nil {
		fields := append(requestLogFields(r), zap.String("payment_id", id))
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		h.log().Warn("payment cancel payment not found", fields...)
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}

	sendSuccess(w, mapPaymentToDTO(payment), 200)
	h.log().Info("payment cancel response sent",
		append(requestLogFields(r),
			zap.String("payment_id", payment.ID),
			zap.String("payment_status", string(payment.Status)),
			zap.Stringp("booking_id", payment.BookingID),
			zap.Stringp("match_player_id", payment.MatchPlayerID),
		)...,
	)
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
	if req.MatchPlayerID == "" {
		sendError(w, "matchPlayerId is required", "BAD_REQUEST", 400)
		return
	}

	// Validate match.
	match, err := h.matchRepo.FindByID(r.Context(), matchID)
	if err != nil || match == nil {
		sendError(w, "Match not found", "NOT_FOUND", 404)
		return
	}

	// Validate player belongs to this user.
	player, err := h.matchRepo.FindPlayerByID(r.Context(), req.MatchPlayerID)
	if err != nil || player == nil {
		sendError(w, "Match player not found", "NOT_FOUND", 404)
		return
	}
	if player.UserID != user.ID {
		sendError(w, "This match player does not belong to you", "FORBIDDEN", 403)
		return
	}
	if player.MatchID != matchID {
		sendError(w, "Match player does not belong to this match", "BAD_REQUEST", 400)
		return
	}
	if player.Status != domain.MatchPlayerStatusPendingPayment {
		sendError(w, "Match player is not in pending payment status", "INVALID_STATUS", 400)
		return
	}

	if match.Price == 0 {
		sendError(w, "This match does not require payment", "BAD_REQUEST", 400)
		return
	}

	// Generate app_trans_id and create payment.
	appTransID := h.zalopay.GenerateAppTransID(req.MatchPlayerID)
	payment, err := h.paymentRepo.Create(r.Context(), nil, &req.MatchPlayerID, domain.PaymentTypeMatchJoin, appTransID, match.Price)
	if err != nil {
		sendError(w, "Failed to create payment", "INTERNAL_ERROR", 500)
		return
	}
	ticket, err := h.createPaymentWSTicket(r.Context(), payment)
	if err != nil {
		h.failPaymentWSTicket(w, r, payment.ID, nil, err)
		return
	}

	// Build description.
	playerName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
	description := fmt.Sprintf("Match join fee for %s", matchID[:8])

	embedData := zalopay.EmbedData{MatchPlayerID: req.MatchPlayerID}
	zpResp, err := h.zalopay.CreateOrder(r.Context(), zalopay.CreateOrderInput{
		AppTransID:            appTransID,
		Description:           description,
		GuestName:             playerName,
		Amount:                match.Price,
		EmbedData:             embedData,
		ExpireDurationSeconds: int64(service.PaymentValiditySeconds),
	})
	if err != nil || zpResp == nil || zpResp.ReturnCode != 1 {
		msg := "Failed to create payment order"
		if zpResp != nil {
			msg = zpResp.ReturnMessage
		}
		sendError(w, msg, "PAYMENT_GATEWAY_ERROR", 502)
		return
	}

	h.paymentRepo.UpdateOrderURL(r.Context(), payment.ID, zpResp.OrderURL, zpResp.ZPTransToken) //nolint:errcheck

	payment, err = h.paymentRepo.FindByID(r.Context(), payment.ID)
	if err != nil || payment == nil {
		sendError(w, "Failed to retrieve payment", "INTERNAL_ERROR", 500)
		return
	}

	qrResp, err := generateQRCode(&zpResp.OrderURL)
	if err != nil {
		sendError(w, "Failed to generate QR code", "INTERNAL_ERROR", 500)
		return
	}

	expireAt := formatPaymentExpireAt(payment)
	wsURL := h.buildWSSubscribeURL(r, payment.ID, ticket)

	sendSuccess(w, dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       zpResp.OrderURL,
		QRCode:         *qrResp,
		ZPTransToken:   &zpResp.ZPTransToken,
		ExpireAt:       expireAt,
		WsSubscribeURL: wsURL,
	}, 201)
}

// GetMatchPaymentStatus GET /api/matches/:matchId/payment/:paymentId/status
func (h *PaymentHandler) GetMatchPaymentStatus(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentId")
	payment, err := h.payments().EnsureCurrentStatus(r.Context(), paymentID)
	if err != nil || payment == nil {
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}

	isExpired := payment.Status == domain.PaymentStatusExpired

	sendSuccess(w, dto.PaymentStatusResponse{
		Payment:   mapPaymentToDTO(payment),
		IsExpired: isExpired,
	}, 200)
}

// These helper functions need to be moved to util folder for easier management
// ==================== Helpers ====================

func mapPaymentToDTO(p *domain.Payment) dto.PaymentResponse {
	return dto.PaymentResponse{
		ID:            p.ID,
		BookingID:     p.BookingID,
		MatchPlayerID: p.MatchPlayerID,
		PaymentType:   string(p.PaymentType),
		AppTransID:    p.AppTransID,
		ZPTransID:     p.ZPTransID,
		Amount:        p.Amount,
		Status:        string(p.Status),
		OrderURL:      p.OrderURL,
		CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:     p.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}

func formatPaymentExpireAt(p *domain.Payment) string {
	return p.CreatedAt.Add(service.PaymentValidity).Format("2006-01-02T15:04:05.000Z")
}

func generateQRCode(orderURL *string) (*dto.QRCodeData, error) {
	url := ""
	if orderURL != nil {
		url = *orderURL
	}
	if url == "" {
		return &dto.QRCodeData{}, nil
	}

	// Generate QR code as PNG bytes.
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}

	// Base64 with data URI prefix.
	rawB64 := base64.StdEncoding.EncodeToString(png)
	dataURI := "data:image/png;base64," + rawB64

	return &dto.QRCodeData{
		Base64:    dataURI,
		RawBase64: rawB64,
	}, nil
}

func (h *PaymentHandler) buildWSSubscribeURL(r *http.Request, paymentID, ticket string) string {
	scheme := "ws"
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if proto == "https" || r.TLS != nil || (proto == "" && h.nodeEnv == "production") {
		scheme = "wss"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		host = fmt.Sprintf("localhost:%d", h.port)
	}
	q := url.Values{}
	q.Set("paymentId", paymentID)
	q.Set("ticket", ticket)
	return fmt.Sprintf("%s://%s/ws/payments?%s", scheme, host, q.Encode())
}

// sanitizeForKey returns a key-safe version of a string (removes hyphens).
func sanitizeForKey(s string) string {
	return strings.ReplaceAll(s, "-", "")
}

var _ = sanitizeForKey // suppress unused warning

// CancelPaymentByID cancels a payment by ID without HTTP context (used by WS auto-cancel).
func (h *PaymentHandler) CancelPaymentByID(ctx context.Context, paymentID string) {
	_, _ = h.payments().CancelPayment(ctx, paymentID, "Payment cancelled due to client disconnect")
}

// GetBookingPayment GET /api/bookings/:id/payment — returns the latest payment for a booking
func (h *PaymentHandler) GetBookingPayment(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "id")
	h.log().Info("booking payment lookup request received", append(requestLogFields(r), zap.String("booking_id", bookingID))...)
	p, err := h.paymentRepo.FindLatestPendingByBookingID(r.Context(), bookingID)
	if err != nil {
		h.log().Error("booking payment lookup failed",
			append(requestLogFields(r),
				zap.String("booking_id", bookingID),
				zap.Error(err),
			)...,
		)
		sendError(w, "Failed to get payment", "INTERNAL_ERROR", 500)
		return
	}
	if p == nil {
		h.log().Info("booking payment lookup no pending payment", append(requestLogFields(r), zap.String("booking_id", bookingID))...)
		sendError(w, "No payment found for this booking", "NOT_FOUND", 404)
		return
	}
	p, err = h.payments().EnsurePaymentCurrent(r.Context(), p)
	if err != nil {
		h.log().Error("booking payment status refresh failed",
			append(requestLogFields(r),
				zap.String("booking_id", bookingID),
				zap.String("payment_id", p.ID),
				zap.Error(err),
			)...,
		)
		sendError(w, "Failed to refresh payment status", "INTERNAL_ERROR", 500)
		return
	}
	sendSuccess(w, mapPaymentToDTO(p), 200)
	h.log().Info("booking payment lookup response sent",
		append(requestLogFields(r),
			zap.String("booking_id", bookingID),
			zap.String("payment_id", p.ID),
			zap.String("payment_status", string(p.Status)),
			zap.Int("amount", p.Amount),
		)...,
	)
}

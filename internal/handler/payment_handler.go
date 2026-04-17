package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
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
)

type PaymentHandler struct {
	paymentRepo *repository.PaymentRepository
	availRepo   *repository.AvailabilityRepository
	matchRepo   *repository.MatchRepository
	redis       *service.RedisService
	zalopay     *zalopay.Client
	hub         *ws.Hub
	slotLockTTL int
	port        int
	nodeEnv     string
}

func NewPaymentHandler(
	pr *repository.PaymentRepository,
	ar *repository.AvailabilityRepository,
	mr *repository.MatchRepository,
	rs *service.RedisService,
	zp *zalopay.Client,
	hub *ws.Hub,
	slotLockTTL, port int,
	nodeEnv string,
) *PaymentHandler {
	return &PaymentHandler{
		paymentRepo: pr,
		availRepo:   ar,
		matchRepo:   mr,
		redis:       rs,
		zalopay:     zp,
		hub:         hub,
		slotLockTTL: slotLockTTL,
		port:        port,
		nodeEnv:     nodeEnv,
	}
}

// POST /api/payments/create
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	_ = middleware.UserFromContext(r.Context()) // auth enforced by middleware; user context not needed here

	var req dto.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid request body", "BAD_REQUEST", 400)
		return
	}
	if req.BookingID == "" {
		sendError(w, "bookingId is required", "BAD_REQUEST", 400)
		return
	}

	// 1. Get booking and validate.
	booking, err := h.availRepo.GetBookingByID(r.Context(), req.BookingID)
	if err != nil || booking == nil {
		sendError(w, "Booking not found", "NOT_FOUND", 404)
		return
	}
	if booking.Status != "pending" {
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
		}
	}
	if len(groupBookingIDs) == 0 {
		groupBookingIDs = []string{req.BookingID}
	}

	// 3. Check for existing pending payment.
	existingPayment, err := h.paymentRepo.FindLatestPendingByBookingID(r.Context(), req.BookingID)
	if err == nil && existingPayment != nil {
		// Return existing payment with a new QR code.
		qrResp, err := generateQRCode(existingPayment.OrderURL)
		if err != nil {
			sendError(w, "Failed to generate QR code", "INTERNAL_ERROR", 500)
			return
		}
		expireAt := existingPayment.CreatedAt.Add(time.Duration(h.slotLockTTL) * time.Second).Format("2006-01-02T15:04:05.000Z")
		wsURL := h.buildWSSubscribeURL(existingPayment.ID)
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

	if len(slots) > 0 {
		ok, err := h.redis.AcquireSlotLocks(r.Context(), slots)
		if err != nil || !ok {
			sendError(w, "Time slot is currently being processed by another user. Please try again.", "SLOT_LOCKED", 409)
			return
		}
	}

	// 5. Create payment record.
	appTransID := h.zalopay.GenerateAppTransID(req.BookingID)
	payment, err := h.paymentRepo.Create(r.Context(), &req.BookingID, nil, domain.PaymentTypeBooking, appTransID, booking.TotalPrice)
	if err != nil {
		h.redis.ReleaseSlotLocks(r.Context(), slots)
		sendError(w, "Failed to create payment", "INTERNAL_ERROR", 500)
		return
	}

	// 6. Call ZaloPay createOrder.
	embedData := zalopay.EmbedData{BookingID: req.BookingID}
	description := fmt.Sprintf("Booking %s at %s", req.BookingID[:8], booking.CourtName)
	guestName := booking.GuestName
	guestPhone := booking.GuestPhone

	zpResp, err := h.zalopay.CreateOrder(
		req.BookingID, appTransID, description,
		guestName, guestPhone,
		booking.TotalPrice, embedData,
	)
	if err != nil || zpResp == nil || zpResp.ReturnCode != 1 {
		// 7. ZaloPay failed: release locks and return error.
		h.redis.ReleaseSlotLocks(r.Context(), slots)
		msg := "Failed to create payment order"
		if zpResp != nil {
			msg = zpResp.ReturnMessage
		}
		sendError(w, msg, "PAYMENT_GATEWAY_ERROR", 502)
		return
	}

	// Update payment with order URL and trans token.
	h.paymentRepo.UpdateOrderURL(r.Context(), payment.ID, zpResp.OrderURL, zpResp.ZPTransToken) //nolint:errcheck

	// Refresh payment.
	payment, err = h.paymentRepo.FindByID(r.Context(), payment.ID)
	if err != nil || payment == nil {
		sendError(w, "Failed to retrieve payment", "INTERNAL_ERROR", 500)
		return
	}

	// Generate QR code.
	qrResp, err := generateQRCode(&zpResp.OrderURL)
	if err != nil {
		sendError(w, "Failed to generate QR code", "INTERNAL_ERROR", 500)
		return
	}

	expireAt := payment.CreatedAt.Add(time.Duration(h.slotLockTTL) * time.Second).Format("2006-01-02T15:04:05.000Z")
	wsURL := h.buildWSSubscribeURL(payment.ID)

	sendSuccess(w, dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       zpResp.OrderURL,
		QRCode:         *qrResp,
		ZPTransToken:   &zpResp.ZPTransToken,
		ExpireAt:       expireAt,
		WsSubscribeURL: wsURL,
	}, 201)
}

// POST /api/payments/callback - ZaloPay webhook.
func (h *PaymentHandler) Callback(w http.ResponseWriter, r *http.Request) {
	var req dto.ZaloPayCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "invalid request"}) //nolint:errcheck
		return
	}

	// 1. Verify MAC.
	cbData, valid := h.zalopay.VerifyCallback(req.Data, req.MAC)
	if !valid || cbData == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "mac not equal"}) //nolint:errcheck
		return
	}

	// 2. Parse app_trans_id and find payment.
	payment, err := h.paymentRepo.FindByAppTransID(r.Context(), cbData.AppTransID)
	if err != nil || payment == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "payment not found"}) //nolint:errcheck
		return
	}

	if payment.Status != domain.PaymentStatusPending {
		// Already processed.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: 1, ReturnMessage: "success"}) //nolint:errcheck
		return
	}

	// 3. Update payment status to success.
	zpTransID := fmt.Sprintf("%d", cbData.ZPTransID)
	rawData, _ := json.Marshal(cbData)
	updatedPayment, err := h.paymentRepo.UpdateStatusByAppTransID(
		r.Context(), cbData.AppTransID,
		domain.PaymentStatusSuccess,
		&zpTransID,
		json.RawMessage(rawData),
	)
	if err != nil || updatedPayment == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: -1, ReturnMessage: "update failed"}) //nolint:errcheck
		return
	}

	// 4. Update booking status to confirmed and release slot locks.
	if updatedPayment.BookingID != nil {
		booking, err := h.availRepo.GetBookingByID(r.Context(), *updatedPayment.BookingID)
		if err == nil && booking != nil {
			h.availRepo.UpdateBookingStatus(r.Context(), *updatedPayment.BookingID, "confirmed") //nolint:errcheck

			// Release slot locks.
			slots := []service.SlotLockSpec{{
				SubCourtID: booking.SubCourtID,
				Date:       booking.Date,
				StartTime:  booking.StartTime,
				EndTime:    booking.EndTime,
				BookingID:  *updatedPayment.BookingID,
			}}
			h.redis.ReleaseSlotLocks(r.Context(), slots)

			// Release group booking locks if any.
			if booking.GroupID != nil {
				groupBookings, err := h.availRepo.GetBookingsByGroupID(r.Context(), *booking.GroupID)
				if err == nil {
					for _, gb := range groupBookings {
						if gb.ID != *updatedPayment.BookingID {
							h.availRepo.UpdateBookingStatus(r.Context(), gb.ID, "confirmed") //nolint:errcheck
							h.redis.ReleaseSlotLocks(r.Context(), []service.SlotLockSpec{{
								SubCourtID: gb.SubCourtID,
								Date:       gb.StartTime, // date comes from booking row
								StartTime:  gb.StartTime,
								EndTime:    gb.EndTime,
								BookingID:  gb.ID,
							}})
						}
					}
				}
			}
		}
	}

	// 5. Update match player status if this is a match payment.
	if updatedPayment.MatchPlayerID != nil {
		player, err := h.matchRepo.FindPlayerByID(r.Context(), *updatedPayment.MatchPlayerID)
		if err == nil && player != nil {
			pos, _ := h.matchRepo.GetNextPosition(r.Context(), player.MatchID)
			h.matchRepo.UpdatePlayerStatus(r.Context(), player.ID, domain.MatchPlayerStatusAccepted, &pos) //nolint:errcheck

			// Check if match is now full.
			match, _ := h.matchRepo.FindByID(r.Context(), player.MatchID)
			if match != nil {
				acceptedCount, _ := h.matchRepo.CountAcceptedPlayers(r.Context(), player.MatchID)
				if acceptedCount >= match.SlotsNeeded {
					h.matchRepo.UpdateStatus(r.Context(), player.MatchID, domain.MatchStatusFull) //nolint:errcheck
				}
			}
		}
	}

	// 6. Notify WebSocket subscribers.
	h.hub.NotifyPaymentStatus(ws.PaymentNotification{
		Type:          "payment_success",
		PaymentID:     updatedPayment.ID,
		Status:        string(domain.PaymentStatusSuccess),
		BookingID:     updatedPayment.BookingID,
		MatchPlayerID: updatedPayment.MatchPlayerID,
		ZPTransID:     &zpTransID,
		Message:       "Payment successful",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.ZaloPayCallbackResponse{ReturnCode: 1, ReturnMessage: "success"}) //nolint:errcheck
}

// GET /api/payments/:id
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	payment, err := h.paymentRepo.FindByID(r.Context(), id)
	if err != nil || payment == nil {
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, mapPaymentToDTO(payment), 200)
}

// GET /api/payments/:id/status
func (h *PaymentHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	payment, err := h.paymentRepo.FindByID(r.Context(), id)
	if err != nil || payment == nil {
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}

	isExpired := false
	if payment.Status == domain.PaymentStatusPending {
		expireAt := payment.CreatedAt.Add(time.Duration(h.slotLockTTL) * time.Second)
		if time.Now().After(expireAt) {
			isExpired = true
		}
	}

	sendSuccess(w, dto.PaymentStatusResponse{
		Payment:   mapPaymentToDTO(payment),
		IsExpired: isExpired,
	}, 200)
}

// POST /api/payments/:id/cancel
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	payment, err := h.paymentRepo.FindByID(r.Context(), id)
	if err != nil || payment == nil {
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}

	if payment.Status != domain.PaymentStatusPending {
		// Already settled - just return ok.
		sendSuccess(w, mapPaymentToDTO(payment), 200)
		return
	}

	// Update payment status to failed.
	h.paymentRepo.UpdateStatus(r.Context(), id, domain.PaymentStatusFailed, nil, nil) //nolint:errcheck

	// Update booking status to cancelled and release locks.
	if payment.BookingID != nil {
		booking, err := h.availRepo.GetBookingByID(r.Context(), *payment.BookingID)
		if err == nil && booking != nil {
			h.availRepo.UpdateBookingStatus(r.Context(), *payment.BookingID, "cancelled") //nolint:errcheck
			h.redis.ReleaseSlotLocks(r.Context(), []service.SlotLockSpec{{
				SubCourtID: booking.SubCourtID,
				Date:       booking.Date,
				StartTime:  booking.StartTime,
				EndTime:    booking.EndTime,
				BookingID:  *payment.BookingID,
			}})
		}
	}

	// Notify WebSocket.
	h.hub.NotifyPaymentStatus(ws.PaymentNotification{
		Type:          "payment_cancelled",
		PaymentID:     payment.ID,
		Status:        string(domain.PaymentStatusFailed),
		BookingID:     payment.BookingID,
		MatchPlayerID: payment.MatchPlayerID,
		Message:       "Payment was cancelled",
	})

	refreshed, _ := h.paymentRepo.FindByID(r.Context(), id)
	if refreshed == nil {
		refreshed = payment
	}
	sendSuccess(w, mapPaymentToDTO(refreshed), 200)
}

// POST /api/matches/:matchId/payment - Create payment for match join.
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

	// Build description.
	playerName := buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername)
	description := fmt.Sprintf("Match join fee for %s", matchID[:8])

	embedData := zalopay.EmbedData{MatchPlayerID: req.MatchPlayerID}
	zpResp, err := h.zalopay.CreateOrder(
		req.MatchPlayerID, appTransID, description,
		playerName, "",
		match.Price, embedData,
	)
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

	expireAt := payment.CreatedAt.Add(time.Duration(h.slotLockTTL) * time.Second).Format("2006-01-02T15:04:05.000Z")
	wsURL := h.buildWSSubscribeURL(payment.ID)

	sendSuccess(w, dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       zpResp.OrderURL,
		QRCode:         *qrResp,
		ZPTransToken:   &zpResp.ZPTransToken,
		ExpireAt:       expireAt,
		WsSubscribeURL: wsURL,
	}, 201)
}

// GET /api/matches/:matchId/payment/:paymentId/status
func (h *PaymentHandler) GetMatchPaymentStatus(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentId")
	payment, err := h.paymentRepo.FindByID(r.Context(), paymentID)
	if err != nil || payment == nil {
		sendError(w, "Payment not found", "NOT_FOUND", 404)
		return
	}

	isExpired := false
	if payment.Status == domain.PaymentStatusPending {
		expireAt := payment.CreatedAt.Add(time.Duration(h.slotLockTTL) * time.Second)
		if time.Now().After(expireAt) {
			isExpired = true
		}
	}

	sendSuccess(w, dto.PaymentStatusResponse{
		Payment:   mapPaymentToDTO(payment),
		IsExpired: isExpired,
	}, 200)
}

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

func (h *PaymentHandler) buildWSSubscribeURL(paymentID string) string {
	scheme := "ws"
	if h.nodeEnv == "production" {
		scheme = "wss"
	}
	host := fmt.Sprintf("localhost:%d", h.port)
	return fmt.Sprintf("%s://%s/ws/payments?paymentId=%s", scheme, host, paymentID)
}

// sanitizeForKey returns a key-safe version of a string (removes hyphens).
func sanitizeForKey(s string) string {
	return strings.ReplaceAll(s, "-", "")
}

var _ = sanitizeForKey // suppress unused warning

// CancelPaymentByID cancels a payment by ID without HTTP context (used by WS auto-cancel).
func (h *PaymentHandler) CancelPaymentByID(ctx context.Context, paymentID string) {
	p, err := h.paymentRepo.FindByID(ctx, paymentID)
	if err != nil || p == nil || p.Status != domain.PaymentStatusPending {
		return
	}
	_ = h.paymentRepo.UpdateStatus(ctx, paymentID, domain.PaymentStatusFailed, nil, nil)
	if p.BookingID != nil {
		_ = h.availRepo.UpdateBookingStatus(ctx, *p.BookingID, "cancelled")
	}
	h.hub.NotifyPaymentStatus(ws.PaymentNotification{
		Type:      "payment_status",
		PaymentID: paymentID,
		Status:    "cancelled",
		BookingID: p.BookingID,
		Message:   "Payment cancelled due to client disconnect",
	})
}

// GET /api/bookings/:id/payment — returns the latest payment for a booking
func (h *PaymentHandler) GetBookingPayment(w http.ResponseWriter, r *http.Request) {
	bookingID := chi.URLParam(r, "id")
	p, err := h.paymentRepo.FindLatestPendingByBookingID(r.Context(), bookingID)
	if err != nil {
		sendError(w, "Failed to get payment", "INTERNAL_ERROR", 500)
		return
	}
	if p == nil {
		sendError(w, "No payment found for this booking", "NOT_FOUND", 404)
		return
	}
	sendSuccess(w, mapPaymentToDTO(p), 200)
}

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	ws "github.com/smatch/badminton-backend/internal/websocket"
	zalopay "github.com/smatch/badminton-backend/platform/zalopay"
	"go.uber.org/zap"
)

// paymentErrorCode wraps a known handler-level error code so the HTTP layer can
// reproduce the exact code/status that the previous inline handler returned.
type paymentErrorCode struct {
	code    string
	message string
	status  int
	err     error
}

func (e *paymentErrorCode) Error() string { return e.message }
func (e *paymentErrorCode) Unwrap() error { return e.err }

func paymentErr(code, message string, status int) error {
	return &paymentErrorCode{code: code, message: message, status: status}
}

// AppErrorFromPaymentErr maps an internal payment error to *domain.AppError so
// the HTTP layer's sendAppError reproduces the right code/status.
func AppErrorFromPaymentErr(err error) *domain.AppError {
	return appErrorFromPaymentErr(err)
}

// appErrorFromPaymentErr maps an internal payment error to *domain.AppError so
// the HTTP layer's sendAppError reproduces the right code/status.
func appErrorFromPaymentErr(err error) *domain.AppError {
	if err == nil {
		return nil
	}
	var pe *paymentErrorCode
	if asCode(err, &pe) {
		return &domain.AppError{Code: pe.code, Message: pe.message, Status: pe.status, Err: pe.err}
	}
	var appErr *domain.AppError
	if asAppErr(err, &appErr) {
		return appErr
	}
	return &domain.AppError{Code: "INTERNAL_ERROR", Message: "Internal server error", Status: 500, Err: err}
}

// indirection to avoid importing "errors" cycle declarations; thin wrappers.
func asCode(err error, dst **paymentErrorCode) bool {
	if e, ok := err.(*paymentErrorCode); ok {
		*dst = e
		return true
	}
	return false
}

func asAppErr(err error, dst **domain.AppError) bool {
	if e, ok := err.(*domain.AppError); ok {
		*dst = e
		return true
	}
	return false
}

// WSURLParams carries request-derived scheme/host plus server config fallbacks
// for building the payment WebSocket subscribe URL.
type WSURLParams struct {
	ForwardedProto string // X-Forwarded-Proto header
	ForwardedHost  string // X-Forwarded-Host header
	Host           string // r.Host
	TLS            bool   // r.TLS != nil
	Port           int    // server port (fallback)
	NodeEnv        string // server env (production → wss)
}

// ==================== CreatePayment ====================

// CreatePayment orchestrates a booking payment: validates the booking, resolves
// group slots, acquires Redis slot locks, creates the payment record, calls the
// ZaloPay gateway, generates a QR code and WebSocket subscribe URL.
func (s *PaymentService) CreatePayment(ctx context.Context, bookingID string, ws WSURLParams) (*dto.CreatePaymentResponse, error) {
	if bookingID == "" {
		return nil, paymentErr("BAD_REQUEST", "bookingId is required", 400)
	}

	booking, err := s.availRepo.GetBookingByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, paymentErr("NOT_FOUND", "Booking not found", 404)
	}
	if booking.Status != "pending" {
		s.logger.Warn("payment create booking invalid status",
			zap.String("booking_id", bookingID),
			zap.String("booking_status", booking.Status))
		return nil, paymentErr("INVALID_STATUS", "Booking is not in pending status", 400)
	}
	s.logger.Info("payment create booking loaded",
		zap.String("booking_id", booking.ID),
		zap.Int("amount", booking.TotalPrice))

	// Resolve grouped bookings.
	var groupBookingIDs []string
	if booking.GroupID != nil {
		groupBookings, gErr := s.availRepo.GetBookingsByGroupID(ctx, *booking.GroupID)
		if gErr == nil {
			for _, gb := range groupBookings {
				groupBookingIDs = append(groupBookingIDs, gb.ID)
			}
		}
	}
	if len(groupBookingIDs) == 0 {
		groupBookingIDs = []string{bookingID}
	}

	// Check for existing pending payment (may return an existing payment to reuse).
	existing, err := s.paymentRepo.FindLatestPendingByBookingID(ctx, bookingID)
	if err == nil && existing != nil {
		refreshed, err := s.EnsurePaymentCurrent(ctx, existing)
		if err != nil {
			return nil, paymentErr("INTERNAL_ERROR", "Failed to refresh payment status", 500)
		}
		if refreshed == nil || refreshed.Status != domain.PaymentStatusPending {
			return nil, paymentErr("PAYMENT_NOT_PENDING", "Payment is no longer pending. Please refresh booking status.", 409)
		}
		resp, err := s.buildCreatePaymentResponse(ctx, refreshed, nil, ws)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Acquire Redis slot locks for every booking in the group.
	slots := s.collectSlotLocks(ctx, groupBookingIDs)
	if ok, lErr := s.acquireSlotLocks(ctx, slots); lErr != nil || !ok {
		return nil, paymentErr("SLOT_LOCKED", "Time slot is currently being processed by another user. Please try again.", 409)
	}

	// Create payment record.
	appTransID := s.zalo.GenerateAppTransID(bookingID)
	payment, err := s.paymentRepo.Create(ctx, &bookingID, nil, domain.PaymentTypeBooking, appTransID, booking.TotalPrice)
	if err != nil {
		s.releaseSlotLocks(ctx, slots)
		s.logger.Error("payment create record insert failed",
			zap.String("booking_id", bookingID), zap.String("app_trans_id", appTransID), zap.Error(err))
		return nil, paymentErr("INTERNAL_ERROR", "Failed to create payment", 500)
	}

	ticket, err := s.createPaymentWSTicket(ctx, payment)
	if err != nil {
		s.failPaymentWSTicket(ctx, payment.ID, slots, err)
		return nil, paymentErr("PAYMENT_WS_TICKET_ERROR", "Payment notification channel unavailable. Please try again.", 503)
	}

	// Build gateway order.
	description := fmt.Sprintf("Booking %s at %s", bookingID[:8], booking.CourtName)
	zpResp, err := s.zalo.CreateOrder(ctx, zalopay.CreateOrderInput{
		AppTransID:            appTransID,
		Description:           description,
		GuestName:             booking.GuestName,
		GuestPhone:            booking.GuestPhone,
		Amount:                booking.TotalPrice,
		EmbedData:             zalopay.EmbedData{BookingID: bookingID},
		ExpireDurationSeconds: int64(PaymentValiditySeconds),
	})
	if err != nil || zpResp == nil || zpResp.ReturnCode != 1 {
		s.releaseSlotLocks(ctx, slots)
		msg := "Failed to create payment order"
		if zpResp != nil && zpResp.ReturnMessage != "" {
			msg = zpResp.ReturnMessage
		}
		s.logger.Warn("payment create gateway order failed",
			zap.String("booking_id", bookingID), zap.String("payment_id", payment.ID), zap.Error(err))
		return nil, paymentErr("PAYMENT_GATEWAY_ERROR", msg, 502)
	}

	_ = s.paymentRepo.UpdateOrderURL(ctx, payment.ID, zpResp.OrderURL, zpResp.ZPTransToken)

	// Refresh payment with the order URL.
	payment, err = s.paymentRepo.FindByID(ctx, payment.ID)
	if err != nil || payment == nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to retrieve payment", 500)
	}

	return s.buildCreatePaymentResponseWith(ctx, payment, &zpResp.OrderURL, &zpResp.ZPTransToken, ticket, slots, ws)
}

func (s *PaymentService) collectSlotLocks(ctx context.Context, bookingIDs []string) []SlotLockSpec {
	var slots []SlotLockSpec
	for _, bID := range bookingIDs {
		b, err := s.availRepo.GetBookingByID(ctx, bID)
		if err != nil || b == nil {
			continue
		}
		slots = append(slots, SlotLockSpec{
			SubCourtID: b.SubCourtID,
			Date:       b.Date,
			StartTime:  b.StartTime,
			EndTime:    b.EndTime,
			BookingID:  bID,
		})
	}
	return slots
}

func (s *PaymentService) buildCreatePaymentResponse(ctx context.Context, payment *domain.Payment, _ interface{}, ws WSURLParams) (*dto.CreatePaymentResponse, error) {
	orderURL := ""
	if payment.OrderURL != nil {
		orderURL = *payment.OrderURL
	}
	qrResp, err := generateQRCode(payment.OrderURL)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to generate QR code", 500)
	}
	ticket, err := s.createPaymentWSTicket(ctx, payment)
	if err != nil {
		s.failPaymentWSTicket(ctx, "", nil, err)
		return nil, paymentErr("PAYMENT_WS_TICKET_ERROR", "Payment notification channel unavailable. Please try again.", 503)
	}
	wsURL := BuildWSSubscribeURL(payment.ID, ticket, ws)
	return &dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       orderURL,
		QRCode:         *qrResp,
		ZPTransToken:   payment.ZPTransToken,
		ExpireAt:       FormatPaymentExpireAt(payment),
		WsSubscribeURL: wsURL,
	}, nil
}

func (s *PaymentService) buildCreatePaymentResponseWith(ctx context.Context, payment *domain.Payment, orderURL *string, zpToken *string, ticket string, slots []SlotLockSpec, ws WSURLParams) (*dto.CreatePaymentResponse, error) {
	qrResp, err := generateQRCode(orderURL)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to generate QR code", 500)
	}
	wsURL := BuildWSSubscribeURL(payment.ID, ticket, ws)
	url := ""
	if orderURL != nil {
		url = *orderURL
	}
	return &dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       url,
		QRCode:         *qrResp,
		ZPTransToken:   zpToken,
		ExpireAt:       FormatPaymentExpireAt(payment),
		WsSubscribeURL: wsURL,
	}, nil
}

// ==================== Callback ====================

// CallbackResult is the ZaloPay-facing callback response payload.
type CallbackResult struct {
	ReturnCode    int
	ReturnMessage string
}

// Callback verifies the ZaloPay callback MAC and settles the payment.
func (s *PaymentService) Callback(ctx context.Context, data, mac string) CallbackResult {
	cbData, valid := s.zalo.VerifyCallback(data, mac)
	if !valid || cbData == nil {
		return CallbackResult{ReturnCode: -1, ReturnMessage: "mac not equal"}
	}

	updated, err := s.SettleCallback(ctx, cbData)
	if err != nil || updated == nil {
		s.logger.Warn("payment callback settlement failed",
			zap.String("app_trans_id", cbData.AppTransID), zap.Error(err))
		return CallbackResult{ReturnCode: -1, ReturnMessage: "payment not found"}
	}
	return CallbackResult{ReturnCode: 1, ReturnMessage: "success"}
}

// ==================== GetPayment / CancelPayment / Booking lookup ====================

// GetPayment returns the current DTO for a payment.
func (s *PaymentService) GetPayment(ctx context.Context, id string) (*dto.PaymentResponse, error) {
	payment, err := s.EnsureCurrentStatus(ctx, id)
	if err != nil || payment == nil {
		return nil, paymentErr("NOT_FOUND", "Payment not found", 404)
	}
	resp := mapPaymentToDTO(payment)
	return &resp, nil
}

// CancelPaymentByID cancels a payment without HTTP context (used by WS auto-cancel).
func (s *PaymentService) CancelPaymentByID(ctx context.Context, paymentID string) {
	_, _ = s.CancelPayment(ctx, paymentID, "Payment cancelled due to client disconnect")
}

// PaymentStatusNotification returns the WS notification payload for a payment.
func (s *PaymentService) PaymentStatusNotification(ctx context.Context, paymentID string) (*ws.PaymentNotification, error) {
	payment, err := s.EnsureCurrentStatus(ctx, paymentID)
	if err != nil || payment == nil {
		return nil, err
	}
	n := s.Notification(payment, "")
	return &n, nil
}

// GetBookingPayment returns the latest pending payment DTO for a booking.
func (s *PaymentService) GetBookingPayment(ctx context.Context, bookingID string) (*dto.PaymentResponse, error) {
	p, err := s.paymentRepo.FindLatestPendingByBookingID(ctx, bookingID)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to get payment", 500)
	}
	if p == nil {
		return nil, paymentErr("NOT_FOUND", "No payment found for this booking", 404)
	}
	refreshed, err := s.EnsurePaymentCurrent(ctx, p)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to refresh payment status", 500)
	}
	resp := mapPaymentToDTO(refreshed)
	return &resp, nil
}

// ==================== Match payment ====================

// CreateMatchPayment orchestrates a match-join payment: validates match/player
// ownership, creates the payment record, calls the gateway and builds DTO+QR+WS.
func (s *PaymentService) CreateMatchPayment(ctx context.Context, matchID, requestingUserID, matchPlayerID string, ws WSURLParams) (*dto.CreatePaymentResponse, error) {
	if matchPlayerID == "" {
		return nil, paymentErr("BAD_REQUEST", "matchPlayerId is required", 400)
	}

	match, err := s.matchRepo.FindByID(ctx, matchID)
	if err != nil || match == nil {
		return nil, paymentErr("NOT_FOUND", "Match not found", 404)
	}

	player, err := s.matchRepo.FindPlayerByID(ctx, matchPlayerID)
	if err != nil || player == nil {
		return nil, paymentErr("NOT_FOUND", "Match player not found", 404)
	}
	if player.UserID != requestingUserID {
		return nil, paymentErr("FORBIDDEN", "This match player does not belong to you", 403)
	}
	if player.MatchID != matchID {
		return nil, paymentErr("BAD_REQUEST", "Match player does not belong to this match", 400)
	}
	if player.Status != domain.MatchPlayerStatusPendingPayment {
		return nil, paymentErr("INVALID_STATUS", "Match player is not in pending payment status", 400)
	}
	if match.Price == 0 {
		return nil, paymentErr("BAD_REQUEST", "This match does not require payment", 400)
	}

	appTransID := s.zalo.GenerateAppTransID(matchPlayerID)
	payment, err := s.paymentRepo.Create(ctx, nil, &matchPlayerID, domain.PaymentTypeMatchJoin, appTransID, match.Price)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to create payment", 500)
	}
	ticket, err := s.createPaymentWSTicket(ctx, payment)
	if err != nil {
		s.failPaymentWSTicket(ctx, payment.ID, nil, err)
		return nil, paymentErr("PAYMENT_WS_TICKET_ERROR", "Payment notification channel unavailable. Please try again.", 503)
	}

	description := fmt.Sprintf("Match join fee for %s", matchID[:min64(len(matchID), 8)])
	zpResp, err := s.zalo.CreateOrder(ctx, zalopay.CreateOrderInput{
		AppTransID:            appTransID,
		Description:           description,
		GuestName:             buildDisplayName(player.UserFirstName, player.UserLastName, player.UserUsername),
		Amount:                match.Price,
		EmbedData:             zalopay.EmbedData{MatchPlayerID: matchPlayerID},
		ExpireDurationSeconds: int64(PaymentValiditySeconds),
	})
	if err != nil || zpResp == nil || zpResp.ReturnCode != 1 {
		msg := "Failed to create payment order"
		if zpResp != nil && zpResp.ReturnMessage != "" {
			msg = zpResp.ReturnMessage
		}
		return nil, paymentErr("PAYMENT_GATEWAY_ERROR", msg, 502)
	}

	_ = s.paymentRepo.UpdateOrderURL(ctx, payment.ID, zpResp.OrderURL, zpResp.ZPTransToken)

	payment, err = s.paymentRepo.FindByID(ctx, payment.ID)
	if err != nil || payment == nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to retrieve payment", 500)
	}

	qrResp, err := generateQRCode(&zpResp.OrderURL)
	if err != nil {
		return nil, paymentErr("INTERNAL_ERROR", "Failed to generate QR code", 500)
	}
	wsURL := BuildWSSubscribeURL(payment.ID, ticket, ws)
	return &dto.CreatePaymentResponse{
		Payment:        mapPaymentToDTO(payment),
		OrderURL:       zpResp.OrderURL,
		QRCode:         *qrResp,
		ZPTransToken:   &zpResp.ZPTransToken,
		ExpireAt:       FormatPaymentExpireAt(payment),
		WsSubscribeURL: wsURL,
	}, nil
}

// GetMatchPaymentStatus returns the current payment DTO and whether it is expired.
func (s *PaymentService) GetMatchPaymentStatus(ctx context.Context, paymentID string) (*dto.PaymentStatusResponse, error) {
	payment, err := s.EnsureCurrentStatus(ctx, paymentID)
	if err != nil || payment == nil {
		return nil, paymentErr("NOT_FOUND", "Payment not found", 404)
	}
	return &dto.PaymentStatusResponse{
		Payment:   mapPaymentToDTO(payment),
		IsExpired: payment.Status == domain.PaymentStatusExpired,
	}, nil
}

// ==================== Redis helpers ====================

func (s *PaymentService) acquireSlotLocks(ctx context.Context, slots []SlotLockSpec) (bool, error) {
	if s.redis == nil || len(slots) == 0 {
		return true, nil
	}
	return s.redis.AcquireSlotLocks(ctx, slots)
}

func (s *PaymentService) releaseSlotLocks(ctx context.Context, slots []SlotLockSpec) {
	if s.redis == nil || len(slots) == 0 {
		return
	}
	s.redis.ReleaseSlotLocks(ctx, slots)
}

func (s *PaymentService) createPaymentWSTicket(ctx context.Context, payment *domain.Payment) (string, error) {
	if s.redis == nil {
		return "", paymentErr("PAYMENT_WS_TICKET_ERROR", "redis unavailable for payment websocket ticket", 503)
	}
	ttl := s.paymentWSTicketTTL
	if ttl <= 0 {
		ttl = 60
	}
	duration := time.Duration(ttl) * time.Second
	remaining := s.RemainingValidity(payment)
	if remaining <= 0 {
		return "", paymentErr("PAYMENT_WS_TICKET_ERROR", "payment transaction expired", 503)
	}
	if duration > remaining {
		duration = remaining
	}
	return s.redis.CreatePaymentWSTicket(ctx, payment.ID, duration)
}

// failPaymentWSTicket marks the payment failed and releases slot locks when the
// WS ticket creation fails (preserving the handler's prior side-effect ordering).
func (s *PaymentService) failPaymentWSTicket(ctx context.Context, paymentID string, slots []SlotLockSpec, cause error) {
	if paymentID != "" {
		if _, updateErr := s.paymentRepo.UpdatePendingStatus(ctx, paymentID, domain.PaymentStatusFailed, nil, nil); updateErr != nil {
			s.logger.Warn("payment websocket ticket status update failed",
				zap.String("payment_id", paymentID), zap.Error(updateErr))
		}
	}
	s.releaseSlotLocks(ctx, slots)
	s.logger.Error("payment websocket ticket creation failed",
		zap.String("payment_id", paymentID), zap.Error(cause))
}

// ==================== Pure helpers ====================

// MapPaymentToDTO converts a domain payment into its DTO representation.
func MapPaymentToDTO(p *domain.Payment) dto.PaymentResponse { return mapPaymentToDTO(p) }

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

// FormatPaymentExpireAt returns the payment expiry timestamp as a JSON string.
func FormatPaymentExpireAt(p *domain.Payment) string {
	return p.CreatedAt.Add(PaymentValidity).Format("2006-01-02T15:04:05.000Z")
}

// generateQRCode renders an order URL as a base64-encoded PNG data URI.
func generateQRCode(orderURL *string) (*dto.QRCodeData, error) {
	urlStr := ""
	if orderURL != nil {
		urlStr = *orderURL
	}
	if urlStr == "" {
		return &dto.QRCodeData{}, nil
	}
	png, err := qrcode.Encode(urlStr, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}
	rawB64 := base64.StdEncoding.EncodeToString(png)
	return &dto.QRCodeData{
		Base64:    "data:image/png;base64," + rawB64,
		RawBase64: rawB64,
	}, nil
}

// BuildWSSubscribeURL builds the WebSocket URL the client should connect to in
// order to receive payment status notifications.
func BuildWSSubscribeURL(paymentID, ticket string, ws WSURLParams) string {
	scheme := "ws"
	proto := strings.ToLower(strings.TrimSpace(ws.ForwardedProto))
	if proto == "https" || ws.TLS || (proto == "" && ws.NodeEnv == "production") {
		scheme = "wss"
	}
	host := strings.TrimSpace(ws.ForwardedHost)
	if host == "" {
		host = ws.Host
	}
	if host == "" {
		host = fmt.Sprintf("localhost:%d", ws.Port)
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

var _ = sanitizeForKey // retained for parity with the previous handler helper

func min64(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildDisplayName mirrors the match handler's name helper.
func buildDisplayName(firstName, lastName, username *string) string {
	if firstName != nil && lastName != nil {
		return *firstName + " " + *lastName
	}
	if firstName != nil {
		return *firstName
	}
	if username != nil {
		return *username
	}
	return "Anonymous"
}

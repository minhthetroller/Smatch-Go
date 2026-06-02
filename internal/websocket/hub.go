package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// PaymentNotification is sent to Flutter when a payment completes.
type PaymentNotification struct {
	Type          string  `json:"type"`
	PaymentID     string  `json:"paymentId"`
	Status        string  `json:"status"`
	BookingID     *string `json:"bookingId"`
	MatchPlayerID *string `json:"matchPlayerId"`
	ZPTransID     *string `json:"zpTransId"`
	Message       string  `json:"message"`
}

// MatchNotification is the union type for match events.
type MatchNotification interface {
	matchNotification()
}

type MatchJoinRequest struct {
	Type         string  `json:"type"` // "match_join_request"
	MatchID      string  `json:"matchId"`
	PlayerID     string  `json:"playerId"`
	UserID       string  `json:"userId"`
	UserName     string  `json:"userName"`
	UserPhotoURL *string `json:"userPhotoUrl"`
	Message      *string `json:"message"`
	QueuePos     int     `json:"queuePosition"`
	Timestamp    string  `json:"timestamp"`
}

func (MatchJoinRequest) matchNotification() {}

type MatchRequestResponse struct {
	Type     string `json:"type"` // "match_request_response"
	MatchID  string `json:"matchId"`
	PlayerID string `json:"playerId"`
	Status   string `json:"status"`
	Position *int   `json:"position"`
	Message  string `json:"message"`
}

func (MatchRequestResponse) matchNotification() {}

type MatchPlayerLeft struct {
	Type           string `json:"type"` // "match_player_left"
	MatchID        string `json:"matchId"`
	UserID         string `json:"userId"`
	UserName       string `json:"userName"`
	SlotsRemaining int    `json:"slotsRemaining"`
}

func (MatchPlayerLeft) matchNotification() {}

type MatchStatusChange struct {
	Type    string `json:"type"` // "match_status_change"
	MatchID string `json:"matchId"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (MatchStatusChange) matchNotification() {}

// Hub manages all WebSocket connections.
type Hub struct {
	logger *zap.Logger
	mu     sync.RWMutex

	// Payment subscriptions
	paymentConn map[string]*websocket.Conn // paymentId -> conn
	connPayment map[*websocket.Conn]string // conn -> paymentId

	// Match subscriptions
	matchSubs  map[string]map[*websocket.Conn]struct{} // matchId -> conns
	connMatchs map[*websocket.Conn]map[string]struct{} // conn -> matchIds
	userConns  map[string]map[*websocket.Conn]struct{} // userId -> conns

	// Callbacks injected by handlers/services.
	ValidatePaymentTicket func(ctx context.Context, paymentID, ticket string) (bool, error)
	PaymentStatusSnapshot func(ctx context.Context, paymentID string) (*PaymentNotification, error)

	// Callback for auto-cancel on payment disconnect
	OnPaymentDisconnect func(paymentID string)
}

// NewHub creates a Hub.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		logger:      logger,
		paymentConn: make(map[string]*websocket.Conn),
		connPayment: make(map[*websocket.Conn]string),
		matchSubs:   make(map[string]map[*websocket.Conn]struct{}),
		connMatchs:  make(map[*websocket.Conn]map[string]struct{}),
		userConns:   make(map[string]map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

// ServePayments upgrades and handles a payment WebSocket connection.
func (h *Hub) ServePayments(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log().Error("ws upgrade", zap.Error(err))
		return
	}
	defer h.disconnectPayment(conn)

	_ = conn.WriteJSON(map[string]string{"type": "connected", "message": "Connected to payment notification service"})

	// Auto-subscribe when paymentId and a single-use ticket are provided.
	paymentID := r.URL.Query().Get("paymentId")
	ticket := r.URL.Query().Get("ticket")
	if paymentID != "" || ticket != "" {
		h.handlePaymentSubscribe(r.Context(), conn, paymentID, ticket)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var data struct {
			Action    string  `json:"action"`
			PaymentID *string `json:"paymentId"`
			Ticket    *string `json:"ticket"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			_ = conn.WriteJSON(map[string]string{"type": "error", "code": "INVALID_MESSAGE", "message": "Invalid message format"})
			continue
		}
		switch data.Action {
		case "subscribe":
			if data.PaymentID != nil {
				subscribeTicket := ""
				if data.Ticket != nil {
					subscribeTicket = *data.Ticket
				}
				h.handlePaymentSubscribe(r.Context(), conn, *data.PaymentID, subscribeTicket)
			}
		case "unsubscribe":
			if data.PaymentID != nil {
				h.unsubscribePayment(*data.PaymentID, conn)
			}
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		}
	}
}

// ServeMatches upgrades and handles a match WebSocket connection.
func (h *Hub) ServeMatches(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log().Error("ws upgrade", zap.Error(err))
		return
	}
	defer h.disconnectMatch(conn)

	_ = conn.WriteJSON(map[string]string{"type": "connected", "message": "Connected to match notification service"})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var data struct {
			Action  string  `json:"action"`
			MatchID *string `json:"matchId"`
			UserID  *string `json:"userId"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			_ = conn.WriteJSON(map[string]string{"error": "Invalid message format"})
			continue
		}
		switch data.Action {
		case "subscribe":
			if data.MatchID != nil {
				h.subscribeMatch(*data.MatchID, data.UserID, conn)
				_ = conn.WriteJSON(map[string]interface{}{"type": "subscribed", "matchId": *data.MatchID})
			}
		case "unsubscribe":
			if data.MatchID != nil {
				h.unsubscribeMatch(*data.MatchID, data.UserID, conn)
			}
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		}
	}
}

// NotifyPaymentStatus sends a payment notification to the sole subscriber of a paymentId.
func (h *Hub) NotifyPaymentStatus(n PaymentNotification) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conn := h.paymentConn[n.PaymentID]
	if conn != nil && isTerminalPaymentStatus(n.Status) {
		delete(h.paymentConn, n.PaymentID)
		delete(h.connPayment, conn)
	}

	if conn != nil {
		_ = conn.WriteJSON(n)
	}
}

// NotifyMatchSubscribers broadcasts a match notification to all subscribers.
func (h *Hub) NotifyMatchSubscribers(matchID string, n MatchNotification) {
	h.mu.RLock()
	conns := copyConns(h.matchSubs[matchID])
	h.mu.RUnlock()
	for conn := range conns {
		_ = conn.WriteJSON(n)
	}
}

// NotifyUser sends a notification directly to a user's connections.
func (h *Hub) NotifyUser(userID string, n MatchNotification) {
	h.mu.RLock()
	conns := copyConns(h.userConns[userID])
	h.mu.RUnlock()
	for conn := range conns {
		_ = conn.WriteJSON(n)
	}
}

func (h *Hub) handlePaymentSubscribe(ctx context.Context, conn *websocket.Conn, paymentID, ticket string) {
	ok, code, msg := h.subscribePayment(ctx, paymentID, ticket, conn)
	if !ok {
		writePaymentError(conn, code, paymentID, msg)
		return
	}

	h.mu.Lock()
	if h.connPayment[conn] == paymentID {
		_ = conn.WriteJSON(map[string]interface{}{"type": "subscribed", "paymentId": paymentID})
	}
	h.mu.Unlock()

	if h.PaymentStatusSnapshot == nil {
		return
	}
	n, err := h.PaymentStatusSnapshot(ctx, paymentID)
	if err != nil {
		h.log().Warn("payment status snapshot failed", zap.String("payment_id", paymentID), zap.Error(err))
		return
	}
	if n == nil {
		return
	}
	h.mu.Lock()
	if h.paymentConn[paymentID] == conn {
		_ = conn.WriteJSON(n)
		if isTerminalPaymentStatus(n.Status) {
			delete(h.paymentConn, paymentID)
			delete(h.connPayment, conn)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) subscribePayment(ctx context.Context, paymentID, ticket string, conn *websocket.Conn) (bool, string, string) {
	if paymentID == "" || ticket == "" {
		return false, "INVALID_PAYMENT_WS_TICKET", "Invalid or expired payment websocket ticket"
	}

	h.mu.Lock()
	if existingPaymentID, ok := h.connPayment[conn]; ok {
		h.mu.Unlock()
		if existingPaymentID == paymentID {
			return true, "", ""
		}
		return false, "PAYMENT_CONNECTION_ALREADY_SUBSCRIBED", "WebSocket connection is already subscribed to another payment"
	}
	if existingConn := h.paymentConn[paymentID]; existingConn != nil && existingConn != conn {
		h.mu.Unlock()
		return false, "PAYMENT_ALREADY_SUBSCRIBED", "Payment already has an active websocket subscriber"
	}
	h.mu.Unlock()

	if h.ValidatePaymentTicket == nil {
		return false, "INVALID_PAYMENT_WS_TICKET", "Invalid or expired payment websocket ticket"
	}
	valid, err := h.ValidatePaymentTicket(ctx, paymentID, ticket)
	if err != nil {
		h.log().Warn("payment websocket ticket validation failed", zap.String("payment_id", paymentID), zap.Error(err))
		return false, "INVALID_PAYMENT_WS_TICKET", "Invalid or expired payment websocket ticket"
	}
	if !valid {
		return false, "INVALID_PAYMENT_WS_TICKET", "Invalid or expired payment websocket ticket"
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if existingPaymentID, ok := h.connPayment[conn]; ok {
		if existingPaymentID == paymentID {
			return true, "", ""
		}
		return false, "PAYMENT_CONNECTION_ALREADY_SUBSCRIBED", "WebSocket connection is already subscribed to another payment"
	}
	if existingConn := h.paymentConn[paymentID]; existingConn != nil && existingConn != conn {
		return false, "PAYMENT_ALREADY_SUBSCRIBED", "Payment already has an active websocket subscriber"
	}
	h.paymentConn[paymentID] = conn
	h.connPayment[conn] = paymentID
	return true, "", ""
}

func (h *Hub) unsubscribePayment(paymentID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.paymentConn[paymentID] == conn {
		delete(h.paymentConn, paymentID)
	}
	if h.connPayment[conn] == paymentID {
		delete(h.connPayment, conn)
	}
}

func (h *Hub) disconnectPayment(conn *websocket.Conn) {
	h.mu.Lock()
	paymentID := h.connPayment[conn]
	if paymentID != "" {
		delete(h.paymentConn, paymentID)
	}
	delete(h.connPayment, conn)
	h.mu.Unlock()
	conn.Close()

	// Auto-cancel pending payments
	if h.OnPaymentDisconnect != nil && paymentID != "" {
		h.OnPaymentDisconnect(paymentID)
	}
}

func writePaymentError(conn *websocket.Conn, code, paymentID, message string) {
	_ = conn.WriteJSON(map[string]interface{}{
		"type":      "error",
		"code":      code,
		"paymentId": paymentID,
		"message":   message,
	})
}

func isTerminalPaymentStatus(status string) bool {
	switch status {
	case "success", "failed", "expired":
		return true
	default:
		return false
	}
}

func (h *Hub) subscribeMatch(matchID string, userID *string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.matchSubs[matchID] == nil {
		h.matchSubs[matchID] = make(map[*websocket.Conn]struct{})
	}
	h.matchSubs[matchID][conn] = struct{}{}
	if h.connMatchs[conn] == nil {
		h.connMatchs[conn] = make(map[string]struct{})
	}
	h.connMatchs[conn][matchID] = struct{}{}
	if userID != nil {
		if h.userConns[*userID] == nil {
			h.userConns[*userID] = make(map[*websocket.Conn]struct{})
		}
		h.userConns[*userID][conn] = struct{}{}
	}
}

func (h *Hub) unsubscribeMatch(matchID string, userID *string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.matchSubs[matchID], conn)
	delete(h.connMatchs[conn], matchID)
	if userID != nil {
		delete(h.userConns[*userID], conn)
	}
}

func (h *Hub) disconnectMatch(conn *websocket.Conn) {
	h.mu.Lock()
	for mid := range h.connMatchs[conn] {
		delete(h.matchSubs[mid], conn)
	}
	delete(h.connMatchs, conn)
	for uid, conns := range h.userConns {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.userConns, uid)
		}
	}
	h.mu.Unlock()
	conn.Close()
}

func copyConns(m map[*websocket.Conn]struct{}) map[*websocket.Conn]struct{} {
	c := make(map[*websocket.Conn]struct{}, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

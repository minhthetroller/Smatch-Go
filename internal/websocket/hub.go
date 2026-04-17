package websocket

import (
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
	Type     string  `json:"type"` // "match_request_response"
	MatchID  string  `json:"matchId"`
	PlayerID string  `json:"playerId"`
	Status   string  `json:"status"`
	Position *int    `json:"position"`
	Message  string  `json:"message"`
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
	paymentSubs map[string]map[*websocket.Conn]struct{} // paymentId -> conns
	connPayments map[*websocket.Conn]map[string]struct{} // conn -> paymentIds

	// Match subscriptions
	matchSubs  map[string]map[*websocket.Conn]struct{} // matchId -> conns
	connMatchs map[*websocket.Conn]map[string]struct{} // conn -> matchIds
	userConns  map[string]map[*websocket.Conn]struct{} // userId -> conns

	// Callback for auto-cancel on payment disconnect
	OnPaymentDisconnect func(paymentID string)
}

// NewHub creates a Hub.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		logger:       logger,
		paymentSubs:  make(map[string]map[*websocket.Conn]struct{}),
		connPayments: make(map[*websocket.Conn]map[string]struct{}),
		matchSubs:    make(map[string]map[*websocket.Conn]struct{}),
		connMatchs:   make(map[*websocket.Conn]map[string]struct{}),
		userConns:    make(map[string]map[*websocket.Conn]struct{}),
	}
}

// ServePayments upgrades and handles a payment WebSocket connection.
func (h *Hub) ServePayments(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade", zap.Error(err))
		return
	}
	defer h.disconnectPayment(conn)

	_ = conn.WriteJSON(map[string]string{"type": "connected", "message": "Connected to payment notification service"})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var data struct {
			Action    string  `json:"action"`
			PaymentID *string `json:"paymentId"`
		}
		if err := json.Unmarshal(msg, &data); err != nil {
			_ = conn.WriteJSON(map[string]string{"error": "Invalid message format"})
			continue
		}
		switch data.Action {
		case "subscribe":
			if data.PaymentID != nil {
				h.subscribePayment(*data.PaymentID, conn)
				_ = conn.WriteJSON(map[string]interface{}{"type": "subscribed", "paymentId": *data.PaymentID})
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
		h.logger.Error("ws upgrade", zap.Error(err))
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

// NotifyPaymentStatus sends a payment notification to all subscribers of a paymentId.
func (h *Hub) NotifyPaymentStatus(n PaymentNotification) {
	h.mu.RLock()
	conns := copyConns(h.paymentSubs[n.PaymentID])
	h.mu.RUnlock()

	for conn := range conns {
		_ = conn.WriteJSON(n)
	}
	// Clean up subscription
	h.mu.Lock()
	delete(h.paymentSubs, n.PaymentID)
	h.mu.Unlock()
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

func (h *Hub) subscribePayment(paymentID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.paymentSubs[paymentID] == nil {
		h.paymentSubs[paymentID] = make(map[*websocket.Conn]struct{})
	}
	h.paymentSubs[paymentID][conn] = struct{}{}
	if h.connPayments[conn] == nil {
		h.connPayments[conn] = make(map[string]struct{})
	}
	h.connPayments[conn][paymentID] = struct{}{}
}

func (h *Hub) unsubscribePayment(paymentID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.paymentSubs[paymentID], conn)
	delete(h.connPayments[conn], paymentID)
}

func (h *Hub) disconnectPayment(conn *websocket.Conn) {
	h.mu.Lock()
	paymentIDs := make([]string, 0, len(h.connPayments[conn]))
	for pid := range h.connPayments[conn] {
		paymentIDs = append(paymentIDs, pid)
		delete(h.paymentSubs[pid], conn)
	}
	delete(h.connPayments, conn)
	h.mu.Unlock()
	conn.Close()

	// Auto-cancel pending payments
	if h.OnPaymentDisconnect != nil {
		for _, pid := range paymentIDs {
			h.OnPaymentDisconnect(pid)
		}
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

package handler

import (
	"net/http"

	ws "github.com/smatch/badminton-backend/internal/websocket"
)

type WebSocketHandler struct {
	hub *ws.Hub
}

func NewWebSocketHandler(hub *ws.Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

func (h *WebSocketHandler) ServePayments(w http.ResponseWriter, r *http.Request) {
	h.hub.ServePayments(w, r)
}

func (h *WebSocketHandler) ServeMatches(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeMatches(w, r)
}

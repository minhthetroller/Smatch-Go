package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestServePaymentsRejectsDuplicateSubscriber(t *testing.T) {
	hub := newTestPaymentHub()
	server := httptest.NewServer(http.HandlerFunc(hub.ServePayments))
	defer server.Close()

	conn1 := dialPaymentWS(t, server.URL, "payment-1", "ticket-payment-1")
	defer func(conn1 *websocket.Conn) {
		err := conn1.Close()
		if err != nil {
			t.Fatalf("Unable to close the websocket: %v", err)
		}
	}(conn1)
	readPaymentMsg(t, conn1, "connected")
	readPaymentMsg(t, conn1, "subscribed")
	readPaymentMsg(t, conn1, "payment_status")

	conn2 := dialPaymentWS(t, server.URL, "payment-1", "ticket-payment-1")
	defer func(conn2 *websocket.Conn) {
		err := conn2.Close()
		if err != nil {
			t.Fatalf("Unable to close the websocket: %v", err)
		}
	}(conn2)
	readPaymentMsg(t, conn2, "connected")
	msg := readPaymentMsg(t, conn2, "error")
	if msg["code"] != "PAYMENT_ALREADY_SUBSCRIBED" {
		t.Fatalf("error code = %v, want PAYMENT_ALREADY_SUBSCRIBED", msg["code"])
	}
}

func TestNotifyPaymentStatusWritesOnceAndCleansUp(t *testing.T) {
	hub := newTestPaymentHub()
	disconnected := make(chan string, 1)
	hub.OnPaymentDisconnect = func(paymentID string) {
		disconnected <- paymentID
	}
	server := httptest.NewServer(http.HandlerFunc(hub.ServePayments))
	defer server.Close()

	conn := dialPaymentWS(t, server.URL, "payment-1", "ticket-payment-1")
	readPaymentMsg(t, conn, "connected")
	readPaymentMsg(t, conn, "subscribed")
	readPaymentMsg(t, conn, "payment_status")

	hub.NotifyPaymentStatus(PaymentNotification{
		Type:      "payment_status",
		PaymentID: "payment-1",
		Status:    "success",
		Message:   "Payment successful",
	})
	msg := readPaymentMsg(t, conn, "payment_status")
	if msg["status"] != "success" {
		t.Fatalf("status = %v, want success", msg["status"])
	}

	_ = conn.Close()
	select {
	case paymentID := <-disconnected:
		t.Fatalf("disconnect auto-cancelled terminal payment %s", paymentID)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServePaymentsRejectsInvalidTicket(t *testing.T) {
	hub := newTestPaymentHub()
	server := httptest.NewServer(http.HandlerFunc(hub.ServePayments))
	defer server.Close()

	conn := dialPaymentWS(t, server.URL, "payment-1", "wrong-ticket")
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			t.Fatalf("Unable to close the websocket: %v", err)
		}
	}(conn)
	readPaymentMsg(t, conn, "connected")
	msg := readPaymentMsg(t, conn, "error")
	if msg["code"] != "INVALID_PAYMENT_WS_TICKET" {
		t.Fatalf("error code = %v, want INVALID_PAYMENT_WS_TICKET", msg["code"])
	}
}

func newTestPaymentHub() *Hub {
	hub := NewHub(zap.NewNop())
	hub.ValidatePaymentTicket = func(_ context.Context, paymentID, ticket string) (bool, error) {
		return ticket == "ticket-"+paymentID, nil
	}
	hub.PaymentStatusSnapshot = func(_ context.Context, paymentID string) (*PaymentNotification, error) {
		return &PaymentNotification{
			Type:      "payment_status",
			PaymentID: paymentID,
			Status:    "pending",
			Message:   "Payment is pending",
		}, nil
	}
	return hub
}

func dialPaymentWS(t *testing.T, serverURL, paymentID, ticket string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/?paymentId=" + paymentID + "&ticket=" + ticket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func readPaymentMsg(t *testing.T, conn *websocket.Conn, wantType string) map[string]interface{} {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read websocket json: %v", err)
	}
	if msg["type"] != wantType {
		t.Fatalf("message type = %v, want %s; msg=%v", msg["type"], wantType, msg)
	}
	return msg
}

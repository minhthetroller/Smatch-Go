package domain

import (
	"encoding/json"
	"time"
)

type PaymentType string

const (
	PaymentTypeBooking   PaymentType = "BOOKING"
	PaymentTypeMatchJoin PaymentType = "MATCH_JOIN"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
	PaymentStatusExpired PaymentStatus = "expired"
)

type Payment struct {
	ID            string
	BookingID     *string
	MatchPlayerID *string
	PaymentType   PaymentType
	AppTransID    string
	ZPTransID     *string
	ZPTransToken  *string
	Amount        int
	Status        PaymentStatus
	OrderURL      *string
	CallbackData  json.RawMessage

	CreatedAt time.Time
	UpdatedAt time.Time
}

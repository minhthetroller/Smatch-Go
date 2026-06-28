package domain

import (
	"encoding/json"
	"time"
)

type Court struct {
	ID           string
	Name         string
	Description  *string
	PhoneNumbers []string

	AddressStreet   *string
	AddressWard     *string
	AddressDistrict *string
	AddressCity     *string

	Details      json.RawMessage
	OpeningHours json.RawMessage

	Lat *float64
	Lng *float64

	OwnerUserID *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubCourt struct {
	ID          string
	CourtID     string
	Name        string
	Description *string
	IsActive    bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PricingRule struct {
	ID           string
	CourtID      string
	Name         string
	DayType      string // 'weekday', 'weekend', 'holiday'
	StartTime    string // HH:MM
	EndTime      string // HH:MM
	PricePerHour int
	IsActive     bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubCourtClosure struct {
	ID         string
	SubCourtID string
	Date       time.Time
	StartTime  *string // nil = full day
	EndTime    *string
	Reason     *string
	CreatedAt  time.Time
}

type Holiday struct {
	ID         string
	Date       time.Time
	Name       *string
	Multiplier float64
	CreatedAt  time.Time
}

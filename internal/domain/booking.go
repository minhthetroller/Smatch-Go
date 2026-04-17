package domain

import "time"

type Booking struct {
	ID          string
	SubCourtID  string
	UserID      *string
	GuestName   *string
	GuestPhone  *string
	GuestEmail  *string
	Date        time.Time
	StartTime   string // HH:MM
	EndTime     string // HH:MM
	TotalPrice  int
	Status      string // pending, confirmed, cancelled, completed, failed
	Notes       *string
	GroupID     *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

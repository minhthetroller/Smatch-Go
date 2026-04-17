package dto

// TimeSlot sent to Flutter in availability response
type TimeSlot struct {
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	IsAvailable bool   `json:"isAvailable"`
	Price       int    `json:"price"`
}

// SubCourtAvailabilityResponse nested in CourtAvailabilityResponse
type SubCourtAvailabilityResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	IsActive    bool       `json:"isActive"`
	Slots       []TimeSlot `json:"slots"`
}

// CourtAvailabilityResponse sent to Flutter
type CourtAvailabilityResponse struct {
	CourtID     string                         `json:"courtId"`
	CourtName   string                         `json:"courtName"`
	Date        string                         `json:"date"`
	DayType     string                         `json:"dayType"`
	OpeningTime string                         `json:"openingTime"`
	ClosingTime string                         `json:"closingTime"`
	SubCourts   []SubCourtAvailabilityResponse `json:"subCourts"`
}

// SingleBookingItem for multi-booking request
type SingleBookingItem struct {
	SubCourtID string `json:"subCourtId" validate:"required,uuid"`
	Date       string `json:"date" validate:"required"`
	StartTime  string `json:"startTime" validate:"required"`
	EndTime    string `json:"endTime" validate:"required"`
}

// CreateBookingRequest from Flutter
type CreateBookingRequest struct {
	// Either single booking fields OR bookings array
	SubCourtID string `json:"subCourtId,omitempty"`
	Date       string `json:"date,omitempty"`
	StartTime  string `json:"startTime,omitempty"`
	EndTime    string `json:"endTime,omitempty"`

	Bookings []SingleBookingItem `json:"bookings,omitempty"`

	GuestName  string  `json:"guestName"`
	GuestPhone string  `json:"guestPhone"`
	GuestEmail *string `json:"guestEmail,omitempty"`
	Notes      *string `json:"notes,omitempty"`
}

// BookingResponse sent to Flutter
type BookingResponse struct {
	ID           string  `json:"id"`
	SubCourtID   string  `json:"subCourtId"`
	SubCourtName string  `json:"subCourtName"`
	CourtID      string  `json:"courtId"`
	CourtName    string  `json:"courtName"`
	GuestName    string  `json:"guestName"`
	GuestPhone   string  `json:"guestPhone"`
	GuestEmail   *string `json:"guestEmail"`
	Date         string  `json:"date"`
	StartTime    string  `json:"startTime"`
	EndTime      string  `json:"endTime"`
	TotalPrice   int     `json:"totalPrice"`
	Status       string  `json:"status"`
	Notes        *string `json:"notes"`
	CreatedAt    string  `json:"createdAt"`
}

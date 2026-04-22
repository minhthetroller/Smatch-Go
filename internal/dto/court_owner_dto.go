package dto

import "encoding/json"

// CourtOwnerCourtResponse sent to court owner web app
type CourtOwnerCourtResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     *string         `json:"description"`
	PhoneNumbers    []string        `json:"phoneNumbers"`
	AddressStreet   *string         `json:"addressStreet"`
	AddressWard     *string         `json:"addressWard"`
	AddressDistrict *string         `json:"addressDistrict"`
	AddressCity     *string         `json:"addressCity"`
	Details         json.RawMessage `json:"details"`
	OpeningHours    json.RawMessage `json:"openingHours"`
	Lat             *float64        `json:"lat"`
	Lng             *float64        `json:"lng"`
	IsActive        bool            `json:"isActive"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

// CourtStatsResponse sent to court owner web app
type CourtStatsResponse struct {
	TotalBookings      int64   `json:"totalBookings"`
	TotalRevenue       int64   `json:"totalRevenue"`
	OccupancyRate      float64 `json:"occupancyRate"`
	CancellationRate   float64 `json:"cancellationRate"`
}

// CloseCourtRequest from court owner web app
type CloseCourtRequest struct {
	Date      string `json:"date" validate:"required"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Reason    string `json:"reason" validate:"required"`
}

// CloseSubCourtRequest from court owner web app
type CloseSubCourtRequest struct {
	Date      string `json:"date" validate:"required"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Reason    string `json:"reason" validate:"required"`
}

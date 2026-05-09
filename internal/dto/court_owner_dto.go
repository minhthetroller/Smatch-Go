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

// CourtOwnerCourtDetailResponse sent to court owner web app
type CourtOwnerCourtDetailResponse struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      *string                  `json:"description"`
	PhoneNumbers     []string                 `json:"phoneNumbers"`
	AddressStreet    *string                  `json:"addressStreet"`
	AddressWard      *string                  `json:"addressWard"`
	AddressDistrict  *string                  `json:"addressDistrict"`
	AddressCity      *string                  `json:"addressCity"`
	Details          json.RawMessage          `json:"details"`
	OpeningHours     json.RawMessage          `json:"openingHours"`
	Lat              *float64                 `json:"lat"`
	Lng              *float64                 `json:"lng"`
	IsActive         bool                     `json:"isActive"`
	SubCourts        []SubCourtResponse       `json:"subCourts"`
	PricingRules     []PricingRuleResponse    `json:"pricingRules"`
	UpcomingClosures []SubCourtClosureResponse `json:"upcomingClosures"`
	CreatedAt        string                   `json:"createdAt"`
	UpdatedAt        string                   `json:"updatedAt"`
}

// SubCourtResponse nested in CourtOwnerCourtDetailResponse
type SubCourtResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    bool    `json:"isActive"`
}

// PricingRuleResponse nested in CourtOwnerCourtDetailResponse
type PricingRuleResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DayType      string `json:"dayType"`
	StartTime    string `json:"startTime"`
	EndTime      string `json:"endTime"`
	PricePerHour int    `json:"pricePerHour"`
	IsActive     bool   `json:"isActive"`
}

// SubCourtClosureResponse nested in CourtOwnerCourtDetailResponse
type SubCourtClosureResponse struct {
	ID         string  `json:"id"`
	SubCourtID string  `json:"subCourtId"`
	Date       string  `json:"date"`
	StartTime  *string `json:"startTime"`
	EndTime    *string `json:"endTime"`
	Reason     *string `json:"reason"`
}

// CourtStatsDailyItem for time-series stats
type CourtStatsDailyItem struct {
	Date          string `json:"date"`
	Bookings      int64  `json:"bookings"`
	Revenue       int64  `json:"revenue"`
	Cancellations int64  `json:"cancellations"`
}

// CourtStatsDetailResponse includes summary + optional daily breakdown
type CourtStatsDetailResponse struct {
	Summary          CourtStatsResponse      `json:"summary"`
	DailyBreakdown   []CourtStatsDailyItem   `json:"dailyBreakdown,omitempty"`
}

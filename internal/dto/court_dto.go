package dto

import "encoding/json"

// CourtResponse sent to Flutter
type CourtResponse struct {
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
	Distance        *float64        `json:"distance,omitempty"`
	DistanceKm      *float64        `json:"distanceKm,omitempty"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

// CreateCourtRequest from admin
type CreateCourtRequest struct {
	Name            string          `json:"name" validate:"required"`
	Description     *string         `json:"description,omitempty"`
	PhoneNumbers    []string        `json:"phoneNumbers"`
	AddressStreet   *string         `json:"addressStreet,omitempty"`
	AddressWard     *string         `json:"addressWard,omitempty"`
	AddressDistrict *string         `json:"addressDistrict,omitempty"`
	AddressCity     *string         `json:"addressCity,omitempty"`
	Details         json.RawMessage `json:"details"`
	OpeningHours    json.RawMessage `json:"openingHours"`
	Lat             *float64        `json:"lat,omitempty"`
	Lng             *float64        `json:"lng,omitempty"`
}

// UpdateCourtRequest from admin
type UpdateCourtRequest struct {
	Name            *string         `json:"name,omitempty"`
	Description     *string         `json:"description,omitempty"`
	PhoneNumbers    []string        `json:"phoneNumbers,omitempty"`
	AddressStreet   *string         `json:"addressStreet,omitempty"`
	AddressWard     *string         `json:"addressWard,omitempty"`
	AddressDistrict *string         `json:"addressDistrict,omitempty"`
	AddressCity     *string         `json:"addressCity,omitempty"`
	Details         json.RawMessage `json:"details,omitempty"`
	OpeningHours    json.RawMessage `json:"openingHours,omitempty"`
	Lat             *float64        `json:"lat,omitempty"`
	Lng             *float64        `json:"lng,omitempty"`
}

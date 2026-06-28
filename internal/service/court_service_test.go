package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
)

func TestMapCourtToDTO(t *testing.T) {
	lat := 21.0285
	lng := 105.8542
	desc := "A nice court"
	street := "123 Main St"
	ward := "Ba Dinh"
	district := "Hoan Kiem"
	city := "Ha Noi"

	c := &domain.Court{
		ID:              "court-1",
		Name:            "Test Court",
		Description:     &desc,
		PhoneNumbers:    []string{"0123456789"},
		AddressStreet:   &street,
		AddressWard:     &ward,
		AddressDistrict: &district,
		AddressCity:     &city,
		Details:         json.RawMessage(`{"courts": 5}`),
		OpeningHours:    json.RawMessage(`{"mon": "06:00-22:00"}`),
		Lat:             &lat,
		Lng:             &lng,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	resp := mapCourtToDTO(c)

	if resp.ID != "court-1" {
		t.Errorf("ID = %q, want %q", resp.ID, "court-1")
	}
	if resp.Name != "Test Court" {
		t.Errorf("Name = %q, want %q", resp.Name, "Test Court")
	}
	if resp.Lat == nil || *resp.Lat != 21.0285 {
		t.Errorf("Lat = %v, want 21.0285", resp.Lat)
	}
	if resp.Lng == nil || *resp.Lng != 105.8542 {
		t.Errorf("Lng = %v, want 105.8542", resp.Lng)
	}
	if len(resp.PhoneNumbers) != 1 || resp.PhoneNumbers[0] != "0123456789" {
		t.Errorf("PhoneNumbers = %v, want [0123456789]", resp.PhoneNumbers)
	}
	if resp.CreatedAt != "2026-01-01T00:00:00.000Z" {
		t.Errorf("CreatedAt = %q, want 2026-01-01T00:00:00.000Z", resp.CreatedAt)
	}
}

func TestMapCourtToDTO_NilPhoneNumbers(t *testing.T) {
	c := &domain.Court{
		ID:           "court-1",
		Name:         "Test Court",
		PhoneNumbers: nil,
		Details:      json.RawMessage(`{}`),
		OpeningHours: json.RawMessage(`{}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	resp := mapCourtToDTO(c)

	// Nil phone_numbers should become empty array
	if resp.PhoneNumbers == nil {
		t.Error("PhoneNumbers should be empty array, not nil")
	}
	if len(resp.PhoneNumbers) != 0 {
		t.Errorf("PhoneNumbers length = %d, want 0", len(resp.PhoneNumbers))
	}
}

func TestMapCourtToDTO_NilLatLng(t *testing.T) {
	c := &domain.Court{
		ID:           "court-1",
		Name:         "Test Court",
		PhoneNumbers: []string{},
		Details:      json.RawMessage(`{}`),
		OpeningHours: json.RawMessage(`{}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	resp := mapCourtToDTO(c)

	if resp.Lat != nil {
		t.Errorf("Lat should be nil, got %v", resp.Lat)
	}
	if resp.Lng != nil {
		t.Errorf("Lng should be nil, got %v", resp.Lng)
	}
}

func TestParseNearbyRadius_Default(t *testing.T) {
	got, err := ParseNearbyRadius("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != courtNearbyDefaultKm {
		t.Errorf("default radius = %v, want %v", got, courtNearbyDefaultKm)
	}
}

func TestParseNearbyRadius_Boundaries(t *testing.T) {
	for _, raw := range []string{"5km", "50km", "10km"} {
		if _, err := ParseNearbyRadius(raw); err != nil {
			t.Errorf("radius=%s should be valid, got %v", raw, err)
		}
	}
	for _, raw := range []string{"4km", "51km"} {
		if _, err := ParseNearbyRadius(raw); err == nil {
			t.Errorf("radius=%s should be invalid", raw)
		}
	}
}

func TestParseNearbyRadius_Suffix(t *testing.T) {
	for _, raw := range []string{"10", "10m", "abckm"} {
		if _, err := ParseNearbyRadius(raw); err == nil {
			t.Errorf("radius=%q should be invalid (suffix/numeric)", raw)
		}
	}
}

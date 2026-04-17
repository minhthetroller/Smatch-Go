package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/repository"
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

func TestSendSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	sendSuccess(w, map[string]string{"hello": "world"}, http.StatusOK)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", w.Header().Get("Content-Type"))
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("success = %v, want true", resp["success"])
	}
	data := resp["data"].(map[string]interface{})
	if data["hello"] != "world" {
		t.Errorf("data.hello = %v, want world", data["hello"])
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	sendError(w, "Something went wrong", "BAD_REQUEST", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != false {
		t.Errorf("success = %v, want false", resp["success"])
	}
	errObj := resp["error"].(map[string]interface{})
	if errObj["message"] != "Something went wrong" {
		t.Errorf("error.message = %v, want 'Something went wrong'", errObj["message"])
	}
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("error.code = %v, want BAD_REQUEST", errObj["code"])
	}
}

func TestSendPaginated(t *testing.T) {
	w := httptest.NewRecorder()
	sendPaginated(w, []string{"a", "b"}, 2, 10, 25)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("success = %v, want true", resp["success"])
	}

	meta := resp["meta"].(map[string]interface{})
	pagination := meta["pagination"].(map[string]interface{})
	if int(pagination["page"].(float64)) != 2 {
		t.Errorf("page = %v, want 2", pagination["page"])
	}
	if int(pagination["limit"].(float64)) != 10 {
		t.Errorf("limit = %v, want 10", pagination["limit"])
	}
	if int(pagination["total"].(float64)) != 25 {
		t.Errorf("total = %v, want 25", pagination["total"])
	}
	if int(pagination["totalPages"].(float64)) != 3 {
		t.Errorf("totalPages = %v, want 3", pagination["totalPages"])
	}
	if pagination["hasNext"] != true {
		t.Errorf("hasNext = %v, want true", pagination["hasNext"])
	}
	if pagination["hasPrev"] != true {
		t.Errorf("hasPrev = %v, want true", pagination["hasPrev"])
	}
}

func TestSendPaginated_FirstPage(t *testing.T) {
	w := httptest.NewRecorder()
	sendPaginated(w, []string{}, 1, 10, 0)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	meta := resp["meta"].(map[string]interface{})
	pagination := meta["pagination"].(map[string]interface{})
	if pagination["hasPrev"] != false {
		t.Errorf("hasPrev = %v, want false on first page", pagination["hasPrev"])
	}
	if pagination["hasNext"] != false {
		t.Errorf("hasNext = %v, want false for empty results", pagination["hasNext"])
	}
	// totalPages should be 1 even when total=0
	if int(pagination["totalPages"].(float64)) != 1 {
		t.Errorf("totalPages = %v, want 1", pagination["totalPages"])
	}
}

func TestSendAppError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := domain.NotFound("User not found")
	sendAppError(w, appErr)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	errObj := resp["error"].(map[string]interface{})
	if errObj["message"] != "User not found" {
		t.Errorf("message = %v, want 'User not found'", errObj["message"])
	}
}

func TestSendAppError_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	sendAppError(w, domain.ErrNotFound) // plain error, not AppError

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// Suppress unused variable warning for dto and repository packages.
var (
	_ dto.CourtResponse
	_ repository.BookingRow
)

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
)

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

func TestCourtResponse_HasDistanceKmField(t *testing.T) {
	distM := 1500.0
	distKm := 1.5
	resp := dto.CourtResponse{
		ID:           "x",
		Name:         "Y",
		PhoneNumbers: []string{},
		Details:      json.RawMessage(`{}`),
		OpeningHours: json.RawMessage(`{}`),
		CreatedAt:    "2026-01-01T00:00:00.000Z",
		UpdatedAt:    "2026-01-01T00:00:00.000Z",
		Distance:     &distM,
		DistanceKm:   &distKm,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["distance"]; !ok {
		t.Error("JSON key 'distance' must still exist for backward compatibility")
	}
	if _, ok := m["distanceKm"]; !ok {
		t.Error("JSON key 'distanceKm' not found")
	}
	if m["distance"].(float64) != 1500.0 {
		t.Errorf("distance = %v, want 1500.0", m["distance"])
	}
	if m["distanceKm"].(float64) != 1.5 {
		t.Errorf("distanceKm = %v, want 1.5", m["distanceKm"])
	}
}

func TestNearby_MissingLatLng(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestNearby_RadiusMissingKmSuffix(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby?lat=21.0&lng=105.8&radius=10", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing km suffix", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	errObj := resp["error"].(map[string]interface{})
	msg, _ := errObj["message"].(string)
	if msg == "" {
		t.Errorf("expected an error message, got %q", msg)
	}
}

func TestNearby_RadiusWrongSuffix(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby?lat=21.0&lng=105.8&radius=10m", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for wrong suffix 'm'", w.Code)
	}
}

func TestNearby_RadiusNotANumber(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby?lat=21.0&lng=105.8&radius=abckm", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for non-numeric radius", w.Code)
	}
}

func TestNearby_RadiusBelowMin(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby?lat=21.0&lng=105.8&radius=4km", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for radius=4km (below min 5km)", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	errObj := resp["error"].(map[string]interface{})
	msg, _ := errObj["message"].(string)
	if msg == "" {
		t.Errorf("expected an error message, got %q", msg)
	}
}

func TestNearby_RadiusExceedsMax(t *testing.T) {
	h := &CourtHandler{svc: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/courts/nearby?lat=21.0&lng=105.8&radius=51km", nil)
	w := httptest.NewRecorder()
	h.Nearby(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for radius=51km", w.Code)
	}
}

func TestNearby_RadiusValidBoundary(t *testing.T) {
	for _, radiusStr := range []string{"5km", "50km", "10km"} {
		h := &CourtHandler{svc: nil}
		url := "/api/courts/nearby?lat=21.0&lng=105.8&radius=" + radiusStr
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		func() {
			defer func() { recover() }() //nolint:errcheck
			h.Nearby(w, req)
		}()
		if w.Code == http.StatusBadRequest {
			t.Errorf("radius=%s should be valid, got 400", radiusStr)
		}
	}
}

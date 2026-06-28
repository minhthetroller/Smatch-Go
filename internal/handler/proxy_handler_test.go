package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMapTile_ReturnsGone(t *testing.T) {
	h := NewProxyHandler("http://localhost:7800", "public.courts")

	req := httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", w.Code)
	}
}

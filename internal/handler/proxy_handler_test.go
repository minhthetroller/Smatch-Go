package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// withChiURLParams creates a request with chi URL params set in its context.
func withChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestMapTile_CacheHit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tile-data")) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	// First request — cache miss
	req1 := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w1 := httptest.NewRecorder()
	h.MapTile(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w1.Code)
	}
	if w1.Header().Get("X-Tile-Cache") != "MISS" {
		t.Errorf("first request: X-Tile-Cache = %q, want MISS", w1.Header().Get("X-Tile-Cache"))
	}

	// Second request — cache hit
	req2 := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w2 := httptest.NewRecorder()
	h.MapTile(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second request: status = %d, want 200", w2.Code)
	}
	if w2.Header().Get("X-Tile-Cache") != "HIT" {
		t.Errorf("second request: X-Tile-Cache = %q, want HIT", w2.Header().Get("X-Tile-Cache"))
	}
	body := w2.Body.String()
	if body != "tile-data" {
		t.Errorf("cached body = %q, want %q", body, "tile-data")
	}
}

func TestMapTile_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMapTile_UpstreamDown(t *testing.T) {
	// Point to a closed port
	h := NewProxyHandler("http://127.0.0.1:1", "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestMapTile_ContentTypeForwarding(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mvt-data")) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	if w.Header().Get("Content-Type") != "application/vnd.mapbox-vector-tile" {
		t.Errorf("Content-Type = %q, want application/vnd.mapbox-vector-tile", w.Header().Get("Content-Type"))
	}
}

func TestMapTile_DefaultContentType(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explicitly remove Content-Type to trigger fallback
		w.Header().Del("Content-Type")
		w.Header().Set("Content-Type", "")
		w.WriteHeader(http.StatusOK)
		// Write binary data so Go doesn't auto-detect as text/plain
		w.Write([]byte{0x00, 0x01, 0x02}) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	// When upstream sends empty Content-Type, proxy should default to application/x-protobuf
	if w.Header().Get("Content-Type") != "application/x-protobuf" {
		t.Errorf("Content-Type = %q, want application/x-protobuf", w.Header().Get("Content-Type"))
	}
}

func TestMapTile_CORSAndCacheHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Cache-Control") != "public, max-age=604800, immutable" {
		t.Errorf("Cache-Control = %q, want standard caching header", w.Header().Get("Cache-Control"))
	}
}

func TestMapTile_UpstreamURL(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	req := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/13006/7212.pbf", nil),
		map[string]string{"z": "14", "x": "13006", "y": "7212"},
	)
	w := httptest.NewRecorder()
	h.MapTile(w, req)

	expected := "/public.courts/14/13006/7212.pbf"
	if receivedPath != expected {
		t.Errorf("upstream path = %q, want %q", receivedPath, expected)
	}
}

func TestMapTile_DoesNotCacheErrors(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success")) //nolint:errcheck
	}))
	defer upstream.Close()

	h := NewProxyHandler(upstream.URL, "public.courts")

	// First request — upstream error
	req1 := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/1/1.pbf", nil),
		map[string]string{"z": "14", "x": "1", "y": "1"},
	)
	w1 := httptest.NewRecorder()
	h.MapTile(w1, req1)

	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("first request: status = %d, want 500", w1.Code)
	}

	// Second request — should NOT be cached, should hit upstream again
	req2 := withChiURLParams(
		httptest.NewRequest("GET", "/api/map-tiles/14/1/1.pbf", nil),
		map[string]string{"z": "14", "x": "1", "y": "1"},
	)
	w2 := httptest.NewRecorder()
	h.MapTile(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second request: status = %d, want 200", w2.Code)
	}
	if callCount != 2 {
		t.Errorf("upstream was called %d times, want 2", callCount)
	}
}

// ── Tile Cache Unit Tests ──

func TestTileCache_GetSet(t *testing.T) {
	c := &tileCache{
		entries: make(map[string]tileEntry),
		ttl:     1 * time.Hour,
	}

	// Get from empty cache
	_, ok := c.get("test-key")
	if ok {
		t.Error("expected cache miss on empty cache")
	}

	// Set and get
	c.set("test-key", tileEntry{data: []byte("test"), contentType: "ct", cachedAt: time.Now()})
	entry, ok := c.get("test-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(entry.data) != "test" {
		t.Errorf("data = %q, want %q", string(entry.data), "test")
	}
	if entry.contentType != "ct" {
		t.Errorf("contentType = %q, want %q", entry.contentType, "ct")
	}
}

func TestTileCache_TTLExpiry(t *testing.T) {
	c := &tileCache{
		entries: make(map[string]tileEntry),
		ttl:     1 * time.Millisecond,
	}

	c.set("key", tileEntry{data: []byte("d"), contentType: "ct", cachedAt: time.Now()})
	time.Sleep(5 * time.Millisecond)

	_, ok := c.get("key")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestTileCache_MultipleKeys(t *testing.T) {
	c := &tileCache{
		entries: make(map[string]tileEntry),
		ttl:     1 * time.Hour,
	}

	c.set("a", tileEntry{data: []byte("data-a"), contentType: "ct", cachedAt: time.Now()})
	c.set("b", tileEntry{data: []byte("data-b"), contentType: "ct", cachedAt: time.Now()})

	a, ok := c.get("a")
	if !ok || string(a.data) != "data-a" {
		t.Errorf("key a: got %q, want data-a", string(a.data))
	}
	b, ok := c.get("b")
	if !ok || string(b.data) != "data-b" {
		t.Errorf("key b: got %q, want data-b", string(b.data))
	}
}

func TestNewProxyHandler(t *testing.T) {
	h := NewProxyHandler("http://localhost:7800", "public.courts")
	if h.tileServerURL != "http://localhost:7800" {
		t.Errorf("tileServerURL = %q, want %q", h.tileServerURL, "http://localhost:7800")
	}
	if h.tileLayerID != "public.courts" {
		t.Errorf("tileLayerID = %q, want %q", h.tileLayerID, "public.courts")
	}
	if h.client == nil {
		t.Error("client should not be nil")
	}
	if h.cache == nil {
		t.Error("cache should not be nil")
	}
}

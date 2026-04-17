package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type tileEntry struct {
	data        []byte
	contentType string
	cachedAt    time.Time
}

type tileCache struct {
	mu      sync.RWMutex
	entries map[string]tileEntry
	ttl     time.Duration
}

func (c *tileCache) get(key string) (tileEntry, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Since(e.cachedAt) > c.ttl {
		return tileEntry{}, false
	}
	return e, true
}

func (c *tileCache) set(key string, e tileEntry) {
	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
}

func (c *tileCache) startCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.entries {
				if now.Sub(e.cachedAt) > c.ttl {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}()
}

type ProxyHandler struct {
	tileServerURL string
	tileLayerID   string
	client        *http.Client
	cache         *tileCache
}

func NewProxyHandler(tileServerURL, tileLayerID string) *ProxyHandler {
	c := &tileCache{
		entries: make(map[string]tileEntry),
		ttl:     24 * time.Hour,
	}
	c.startCleanup(1 * time.Hour)
	return &ProxyHandler{
		tileServerURL: tileServerURL,
		tileLayerID:   tileLayerID,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		cache: c,
	}
}

// GET /api/map-tiles/:z/:x/:y.pbf
func (h *ProxyHandler) MapTile(w http.ResponseWriter, r *http.Request) {
	z := chi.URLParam(r, "z")
	x := chi.URLParam(r, "x")
	y := chi.URLParam(r, "y")

	key := fmt.Sprintf("tile:%s/%s/%s", z, x, y)

	if entry, ok := h.cache.get(key); ok {
		w.Header().Set("Content-Type", entry.contentType)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		w.Header().Set("X-Tile-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write(entry.data) //nolint:errcheck
		return
	}

	url := fmt.Sprintf("%s/%s/%s/%s/%s.pbf", h.tileServerURL, h.tileLayerID, z, x, y)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, "Tile server unavailable", http.StatusBadGateway)
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("[tile-proxy] upstream request failed: %s: %v", url, err)
		http.Error(w, "Tile server unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Tile server unavailable", http.StatusBadGateway)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/x-protobuf"
	}

	if resp.StatusCode == http.StatusOK {
		h.cache.set(key, tileEntry{data: body, contentType: ct, cachedAt: time.Now()})
	} else {
		log.Printf("[tile-proxy] upstream returned %d for %s", resp.StatusCode, url)
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("X-Tile-Cache", "MISS")
	w.WriteHeader(resp.StatusCode)
	w.Write(body) //nolint:errcheck
}

package handler

import "net/http"

// ProxyHandler is retained as a safety fallback.
// Tile requests are now routed directly to the pg_tileserv Fargate service by the ALB
// via a path-based listener rule on /api/map-tiles/*.
// If a request somehow bypasses the ALB rule and reaches this handler,
// return 410 Gone to signal the route is no longer served here.
type ProxyHandler struct{}

func NewProxyHandler(_, _ string) *ProxyHandler { return &ProxyHandler{} }

// MapTile returns 410 Gone. The ALB routes /api/map-tiles/* to pg_tileserv
// before this handler is reached under normal operation.
func (h *ProxyHandler) MapTile(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Gone: tile requests are served by the tile service", http.StatusGone)
}

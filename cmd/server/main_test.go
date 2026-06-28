package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadTestStressRateLimitBypass(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/load-test/stress", nil)
	req.Header.Set("X-Admin-Secret", "secret")

	if !loadTestStressRateLimitBypass(req, true, "secret") {
		t.Fatal("expected authenticated enabled stress request to bypass rate limit")
	}
}

func TestLoadTestStressRateLimitBypassRejectsOtherRequests(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		enabled     bool
		adminSecret string
		header      string
	}{
		{name: "disabled", method: http.MethodPost, path: "/api/load-test/stress", enabled: false, adminSecret: "secret", header: "secret"},
		{name: "empty secret", method: http.MethodPost, path: "/api/load-test/stress", enabled: true, adminSecret: "", header: ""},
		{name: "wrong secret", method: http.MethodPost, path: "/api/load-test/stress", enabled: true, adminSecret: "secret", header: "wrong"},
		{name: "wrong method", method: http.MethodGet, path: "/api/load-test/stress", enabled: true, adminSecret: "secret", header: "secret"},
		{name: "wrong path", method: http.MethodPost, path: "/health", enabled: true, adminSecret: "secret", header: "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-Admin-Secret", tt.header)

			if loadTestStressRateLimitBypass(req, tt.enabled, tt.adminSecret) {
				t.Fatal("expected request not to bypass rate limit")
			}
		})
	}
}

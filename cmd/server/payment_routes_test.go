package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/smatch/badminton-backend/internal/middleware"
)

func TestRegisterPaymentRoutes_AuthBehavior(t *testing.T) {
	authMw := middleware.NewAuthMiddleware(nil, nil, "")

	var createCalled bool
	var getStatusCalled bool
	var cancelCalled bool
	var callbackCalled bool

	router := chi.NewRouter()
	registerPaymentRoutes(router, authMw, paymentRouteHandlers{
		create: func(w http.ResponseWriter, _ *http.Request) {
			createCalled = true
			w.WriteHeader(http.StatusCreated)
		},
		callback: func(w http.ResponseWriter, _ *http.Request) {
			callbackCalled = true
			w.WriteHeader(http.StatusOK)
		},
		get: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		getStatus: func(w http.ResponseWriter, _ *http.Request) {
			getStatusCalled = true
			w.WriteHeader(http.StatusOK)
		},
		cancel: func(w http.ResponseWriter, _ *http.Request) {
			cancelCalled = true
			w.WriteHeader(http.StatusOK)
		},
	})

	t.Run("create allows anonymous requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payments/create", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}
		if !createCalled {
			t.Fatal("expected create handler to be called")
		}
	})

	t.Run("callback remains public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payments/callback", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !callbackCalled {
			t.Fatal("expected callback handler to be called")
		}
	})

	t.Run("status still requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/payment-1/status", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if getStatusCalled {
			t.Fatal("expected status handler not to be called")
		}
	})

	t.Run("cancel still requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/payments/payment-1/cancel", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if cancelCalled {
			t.Fatal("expected cancel handler not to be called")
		}
	})
}

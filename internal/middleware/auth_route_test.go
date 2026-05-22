package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/smatch/badminton-backend/internal/domain"
)

func TestCourtBookingPaymentGuestRoutePolicy(t *testing.T) {
	authMw := NewAuthMiddleware(nil, nil, "")
	r := chi.NewRouter()
	r.Route("/api/bookings", func(r chi.Router) {
		r.With(authMw.OptionalAuth).Post("/", statusHandler(http.StatusNoContent))
		r.With(authMw.RequireAuth).Delete("/{id}", statusHandler(http.StatusNoContent))
		r.With(authMw.OptionalAuth).Get("/{id}/payment", statusHandler(http.StatusNoContent))
	})
	r.Route("/api/payments", func(r chi.Router) {
		r.With(authMw.OptionalAuth).Post("/create", statusHandler(http.StatusNoContent))
		r.With(authMw.RequireAuth).Get("/{id}", statusHandler(http.StatusNoContent))
		r.With(authMw.RequireAuth).Post("/{id}/cancel", statusHandler(http.StatusNoContent))
	})

	publicCases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/bookings/"},
		{http.MethodPost, "/api/payments/create"},
		{http.MethodGet, "/api/bookings/booking-1/payment"},
	}
	for _, tc := range publicCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, rec.Code, http.StatusNoContent)
		}
	}

	protectedCases := []struct {
		method string
		path   string
	}{
		{http.MethodDelete, "/api/bookings/booking-1"},
		{http.MethodGet, "/api/payments/payment-1"},
		{http.MethodPost, "/api/payments/payment-1/cancel"},
	}
	for _, tc := range protectedCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestOptionalAuthDoesNotRejectAnonymousUserContext(t *testing.T) {
	authMw := NewAuthMiddleware(nil, nil, "")
	r := chi.NewRouter()
	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := &domain.User{ID: "user-1", IsAnonymous: true}
			ctx := context.WithValue(r.Context(), CtxKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, authMw.OptionalAuth).Post("/api/bookings/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || !user.IsAnonymous {
			t.Fatalf("anonymous user missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/bookings/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func statusHandler(status int) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

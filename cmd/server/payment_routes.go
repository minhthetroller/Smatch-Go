package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/smatch/badminton-backend/internal/middleware"
)

type paymentRouteHandlers struct {
	create    http.HandlerFunc
	callback  http.HandlerFunc
	get       http.HandlerFunc
	getStatus http.HandlerFunc
	cancel    http.HandlerFunc
}

func registerPaymentRoutes(r chi.Router, authMw *middleware.AuthMiddleware, h paymentRouteHandlers) {
	r.Route("/payments", func(r chi.Router) {
		r.With(authMw.OptionalAuth).Post("/create", h.create)
		r.With(httprate.LimitByIP(10, time.Minute)).Post("/callback", h.callback)
		r.With(authMw.RequireAuth).Get("/{id}", h.get)
		r.With(authMw.RequireAuth).Get("/{id}/status", h.getStatus)
		r.With(authMw.RequireAuth).Post("/{id}/cancel", h.cancel)
	})
}

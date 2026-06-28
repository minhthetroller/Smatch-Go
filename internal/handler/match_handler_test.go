package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
)

func newMatchHandlerBoundary() *MatchHandler {
	return NewMatchHandler(service.NewMatchService(
		(*repository.MatchRepository)(nil),
		nil,
		nil,
		testResolver,
	))
}

func setReqUser(r *http.Request, u *domain.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.CtxKeyUser, u))
}

func TestMatch_CreateMatch_InvalidBody(t *testing.T) {
	h := newMatchHandlerBoundary()
	req := setReqUser(httptest.NewRequest(http.MethodPost, "/api/matches", bytes.NewBufferString("{bad")), &domain.User{ID: "u1"})
	rec := httptest.NewRecorder()
	h.CreateMatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMatch_RespondToJoinRequest_InvalidBody(t *testing.T) {
	h := newMatchHandlerBoundary()
	req := setReqUser(httptest.NewRequest(http.MethodPut, "/api/matches/m1/requests/p1/respond", bytes.NewBufferString("x")), &domain.User{ID: "u1"})
	rec := httptest.NewRecorder()
	h.RespondToJoinRequest(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/middleware"
	"github.com/smatch/badminton-backend/internal/repository"
	"github.com/smatch/badminton-backend/internal/service"
)

var sentinelErrUser = &domain.User{ID: "user-1"}

func newAuthHandlerBoundary() *AuthHandler {
	// Service deps are nil — these tests only exercise request-decoding boundaries
	// that fail before any service/repository call is made.
	return NewAuthHandler(service.NewAuthService(
		nil,
		(*repository.UserRepository)(nil),
		(*repository.AvailabilityRepository)(nil),
		(*service.UploadService)(nil),
		testResolver,
	))
}

func TestAuth_Verify_InvalidBody(t *testing.T) {
	h := newAuthHandlerBoundary()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAuth_UpdateMe_InvalidBody(t *testing.T) {
	h := newAuthHandlerBoundary()
	req := httptest.NewRequest(http.MethodPut, "/api/auth/me", bytes.NewBufferString("nope"))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxKeyUser, sentinelErrUser))
	rec := httptest.NewRecorder()
	h.UpdateMe(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAuth_UploadPhoto_NoFile(t *testing.T) {
	h := newAuthHandlerBoundary()
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	err := writer.Close()
	if err != nil {
		return
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/photo", buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxKeyUser, sentinelErrUser))
	rec := httptest.NewRecorder()
	h.UploadPhoto(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d", rec.Code)
	}
}

func TestAuth_CheckUsername_MissingParam(t *testing.T) {
	h := newAuthHandlerBoundary()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/username/check", nil)
	rec := httptest.NewRecorder()
	h.CheckUsername(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing username, got %d", rec.Code)
	}
}

func TestAuth_Convert_InvalidBody(t *testing.T) {
	h := newAuthHandlerBoundary()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/convert", bytes.NewBufferString("x"))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxKeyUser, sentinelErrUser))
	rec := httptest.NewRecorder()
	h.Convert(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

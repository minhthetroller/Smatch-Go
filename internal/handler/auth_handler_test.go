package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/middleware"
)

type stubProfileUploader struct {
	key       string
	err       error
	gotUserID string
	gotOldKey string
}

func (s *stubProfileUploader) UploadProfilePhoto(_ context.Context, userID, oldKey string, _ multipart.File, _ *multipart.FileHeader) (string, error) {
	s.gotUserID = userID
	s.gotOldKey = oldKey
	return s.key, s.err
}

type stubUserRepoForPhoto struct {
	updatedID     string
	updatedFields map[string]interface{}
	returnUser    *domain.User
	returnErr     error
}

func (s *stubUserRepoForPhoto) UpdateProfile(_ context.Context, id string, fields map[string]interface{}) (*domain.User, error) {
	s.updatedID = id
	s.updatedFields = fields
	return s.returnUser, s.returnErr
}

func newAuthHandlerWithStubs(uploader *stubProfileUploader, userRepo *stubUserRepoForPhoto) *AuthHandler {
	return &AuthHandler{
		profileUpd: userRepo,
		upload:     uploader,
		images:     testResolver,
	}
}

func strPtr(s string) *string { return &s }

func makePhotoRequest(body []byte) *http.Request {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	part, _ := writer.CreateFormFile("image", "avatar.jpg")
	_, _ = part.Write(body)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/photo", buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadPhoto_NilUploader(t *testing.T) {
	h := &AuthHandler{upload: nil, images: testResolver}

	req := makePhotoRequest([]byte("fake"))
	ctx := context.WithValue(req.Context(), middleware.CtxKeyUser, &domain.User{ID: "user-1"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.UploadPhoto(rec, req)

	if rec.Code != 503 {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestUploadPhoto_NoFile(t *testing.T) {
	uploader := &stubProfileUploader{key: "profile/user-1/abc.jpg"}
	h := newAuthHandlerWithStubs(uploader, &stubUserRepoForPhoto{})

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/photo", buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), middleware.CtxKeyUser, &domain.User{ID: "user-1"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.UploadPhoto(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400 for missing file, got %d", rec.Code)
	}
}

func TestUploadPhoto_Success(t *testing.T) {
	uploader := &stubProfileUploader{key: "profile/user-1/abc.jpg"}
	userRepo := &stubUserRepoForPhoto{
		returnUser: &domain.User{
			ID:        "user-1",
			PhotoURL:  strPtr("profile/user-1/abc.jpg"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	h := newAuthHandlerWithStubs(uploader, userRepo)

	req := makePhotoRequest([]byte("fake-image-data"))
	ctx := context.WithValue(req.Context(), middleware.CtxKeyUser, &domain.User{ID: "user-1"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.UploadPhoto(rec, req)

	if rec.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if userRepo.updatedFields["photo_url"] != "profile/user-1/abc.jpg" {
		t.Errorf("expected UpdateProfile called with photo_url=key, got %v", userRepo.updatedFields["photo_url"])
	}
}

func TestUploadPhoto_ReplacesOldPhoto(t *testing.T) {
	uploader := &stubProfileUploader{key: "profile/user-1/new.jpg"}
	userRepo := &stubUserRepoForPhoto{
		returnUser: &domain.User{
			ID:        "user-1",
			PhotoURL:  strPtr("profile/user-1/new.jpg"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	h := newAuthHandlerWithStubs(uploader, userRepo)

	req := makePhotoRequest([]byte("fake-image-data"))
	ctx := context.WithValue(req.Context(), middleware.CtxKeyUser, &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("profile/user-1/old.jpg"),
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.UploadPhoto(rec, req)

	if uploader.gotOldKey != "profile/user-1/old.jpg" {
		t.Errorf("expected old key passed to uploader, got %q", uploader.gotOldKey)
	}
}

func TestMapUserToDTO_ResolvesPhotoURL(t *testing.T) {
	h := &AuthHandler{images: testResolver}
	user := &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("profile/user-1/abc.jpg"),
	}

	resp := h.mapUserToDTO(user)

	if resp.PhotoURL == nil || *resp.PhotoURL != "http://localhost:4566/smatch-profiles/profile/user-1/abc.jpg" {
		t.Errorf("expected resolved photoUrl, got %v", resp.PhotoURL)
	}
}

func TestMapUserToDTO_TolerantFullURL(t *testing.T) {
	h := &AuthHandler{images: testResolver}
	user := &domain.User{
		ID:       "user-1",
		PhotoURL: strPtr("https://cdn.example.com/already.jpg"),
	}

	resp := h.mapUserToDTO(user)

	if resp.PhotoURL == nil || *resp.PhotoURL != "https://cdn.example.com/already.jpg" {
		t.Errorf("expected full URL to pass through, got %v", resp.PhotoURL)
	}
}

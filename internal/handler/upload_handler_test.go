package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smatch/badminton-backend/internal/domain"
	"github.com/smatch/badminton-backend/internal/dto"
	"github.com/smatch/badminton-backend/internal/imageurl"
)

type stubUploadService struct {
	key string
	err error
}

func (s stubUploadService) UploadMatchImage(_ context.Context, _ multipart.File, _ *multipart.FileHeader) (string, error) {
	return s.key, s.err
}

var testResolver = imageurl.New("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")

func TestUploadHandler_NilUploadService(t *testing.T) {
	h := NewUploadHandler(nil, testResolver)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.jpg")
	_, _ = part.Write([]byte("fake-image-data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/match-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.UploadMatchImage(rec, req)

	if rec.Code != 503 {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestUploadHandler_NoFile(t *testing.T) {
	h := NewUploadHandler(stubUploadService{}, testResolver)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/match-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.UploadMatchImage(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400 for missing file, got %d", rec.Code)
	}
}

func TestUploadHandler_Success(t *testing.T) {
	h := NewUploadHandler(stubUploadService{key: "matches/test.jpg"}, testResolver)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("fake-image-data")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/match-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.UploadMatchImage(rec, req)

	if rec.Code != 201 {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp struct {
		Success bool                    `json:"success"`
		Data    dto.ImageUploadResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success response")
	}
	if resp.Data.Key != "matches/test.jpg" {
		t.Fatalf("unexpected key %q, want %q", resp.Data.Key, "matches/test.jpg")
	}
	if resp.Data.URL != "http://localhost:4566/smatch-matches/matches/test.jpg" {
		t.Fatalf("unexpected url %q", resp.Data.URL)
	}
	if resp.Data.FileName != "test.jpg" {
		t.Fatalf("unexpected filename %q", resp.Data.FileName)
	}
}

func TestUploadHandler_ServiceError(t *testing.T) {
	h := NewUploadHandler(stubUploadService{err: domain.BadRequest("Unsupported image type")}, testResolver)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "test.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("not-an-image")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/match-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.UploadMatchImage(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Unsupported image type")) {
		t.Fatalf("expected error body to mention unsupported image type, got %s", rec.Body.String())
	}
}

func TestUploadHandler_ServiceGenericError(t *testing.T) {
	h := NewUploadHandler(stubUploadService{err: errors.New("boom")}, testResolver)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("fake-image-data")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/match-image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.UploadMatchImage(rec, req)

	if rec.Code != 500 {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

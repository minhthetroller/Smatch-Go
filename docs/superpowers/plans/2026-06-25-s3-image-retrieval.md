# S3 Image Upload & Retrieval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement real S3 file upload and retrieval for profile photos and match images, with image URLs returned inside the matches-list and profile responses, using LocalStack for local development.

**Architecture:** Store S3 keys (not full URLs) in the database. An `ImageURLResolver` injected into handlers builds public URLs from keys at response time using a configurable base URL. Buckets are public-read (S3 bucket policy). `UploadService` returns keys; handlers resolve them to URLs. A `localstack` service in docker-compose.yml provides S3 for local testing.

**Tech Stack:** Go 1.23, chi router, pgx/v5, AWS SDK for Go v2, LocalStack, docker-compose

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/handler/imageurl.go` | Create | `ImageURLResolver` struct and resolve methods |
| `internal/handler/imageurl_test.go` | Create | Unit tests for resolver |
| `internal/config/config.go` | Modify | Add `PublicBaseURLMatches`/`PublicBaseURLProfile` config fields |
| `platform/s3/s3.go` | Modify | Add `EnsureBucketPublicRead` method |
| `internal/service/upload_service.go` | Modify | `UploadMatchImage` returns key; add `UploadProfilePhoto`; add `DeleteObject` to interface |
| `internal/service/upload_service_test.go` | Modify | Update existing tests; add profile photo tests |
| `internal/dto/upload_dto.go` | Modify | Add `Key` field to `ImageUploadResponse` |
| `internal/handler/upload_handler.go` | Modify | Inject resolver; return `{ key, url, fileName }` |
| `internal/handler/upload_handler_test.go` | Modify | Update tests for new constructor + response shape |
| `internal/handler/auth_handler.go` | Modify | Inject uploader + resolver; implement `UploadPhoto`; `mapUserToDTO` becomes method |
| `internal/handler/auth_handler_test.go` | Create | Tests for `UploadPhoto` and `mapUserToDTO` resolution |
| `internal/handler/match_handler.go` | Modify | Inject resolver; `mapMatchRowToDTO`/`mapPlayerRowToDTO`/`mapPlayersToDTO` become methods |
| `internal/handler/match_handler_test.go` | Modify | Update tests for method-based mapping + URL resolution |
| `cmd/server/main.go` | Modify | Build resolver; wire into handlers; call `EnsureBucketPublicRead` |
| `docker-compose.yml` | Modify | Add `localstack` service |
| `.env.example` | Modify | Add `AWS_S3_PUBLIC_BASE_URL_*` vars |

---

## Task 1: ImageURLResolver

**Files:**
- Create: `internal/handler/imageurl.go`
- Test: `internal/handler/imageurl_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/handler/imageurl_test.go`:

```go
package handler

import "testing"

func TestImageURLResolver_Match_PrependBase(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Match("matches/abc-123.jpg")
	want := "http://localhost:4566/smatch-matches/matches/abc-123.jpg"
	if got != want {
		t.Errorf("Match() = %q, want %q", got, want)
	}
}

func TestImageURLResolver_Profile_PrependBase(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Profile("profile/user-1/abc.jpg")
	want := "http://localhost:4566/smatch-profiles/profile/user-1/abc.jpg"
	if got != want {
		t.Errorf("Profile() = %q, want %q", got, want)
	}
}

func TestImageURLResolver_TolerantFullURL(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	got := r.Match("https://cdn.example.com/already-a-url.jpg")
	if got != "https://cdn.example.com/already-a-url.jpg" {
		t.Errorf("full URL should pass through, got %q", got)
	}
}

func TestImageURLResolver_EmptyKey(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")
	if r.Match("") != "" {
		t.Error("empty key should return empty string")
	}
	if r.Profile("") != "" {
		t.Error("empty key should return empty string")
	}
}

func TestImageURLResolver_TrailingSlashNormalization(t *testing.T) {
	r := NewImageURLResolver("http://localhost:4566/smatch-matches/", "http://localhost:4566/smatch-profiles/")
	got := r.Match("matches/abc.jpg")
	want := "http://localhost:4566/smatch-matches/matches/abc.jpg"
	if got != want {
		t.Errorf("with trailing slash, Match() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestImageURLResolver -v`
Expected: FAIL — `undefined: NewImageURLResolver`

- [ ] **Step 3: Write minimal implementation**

Create `internal/handler/imageurl.go`:

```go
package handler

import "strings"

// ImageURLResolver builds public S3 URLs from stored object keys.
// It is tolerant: values that already look like full URLs (http://, https://)
// are returned as-is, so existing DB rows with full URLs need no migration.
type ImageURLResolver struct {
	matchesBase string
	profileBase string
}

// NewImageURLResolver creates a resolver with the given public base URLs.
// Trailing slashes on base URLs are normalized away.
func NewImageURLResolver(matchesBase, profileBase string) ImageURLResolver {
	return ImageURLResolver{
		matchesBase: strings.TrimRight(matchesBase, "/"),
		profileBase: strings.TrimRight(profileBase, "/"),
	}
}

// Match resolves a match-image key to a public URL.
func (r ImageURLResolver) Match(key string) string {
	return resolve(r.matchesBase, key)
}

// Profile resolves a profile-photo key to a public URL.
func (r ImageURLResolver) Profile(key string) string {
	return resolve(r.profileBase, key)
}

func resolve(base, key string) string {
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	return base + "/" + key
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestImageURLResolver -v`
Expected: PASS — all 5 tests

- [ ] **Step 5: Commit**

```bash
git add internal/handler/imageurl.go internal/handler/imageurl_test.go
git commit -m "feat: add ImageURLResolver for S3 key-to-URL resolution"
```

---

## Task 2: Config additions

**Files:**
- Modify: `internal/config/config.go:39-47` (AWS struct) and `:91-97` (Load)
- Modify: `.env.example:38-45` (AWS S3 section)

- [ ] **Step 1: Add config fields**

In `internal/config/config.go`, add two fields to the `AWS` struct (after line 46 `BucketBusinessDocs string`):

```go
	AWS struct {
		Region             string
		AccessKeyID        string
		SecretAccessKey    string
		Endpoint           string // optional: LocalStack override
		BucketProfile      string
		BucketMatches      string
		BucketBusinessDocs string
		PublicBaseURLMatches string // optional: override for match image public URL base (e.g. CloudFront)
		PublicBaseURLProfile string // optional: override for profile photo public URL base
	}
```

- [ ] **Step 2: Load the new env vars**

In `internal/config/config.go` `Load()`, after line 97 (`cfg.AWS.BucketBusinessDocs = ...`), add:

```go
	cfg.AWS.PublicBaseURLMatches = getEnv("AWS_S3_PUBLIC_BASE_URL_MATCHES", "")
	cfg.AWS.PublicBaseURLProfile = getEnv("AWS_S3_PUBLIC_BASE_URL_PROFILE", "")
```

- [ ] **Step 3: Update .env.example**

In `.env.example`, after the `AWS_S3_BUCKET_BUSINESS_DOS` line (line 44), add:

```
AWS_S3_BUCKET_BUSINESS_DOCS=smatch-business-docs  # bucket for court owner business documents
# AWS_S3_PUBLIC_BASE_URL_MATCHES=   # optional: CDN base for match image URLs (defaults to S3 endpoint)
# AWS_S3_PUBLIC_BASE_URL_PROFILE=   # optional: CDN base for profile photo URLs (defaults to S3 endpoint)
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/config/`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go .env.example
git commit -m "feat: add AWS_S3_PUBLIC_BASE_URL_* config for CDN-ready image URLs"
```

---

## Task 3: S3 EnsureBucketPublicRead

**Files:**
- Modify: `platform/s3/s3.go` (add method at end of file)

- [ ] **Step 1: Add EnsureBucketPublicRead method**

Append to `platform/s3/s3.go` (after the `DeleteObject` method at line 141):

```go
// EnsureBucketPublicRead applies a bucket policy granting s3:GetObject to everyone.
// This makes all objects in the bucket publicly readable. Idempotent — safe to call on startup.
func (c *Client) EnsureBucketPublicRead(ctx context.Context, bucket string) error {
	if bucket == "" {
		return nil
	}
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": "*",
				"Action": "s3:GetObject",
				"Resource": "arn:aws:s3:::%s/*"
			}
		]
	}`, bucket)

	_, err := c.s3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		return fmt.Errorf("s3: set public-read policy for bucket %q: %w", bucket, err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./platform/s3/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add platform/s3/s3.go
git commit -m "feat: add EnsureBucketPublicRead to S3 client"
```

---

## Task 4: UploadService — return keys + UploadProfilePhoto

**Files:**
- Modify: `internal/service/upload_service.go`
- Test: `internal/service/upload_service_test.go`

- [ ] **Step 1: Write failing tests for UploadProfilePhoto**

Add to `internal/service/upload_service_test.go` (after the existing `stubS3Uploader` struct, add `DeleteObject` to the stub and new tests):

First, update `stubS3Uploader` to implement `DeleteObject`:

```go
type stubS3Uploader struct {
	putErr     error
	deleteErr  error
	gotKey     string
	deletedKey string
}

func (s *stubS3Uploader) PutObject(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func (s *stubS3Uploader) PutObjectEncrypted(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func (s *stubS3Uploader) DeleteObject(_ context.Context, _, key string) error {
	s.deletedKey = key
	return s.deleteErr
}
```

Then update `TestUploadMatchImage_Success` — change the URL assertion to a key assertion:

```go
func TestUploadMatchImage_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-matches")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "photo.jpg"}

	key, err := svc.UploadMatchImage(context.Background(), file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(key, "matches/") {
		t.Errorf("expected key to start with 'matches/', got: %s", key)
	}

	if !strings.HasPrefix(stub.gotKey, "matches/") {
		t.Errorf("unexpected key: %s", stub.gotKey)
	}
}
```

Add new tests at the end of the file:

```go
func TestUploadProfilePhoto_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.jpg"}

	key, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(key, "profile/user-123/") {
		t.Errorf("expected key to start with 'profile/user-123/', got: %s", key)
	}
}

func TestUploadProfilePhoto_InvalidExtension(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake pdf data")
	header := &multipart.FileHeader{Filename: "doc.pdf"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err == nil {
		t.Fatal("expected an error for invalid extension, got nil")
	}
}

func TestUploadProfilePhoto_DeletesOldPhoto(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.jpg"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "profile/user-123/old-photo.jpg", file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stub.deletedKey != "profile/user-123/old-photo.jpg" {
		t.Errorf("expected old key to be deleted, got deletedKey=%q", stub.deletedKey)
	}
}

func TestUploadProfilePhoto_S3Error(t *testing.T) {
	stub := &stubS3Uploader{putErr: errors.New("s3 down")}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.png"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err == nil {
		t.Fatal("expected an error when S3 fails, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestUploadProfilePhoto -v`
Expected: FAIL — `undefined: UploadProfilePhoto` and `stubS3Uploader does not implement s3Uploader (missing DeleteObject)`

Run: `go test ./internal/service/ -run TestUploadMatchImage_Success -v`
Expected: FAIL — key assertion fails (still returns URL)

- [ ] **Step 3: Update UploadService implementation**

In `internal/service/upload_service.go`:

1. Add `DeleteObject` to the `s3Uploader` interface (line 16-19):

```go
type s3Uploader interface {
	PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	PutObjectEncrypted(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	DeleteObject(ctx context.Context, bucket, key string) error
}
```

2. Change `UploadMatchImage` to return key instead of URL (line 60-74). Replace the return statement on line 73:

```go
func (s *UploadService) UploadMatchImage(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedImageExts[ext]
	if !ok {
		return "", domain.BadRequest("Unsupported image type. Allowed: jpg, jpeg, png")
	}

	key := fmt.Sprintf("matches/%s-%d%s", uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObject(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload image", Status: 500, Err: err}
	}

	return key, nil
}
```

3. Add `UploadProfilePhoto` method (at end of file):

```go
// UploadProfilePhoto uploads a profile photo for a user and deletes the previous photo if any.
// Returns the S3 key (not a full URL).
func (s *UploadService) UploadProfilePhoto(ctx context.Context, userID, oldKey string, file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedImageExts[ext]
	if !ok {
		return "", domain.BadRequest("Unsupported image type. Allowed: jpg, jpeg, png")
	}

	key := fmt.Sprintf("profile/%s/%s-%d%s", userID, uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObject(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload profile photo", Status: 500, Err: err}
	}

	if oldKey != "" {
		if err := s.s3.DeleteObject(ctx, s.bucket, oldKey); err != nil {
			// Best-effort cleanup; don't fail the upload
			fmt.Printf("[upload] warning: failed to delete old profile photo %q: %v\n", oldKey, err)
		}
	}

	return key, nil
}
```

Note: the `fmt` import is already present in the file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -v`
Expected: PASS — all tests including new profile photo tests and updated match image test

- [ ] **Step 5: Commit**

```bash
git add internal/service/upload_service.go internal/service/upload_service_test.go
git commit -m "feat: UploadMatchImage returns key; add UploadProfilePhoto with old-photo cleanup"
```

---

## Task 5: DTO — add Key field to ImageUploadResponse

**Files:**
- Modify: `internal/dto/upload_dto.go`

- [ ] **Step 1: Add Key field**

In `internal/dto/upload_dto.go`, update `ImageUploadResponse`:

```go
type ImageUploadResponse struct {
	Key      string `json:"key"`
	URL      string `json:"url"`
	FileName string `json:"fileName"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/dto/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/dto/upload_dto.go
git commit -m "feat: add Key field to ImageUploadResponse DTO"
```

---

## Task 6: UploadHandler — inject resolver, return key + url

**Files:**
- Modify: `internal/handler/upload_handler.go`
- Test: `internal/handler/upload_handler_test.go`

- [ ] **Step 1: Write failing tests**

Update `internal/handler/upload_handler_test.go`:

Update `stubUploadService` to return a key:

```go
type stubUploadService struct {
	key string
	err error
}

func (s stubUploadService) UploadMatchImage(_ context.Context, _ multipart.File, _ *multipart.FileHeader) (string, error) {
	return s.key, s.err
}
```

Update all test constructors to pass a resolver. Replace the entire test file with:

```go
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
)

type stubUploadService struct {
	key string
	err error
}

func (s stubUploadService) UploadMatchImage(_ context.Context, _ multipart.File, _ *multipart.FileHeader) (string, error) {
	return s.key, s.err
}

var testResolver = NewImageURLResolver("http://localhost:4566/smatch-matches", "http://localhost:4566/smatch-profiles")

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestUploadHandler -v`
Expected: FAIL — `too few arguments in call to NewUploadHandler`

- [ ] **Step 3: Update UploadHandler implementation**

In `internal/handler/upload_handler.go`, replace the entire file:

```go
package handler

import (
	"context"
	"mime/multipart"
	"net/http"

	"github.com/smatch/badminton-backend/internal/dto"
)

type matchImageUploader interface {
	UploadMatchImage(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error)
}

type UploadHandler struct {
	upload matchImageUploader
	images ImageURLResolver
}

func NewUploadHandler(upload matchImageUploader, images ImageURLResolver) *UploadHandler {
	return &UploadHandler{upload: upload, images: images}
}

const maxUploadSize = 5 << 20

func (h *UploadHandler) UploadMatchImage(w http.ResponseWriter, r *http.Request) {
	if h.upload == nil {
		sendError(w, "Image upload not available", "UPLOAD_UNAVAILABLE", 503)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, "File too large or invalid form data", "BAD_REQUEST", 400)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		sendError(w, "No image file provided", "BAD_REQUEST", 400)
		return
	}
	defer file.Close()

	key, err := h.upload.UploadMatchImage(r.Context(), file, header)
	if err != nil {
		sendAppError(w, err)
		return
	}

	sendSuccess(w, dto.ImageUploadResponse{
		Key:      key,
		URL:      h.images.Match(key),
		FileName: header.Filename,
	}, 201)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestUploadHandler -v`
Expected: PASS — all 5 tests

- [ ] **Step 5: Commit**

```bash
git add internal/handler/upload_handler.go internal/handler/upload_handler_test.go
git commit -m "feat: UploadHandler injects ImageURLResolver, returns key + url"
```

---

## Task 7: AuthHandler — inject uploader + resolver, implement UploadPhoto

**Files:**
- Modify: `internal/handler/auth_handler.go`
- Create: `internal/handler/auth_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/handler/auth_handler_test.go`:

```go
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

// --- Stubs ---

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

// --- Tests ---

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestUploadPhoto -v`
Expected: FAIL — `AuthHandler has no field upload`, `AuthHandler has no field profileUpd`, `AuthHandler has no field images`

Run: `go test ./internal/handler/ -run TestMapUserToDTO -v`
Expected: FAIL — `h.mapUserToDTO undefined`

- [ ] **Step 3: Update AuthHandler implementation**

In `internal/handler/auth_handler.go`:

1. Add `profilePhotoUploader` interface and `userProfileUpdater` interface, and new fields to the struct (replace lines 16-24). Note: `userRepo` stays as `*repository.UserRepository` for all existing methods (Upsert, FindByUsername, AddFCMToken, etc.). A separate `profileUpd` field holds the narrow interface used only by `UploadPhoto`, so it can be mocked in tests:

```go
type profilePhotoUploader interface {
	UploadProfilePhoto(ctx context.Context, userID, oldKey string, file multipart.File, header *multipart.FileHeader) (string, error)
}

type userProfileUpdater interface {
	UpdateProfile(ctx context.Context, id string, fields map[string]interface{}) (*domain.User, error)
}

type AuthHandler struct {
	firebase   *firebasepkg.Client
	userRepo   *repository.UserRepository
	availRepo  *repository.AvailabilityRepository
	upload     profilePhotoUploader
	profileUpd userProfileUpdater
	images     ImageURLResolver
}

func NewAuthHandler(fb *firebasepkg.Client, ur *repository.UserRepository, ar *repository.AvailabilityRepository, upload profilePhotoUploader, images ImageURLResolver) *AuthHandler {
	return &AuthHandler{firebase: fb, userRepo: ur, availRepo: ar, upload: upload, profileUpd: ur, images: images}
}
```

Note: `*repository.UserRepository` satisfies `userProfileUpdater` because it has `UpdateProfile`. Need to add `"mime/multipart"` to imports.

2. Convert `mapUserToDTO` from a package-level function to a method on `*AuthHandler` (line 256). Replace the function signature and the `PhotoURL` assignment:

```go
func (h *AuthHandler) mapUserToDTO(u *domain.User) dto.UserProfileResponse {
	if u == nil {
		return dto.UserProfileResponse{}
	}
	resp := dto.UserProfileResponse{
		ID:          u.ID,
		FirebaseUID: u.FirebaseUID,
		Email:       u.Email,
		Username:    u.Username,
		Provider:    u.Provider,
		IsAnonymous: u.IsAnonymous,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Gender:      u.Gender,
		PhoneNumber: u.PhoneNumber,
		PhotoURL:    h.resolvePhotoURL(u.PhotoURL),
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if u.AddressStreet != nil || u.AddressWard != nil || u.AddressDistrict != nil || u.AddressCity != nil {
		resp.Address = &dto.UserAddressResponse{
			Street:   u.AddressStreet,
			Ward:     u.AddressWard,
			District: u.AddressDistrict,
			City:     u.AddressCity,
		}
	}
	return resp
}

func (h *AuthHandler) resolvePhotoURL(photoURL *string) *string {
	if photoURL == nil || *photoURL == "" {
		return photoURL
	}
	resolved := h.images.Profile(*photoURL)
	return &resolved
}
```

3. Update all call sites of `mapUserToDTO` to `h.mapUserToDTO`:
   - Line 66: `User: h.mapUserToDTO(created),`
   - Line 91: `sendSuccess(w, dto.AuthResponse{User: h.mapUserToDTO(created), IsNewUser: isNew}, 201)`
   - Line 97: `sendSuccess(w, h.mapUserToDTO(user), 200)`
   - Line 146: `sendSuccess(w, h.mapUserToDTO(updated), 200)`
   - Line 342: `sendSuccess(w, dto.AuthResponse{User: h.mapUserToDTO(updated), IsNewUser: false}, 200)`

4. Replace the `UploadPhoto` stub (lines 295-298) with the real implementation:

```go
// POST /api/auth/me/photo - Upload profile photo
func (h *AuthHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	if h.upload == nil {
		sendError(w, "Photo upload not available", "UPLOAD_UNAVAILABLE", 503)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, "File too large or invalid form data", "BAD_REQUEST", 400)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		sendError(w, "No image file provided", "BAD_REQUEST", 400)
		return
	}
	defer file.Close()

	user := middleware.UserFromContext(r.Context())

	var oldKey string
	if user.PhotoURL != nil && *user.PhotoURL != "" {
		oldKey = *user.PhotoURL
	}

	key, err := h.upload.UploadProfilePhoto(r.Context(), user.ID, oldKey, file, header)
	if err != nil {
		sendAppError(w, err)
		return
	}

	updated, err := h.profileUpd.UpdateProfile(r.Context(), user.ID, map[string]interface{}{"photo_url": key})
	if err != nil {
		sendError(w, "Failed to update profile photo", "INTERNAL_ERROR", 500)
		return
	}

	sendSuccess(w, dto.ProfilePhotoUploadResponse{User: h.mapUserToDTO(updated)}, 201)
}
```

5. Add `"mime/multipart"` to the import block at the top of the file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run "TestUploadPhoto|TestMapUserToDTO" -v`
Expected: PASS — all 6 tests

- [ ] **Step 5: Verify all handler tests still compile**

Run: `go test ./internal/handler/ -v`
Expected: PASS — all tests (including match handler tests, which may fail at this point since `mapMatchRowToDTO` is still a package function — that's OK, will be fixed in Task 8)

Actually, the match handler tests call `mapMatchRowToDTO(mr)` as a package function. Those will still work since we haven't changed match_handler.go yet. So all tests should pass.

Run: `go test ./internal/handler/ -v`
Expected: PASS — all tests

- [ ] **Step 6: Commit**

```bash
git add internal/handler/auth_handler.go internal/handler/auth_handler_test.go
git commit -m "feat: implement profile photo upload with S3 + URL resolution"
```

---

## Task 8: MatchHandler — inject resolver, convert mapping helpers to methods

**Files:**
- Modify: `internal/handler/match_handler.go`
- Test: `internal/handler/match_handler_test.go`

- [ ] **Step 1: Write failing tests**

Update `internal/handler/match_handler_test.go`. Add a test resolver and convert function calls to method calls.

Add after the import block (line 9):

```go
func newTestMatchHandler() *MatchHandler {
	return &MatchHandler{images: testResolver}
}
```

Update `TestMapMatchRowToDTO` (line 47): change `mapMatchRowToDTO(mr)` to `h.mapMatchRowToDTO(mr)`:

```go
	h := newTestMatchHandler()
	resp := h.mapMatchRowToDTO(mr)
```

Add a new assertion for resolved image URLs inside `TestMapMatchRowToDTO` (after the existing `len(resp.Images) != 2` check):

```go
	if resp.Images[0] != "http://localhost:4566/smatch-matches/img1.jpg" {
		t.Errorf("Images[0] = %q, want resolved URL", resp.Images[0])
	}
```

Update `TestMapMatchRowToDTO_NilImages` (line 86): change to method call:

```go
	h := newTestMatchHandler()
	resp := h.mapMatchRowToDTO(mr)
```

Update `TestMapPlayerRowToDTO` (line 123): change to method call:

```go
	h := newTestMatchHandler()
	resp := h.mapPlayerRowToDTO(p)
```

Update the photo URL assertion (line 140) — the photo URL `https://example.com/photo.jpg` is a full URL so it should pass through tolerantly:

```go
	if resp.UserPhotoURL == nil || *resp.UserPhotoURL != "https://example.com/photo.jpg" {
		t.Errorf("UserPhotoURL = %v, want %q (tolerant passthrough)", resp.UserPhotoURL, photoURL)
	}
```

Update `TestMapPlayerRowToDTO_NilRespondedAt` (line 154): change to method call:

```go
	h := newTestMatchHandler()
	resp := h.mapPlayerRowToDTO(p)
```

Update `TestMapPlayersToDTO` (line 175): change to method call:

```go
	h := newTestMatchHandler()
	resp := h.mapPlayersToDTO(players)
```

`TestMapCurrentUserStatus` and `TestBuildDisplayName` — no changes needed (they stay as package-level functions).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestMap -v`
Expected: FAIL — `h.mapMatchRowToDTO undefined` (not yet a method)

- [ ] **Step 3: Update MatchHandler implementation**

In `internal/handler/match_handler.go`:

1. Add `images ImageURLResolver` field to the struct (line 18-22):

```go
type MatchHandler struct {
	matchRepo    *repository.MatchRepository
	redisService *service.RedisService
	hub          *ws.Hub
	images       ImageURLResolver
}

func NewMatchHandler(mr *repository.MatchRepository, rs *service.RedisService, hub *ws.Hub, images ImageURLResolver) *MatchHandler {
	return &MatchHandler{matchRepo: mr, redisService: rs, hub: hub, images: images}
}
```

2. Convert `mapMatchRowToDTO` to a method (line 616). Replace `Images: m.Images` with a resolved slice:

```go
func (h *MatchHandler) mapMatchRowToDTO(m *repository.MatchRow) dto.MatchResponse {
	resp := dto.MatchResponse{
		ID:            m.ID,
		CourtID:       m.CourtID,
		CourtName:     m.CourtName,
		CourtAddress:  m.CourtAddressStr,
		HostUserID:    m.HostUserID,
		HostName:      buildDisplayName(m.HostFirstName, m.HostLastName, m.HostUsername),
		Title:         m.Title,
		Description:   m.Description,
		Images:        h.resolveMatchImages(m.Images),
		SkillLevel:    string(m.SkillLevel),
		ShuttleType:   string(m.ShuttleType),
		PlayerFormat:  string(m.PlayerFormat),
		Date:          m.Date.Format("2006-01-02"),
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
		IsPrivate:     m.IsPrivate,
		Price:         m.Price,
		SlotsNeeded:   m.SlotsNeeded,
		SlotsAccepted: m.SlotsAccepted,
		Status:        string(m.Status),
		CreatedAt:     m.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:     m.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if resp.Images == nil {
		resp.Images = []string{}
	}
	return resp
}

func (h *MatchHandler) resolveMatchImages(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = h.images.Match(k)
	}
	return resolved
}
```

3. Convert `mapPlayersToDTO` to a method (line 647):

```go
func (h *MatchHandler) mapPlayersToDTO(players []*repository.MatchPlayerRow) []dto.MatchPlayerResponse {
	resp := make([]dto.MatchPlayerResponse, len(players))
	for i, p := range players {
		resp[i] = h.mapPlayerRowToDTO(p)
	}
	return resp
}
```

4. Convert `mapPlayerRowToDTO` to a method (line 655). Replace `UserPhotoURL: p.UserPhotoURL` with resolved value:

```go
func (h *MatchHandler) mapPlayerRowToDTO(p *repository.MatchPlayerRow) dto.MatchPlayerResponse {
	r := dto.MatchPlayerResponse{
		ID:           p.ID,
		MatchID:      p.MatchID,
		UserID:       p.UserID,
		UserName:     buildDisplayName(p.UserFirstName, p.UserLastName, p.UserUsername),
		UserPhotoURL: h.resolvePlayerPhotoURL(p.UserPhotoURL),
		Status:       string(p.Status),
		Message:      p.Message,
		Position:     p.Position,
		RequestedAt:  p.RequestedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	if p.RespondedAt != nil {
		s := p.RespondedAt.Format("2006-01-02T15:04:05.000Z")
		r.RespondedAt = &s
	}
	return r
}

func (h *MatchHandler) resolvePlayerPhotoURL(photoURL *string) *string {
	if photoURL == nil || *photoURL == "" {
		return photoURL
	}
	resolved := h.images.Profile(*photoURL)
	return &resolved
}
```

5. Update all call sites in `match_handler.go` to use `h.`:
   - Line 75: `resp[i] = h.mapMatchRowToDTO(m)`
   - Line 93: `resp[i] = h.mapMatchRowToDTO(m)`
   - Line 111: `resp[i] = h.mapMatchRowToDTO(m)`
   - Line 132: `MatchResponse: h.mapMatchRowToDTO(match),`
   - Line 133: `Players: h.mapPlayersToDTO(players),`
   - Line 191: `sendSuccess(w, h.mapMatchRowToDTO(created), 201)`
   - Line 261: `sendSuccess(w, h.mapMatchRowToDTO(updated), 200)`
   - Line 418: `sendSuccess(w, h.mapPlayerRowToDTO(player), 201)`
   - Line 495: `resp := h.mapPlayersToDTO(players)`
   - Line 611: `sendSuccess(w, h.mapPlayerRowToDTO(updated), 200)`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -v`
Expected: PASS — all tests

- [ ] **Step 5: Commit**

```bash
git add internal/handler/match_handler.go internal/handler/match_handler_test.go
git commit -m "feat: MatchHandler resolves S3 keys to public URLs in match/player DTOs"
```

---

## Task 9: Wire everything in cmd/server/main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Build the ImageURLResolver and update S3 init**

In `cmd/server/main.go`, after the S3 client init block (after line 97), add resolver construction. Also add `BucketBusinessDocs` to the `s3pkg.New` call and `EnsureBucketPublicRead` calls:

Replace lines 81-97 with:

```go
	// ── S3 ──────────────────────────────────────────────────────────────────
	s3Client, err := s3pkg.New(ctx, s3pkg.Config{
		Region:             cfg.AWS.Region,
		AccessKeyID:        cfg.AWS.AccessKeyID,
		SecretAccessKey:    cfg.AWS.SecretAccessKey,
		Endpoint:           cfg.AWS.Endpoint,
		BucketProfile:      cfg.AWS.BucketProfile,
		BucketMatches:      cfg.AWS.BucketMatches,
		BucketBusinessDocs: cfg.AWS.BucketBusinessDocs,
	})
	if err != nil {
		logger.Warn("s3 unavailable", zap.Error(err))
	}
	if s3Client != nil {
		if err := s3Client.EnsureBuckets(ctx, cfg.AWS.BucketProfile, cfg.AWS.BucketMatches); err != nil {
			logger.Warn("s3 bucket init failed", zap.Error(err))
		}
		if err := s3Client.EnsureBucketPublicRead(ctx, cfg.AWS.BucketProfile); err != nil {
			logger.Warn("s3 public-read policy failed for profile bucket", zap.Error(err))
		}
		if err := s3Client.EnsureBucketPublicRead(ctx, cfg.AWS.BucketMatches); err != nil {
			logger.Warn("s3 public-read policy failed for matches bucket", zap.Error(err))
		}
	}

	// ── Image URL Resolver ──────────────────────────────────────────────────
	matchesBase := cfg.AWS.PublicBaseURLMatches
	if matchesBase == "" {
		if cfg.AWS.Endpoint != "" {
			matchesBase = strings.TrimRight(cfg.AWS.Endpoint, "/") + "/" + cfg.AWS.BucketMatches
		} else {
			matchesBase = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", cfg.AWS.BucketMatches, cfg.AWS.Region)
		}
	}
	profileBase := cfg.AWS.PublicBaseURLProfile
	if profileBase == "" {
		if cfg.AWS.Endpoint != "" {
			profileBase = strings.TrimRight(cfg.AWS.Endpoint, "/") + "/" + cfg.AWS.BucketProfile
		} else {
			profileBase = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", cfg.AWS.BucketProfile, cfg.AWS.Region)
		}
	}
	imageResolver := handler.NewImageURLResolver(matchesBase, profileBase)
```

Add `"strings"` and `"fmt"` to the imports of `cmd/server/main.go` if not already present.

- [ ] **Step 2: Update handler constructors**

Replace lines 135-150 with:

```go
	// ── Handlers ────────────────────────────────────────────────────────────
	var matchUploadSvc *service.UploadService
	var profileUploadSvc *service.UploadService
	if s3Client != nil {
		matchUploadSvc = service.NewUploadService(s3Client, cfg.AWS.BucketMatches)
		profileUploadSvc = service.NewUploadService(s3Client, cfg.AWS.BucketProfile)
	}

	authH := handler.NewAuthHandler(fbClient, userRepo, availRepo, profileUploadSvc, imageResolver)
	courtH := handler.NewCourtHandler(courtRepo)
	availH := handler.NewAvailabilityHandler(availSvc, logger)
	matchH := handler.NewMatchHandler(matchRepo, redisSvc, hub, imageResolver)
	paymentH := handler.NewPaymentHandler(paymentRepo, availRepo, matchRepo, redisSvc, zaloClient, hub, logger,
		cfg.PaymentWSTicketTTLSec, cfg.Port, cfg.NodeEnv)
	searchH := handler.NewSearchHandler(redisSvc, searchRepo, courtRepo)
	proxyH := handler.NewProxyHandler(cfg.TileServerURL, cfg.TileLayerID)
	wsH := handler.NewWebSocketHandler(hub)
	loadTestH := handler.NewLoadTestHandler(cfg.LoadTestStressEnabled, cfg.AdminSecret)
	uploadH := handler.NewUploadHandler(matchUploadSvc, imageResolver)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/server/`
Expected: no errors

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS — all tests

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire ImageURLResolver and profile photo upload into server"
```

---

## Task 10: Add LocalStack to docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add localstack service**

In `docker-compose.yml`, add a `localstack` service after the `redis` service (before `pg_tileserv`):

```yaml
  localstack:
    image: localstack/localstack:3.4
    environment:
      - SERVICES=s3
      - LOCALSTACK_AUTH_TOKEN=${LOCALSTACK_AUTH_TOKEN:-}
    ports:
      - "4566:4566"
    networks:
      - smatch-net
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4566/_localstack/health"]
      interval: 5s
      timeout: 3s
      retries: 20
```

- [ ] **Step 2: Add AWS env vars to the backend service**

In the `backend` service's `environment` block (after line 59 `TILE_SERVER_URL=...`), add:

```yaml
      - AWS_REGION=ap-southeast-1
      - AWS_ACCESS_KEY_ID=test
      - AWS_SECRET_ACCESS_KEY=test
      - AWS_ENDPOINT=http://localstack:4566
      - AWS_S3_BUCKET_PROFILE=smatch-profiles
      - AWS_S3_BUCKET_MATCHES=smatch-matches
```

- [ ] **Step 3: Add localstack dependency to backend service**

In the `backend` service's `depends_on` block, add:

```yaml
      localstack:
        condition: service_healthy
```

- [ ] **Step 4: Verify docker-compose config is valid**

Run: `docker compose config --quiet`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml
git commit -m "feat: add LocalStack S3 service to docker-compose for local dev"
```

---

## Task 11: Final verification

- [ ] **Step 1: Run all tests**

Run: `go test ./...`
Expected: PASS — all tests

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Build the server**

Run: `go build ./cmd/server/`
Expected: no errors

- [ ] **Step 4: Manual local-dev smoke test (if LocalStack is available)**

```bash
# Start infra
docker compose up -d localstack postgres redis pg_tileserv

# Wait for localstack health
curl -s http://localhost:4566/_localstack/health | jq .

# Configure .env for local dev
export AWS_ENDPOINT=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=ap-southeast-1

# Run migrations
DATABASE_URL="postgresql://postgres:postgres@localhost:5433/smatch?sslmode=disable" bash infra/scripts/migrate.sh

# Start server
go run ./cmd/server
```

In another terminal, verify buckets and policies:

```bash
awslocal s3api list-buckets
awslocal s3api get-bucket-policy --bucket smatch-matches
```

- [ ] **Step 5: Final commit (if any stray changes)**

```bash
git status
git add -A
git commit -m "chore: final verification pass for S3 image retrieval"
```

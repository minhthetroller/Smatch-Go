# Fix Match Image Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the match-image upload endpoint so `POST /api/uploads/match-image` works end-to-end, eliminating the frontend "No internet connection after 3 attempts" error.

**Architecture:** Three-layer fix — add `UploadMatchImage` to `UploadService` (service layer), wire `UploadService` + `UploadHandler` in `main.go` (wiring layer), register the missing route (routing layer). Extract a minimal `s3Uploader` interface inside the service package to keep the service unit-testable without hitting real S3.

**Tech Stack:** Go 1.23, chi router, AWS SDK v2 S3, pgx/v5 (unchanged), `go.uber.org/zap`

---

## Root Cause Analysis

Three problems compound to produce the 404 that the frontend misreads as "no internet":

1. **`UploadService` missing `UploadMatchImage` method** — the handler's `matchImageUploader` interface requires it, but the service only has `UploadDocument`.
2. **`UploadHandler` never instantiated** in `cmd/server/main.go` — `s3Client` is discarded with `_ = s3Client` (line 140).
3. **Route `POST /api/uploads/match-image` never registered** — chi returns 404, frontend retries 3×, shows "No internet connection".

---

## File Structure

| File | Change |
|---|---|
| `internal/service/upload_service.go` | Extract `s3Uploader` interface; add `UploadMatchImage` method |
| `internal/service/upload_service_test.go` | New: unit tests for `UploadMatchImage` |
| `cmd/server/main.go` | Wire `UploadService` + `UploadHandler`; register route |

---

### Task 1: Add `UploadMatchImage` to `UploadService` with testable interface

**Files:**
- Modify: `internal/service/upload_service.go`

- [ ] **Step 1: Write the failing test first**

Create `internal/service/upload_service_test.go`:

```go
package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

// fakeFile implements multipart.File using an in-memory buffer.
type fakeFile struct{ *bytes.Reader }

func (f *fakeFile) Close() error                                      { return nil }
func (f *fakeFile) ReadAt(p []byte, off int64) (n int, err error)     { return f.Reader.ReadAt(p, off) }

func newFakeFile(data string) multipart.File {
	return &fakeFile{bytes.NewReader([]byte(data))}
}

// stubS3Uploader records calls and returns a configurable error.
type stubS3Uploader struct {
	err    error
	gotKey string
}

func (s *stubS3Uploader) PutObject(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.err
}

func TestUploadMatchImage_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := &UploadService{s3: stub, bucket: "smatch-matches"}

	header := &multipart.FileHeader{Filename: "photo.jpg"}
	url, err := svc.UploadMatchImage(context.Background(), newFakeFile("img-data"), header)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://smatch-matches.s3.amazonaws.com/matches/") {
		t.Fatalf("unexpected url: %s", url)
	}
	if !strings.HasPrefix(stub.gotKey, "matches/") {
		t.Fatalf("expected key under matches/, got %s", stub.gotKey)
	}
}

func TestUploadMatchImage_InvalidExtension(t *testing.T) {
	svc := &UploadService{s3: &stubS3Uploader{}, bucket: "smatch-matches"}
	header := &multipart.FileHeader{Filename: "doc.pdf"}
	_, err := svc.UploadMatchImage(context.Background(), newFakeFile("data"), header)
	if err == nil {
		t.Fatal("expected error for pdf extension")
	}
}

func TestUploadMatchImage_S3Error(t *testing.T) {
	stub := &stubS3Uploader{err: errors.New("s3 down")}
	svc := &UploadService{s3: stub, bucket: "smatch-matches"}
	header := &multipart.FileHeader{Filename: "photo.png"}
	_, err := svc.UploadMatchImage(context.Background(), newFakeFile("img-data"), header)
	if err == nil {
		t.Fatal("expected error on s3 failure")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /path/to/smatch-backend-go
go test ./internal/service/ -run TestUploadMatchImage -v
```

Expected: `FAIL` — `UploadMatchImage undefined`, `s3Uploader undefined`, `UploadService.s3 field type mismatch`.

- [ ] **Step 3: Update `upload_service.go` — extract interface and add method**

Replace entire `internal/service/upload_service.go` with:

```go
package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/smatch/badminton-backend/internal/domain"
)

type s3Uploader interface {
	PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
	PutObjectEncrypted(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
}

type UploadService struct {
	s3     s3Uploader
	bucket string
}

func NewUploadService(s3Client s3Uploader, bucket string) *UploadService {
	return &UploadService{s3: s3Client, bucket: bucket}
}

var allowedDocExts = map[string]string{
	".pdf":  "application/pdf",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

var allowedImageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

func (s *UploadService) UploadDocument(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := allowedDocExts[ext]
	if !ok {
		return "", domain.BadRequest("Invalid file type. Allowed: pdf, jpg, jpeg, png")
	}

	key := fmt.Sprintf("%s/%s-%d%s", folder, uuid.New().String(), time.Now().Unix(), ext)

	if err := s.s3.PutObjectEncrypted(ctx, s.bucket, key, file, contentType); err != nil {
		return "", &domain.AppError{Code: "UPLOAD_FAILED", Message: "Failed to upload document", Status: 500, Err: err}
	}

	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key), nil
}

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

	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/service/ -run TestUploadMatchImage -v
```

Expected: `PASS` — all 3 test cases green.

- [ ] **Step 5: Run full service test suite to check no regressions**

```bash
go test ./internal/service/... -v
```

Expected: all existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/service/upload_service.go internal/service/upload_service_test.go
git commit -m "feat: add UploadMatchImage to UploadService with testable s3Uploader interface"
```

---

### Task 2: Wire UploadHandler and register route in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Replace the dead `_ = s3Client` block and add upload handler wiring**

In `cmd/server/main.go`, find lines 138–140:
```go
	// S3 available for future image upload use
	_ = s3Client
```

Replace with:
```go
	var uploadSvc *service.UploadService
	if s3Client != nil {
		uploadSvc = service.NewUploadService(s3Client, cfg.AWS.BucketMatches)
	}
	uploadH := handler.NewUploadHandler(uploadSvc)
```

Note: `handler.NewUploadHandler` accepts `matchImageUploader` (an interface). Since `*service.UploadService` implements it, and `nil` is handled in the handler (returns 503), this is safe whether or not S3 is available.

- [ ] **Step 2: Register the upload route**

In `cmd/server/main.go`, inside the `/api` router block, add a new route group after the `// ── Search` block (around line 244):

```go
		// ── Uploads ───────────────────────────────────────────────────────────
		r.Route("/uploads", func(r chi.Router) {
			r.With(authMw.RequireRegisteredUser).Post("/match-image", uploadH.UploadMatchImage)
		})
```

- [ ] **Step 3: Build to verify compilation**

```bash
go build ./cmd/server/
```

Expected: no errors.

- [ ] **Step 4: Run the full test suite**

```bash
go test ./...
```

Expected: all tests pass (including `TestUploadHandler_*` in `internal/handler/upload_handler_test.go`).

- [ ] **Step 5: Smoke test the endpoint locally**

Start services:
```bash
docker-compose up postgres redis -d
```

Run server:
```bash
go run ./cmd/server/
```

In a second terminal (replace `<TOKEN>` with a valid Firebase JWT):
```bash
curl -X POST http://localhost:3000/api/uploads/match-image \
  -H "Authorization: Bearer <TOKEN>" \
  -F "image=@/path/to/test.jpg" \
  -v
```

Expected without S3 configured: `503 {"error":"Image upload not available","code":"UPLOAD_UNAVAILABLE"}`  
Expected with S3/LocalStack configured: `201 {"success":true,"data":{"url":"https://...","fileName":"test.jpg"}}`

Previously the route returned `404` — any non-404 confirms the route is now registered.

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire UploadHandler and register POST /api/uploads/match-image route"
```

---

## Self-Review

**Spec coverage:**
- ✅ Route `POST /api/uploads/match-image` now registered
- ✅ `UploadService.UploadMatchImage` implemented and satisfies the `matchImageUploader` interface
- ✅ S3 unavailability handled gracefully (503 instead of panic)
- ✅ Auth guard (`RequireRegisteredUser`) on upload route
- ✅ Unit tests for all three `UploadMatchImage` paths (success, bad ext, S3 error)

**Placeholder scan:** None found.

**Type consistency:**
- `s3Uploader` interface used in `upload_service.go` and `upload_service_test.go` — consistent.
- `NewUploadService` signature changed from `*s3.Client` to `s3Uploader` — `*s3pkg.Client` satisfies the interface (both `PutObject` and `PutObjectEncrypted` methods exist on it).
- `handler.NewUploadHandler(uploadSvc)` — `uploadSvc` is `*service.UploadService` which has `UploadMatchImage`, satisfying `matchImageUploader`. Nil case handled.

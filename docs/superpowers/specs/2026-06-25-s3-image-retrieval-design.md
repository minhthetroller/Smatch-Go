# S3 Image Upload & Retrieval — Design

**Date:** 2026-06-25
**Status:** Approved
**Scope:** LocalStack-first implementation of real S3 file retrieval for profile photos and match images, with image URLs returned inside the matches-list and profile responses.

## Context

The smatch-backend-go codebase already has an S3 client (`platform/s3/s3.go`, AWS SDK v2) wired into an `UploadService`, and a working match-image upload endpoint (`POST /api/uploads/match-image`). However:

- The `UploadService` returns a hardcoded `https://{bucket}.s3.amazonaws.com/{key}` URL that breaks in LocalStack and would need rewriting if a CDN is ever added.
- Profile photo upload (`POST /api/auth/me/photo`) is a 501 stub, despite the route, DTO (`ProfilePhotoUploadResponse`), and DB column (`users.photo_url`) all existing.
- The buckets have no public-read policy, so retrieval of stored objects is not possible.
- `docker-compose.yml` has no LocalStack service, making local S3 testing impossible without the heavyweight Terraform bootstrap.

## Decisions

1. **Privacy:** Buckets are public-read. Match images are browsable via the public `GET /api/matches` endpoint (`OptionalAuth`), so public URLs are appropriate. Profile photos are also public-read for simplicity and speed. No presigned URLs.
2. **DB storage format:** Store the S3 **key** only (e.g. `matches/abc-123.jpg`) in `matches.images` and `users.photo_url`. Build the full public URL at response time via a configurable base. Decouples data from environment/CDN.
3. **LocalStack dev setup:** Add a `localstack` service to `docker-compose.yml` (S3 only). Buckets + public-read policies are applied by the Go server's `EnsureBuckets`/`EnsureBucketPublicRead` on startup. No separate bootstrap script.
4. **URL-builder architecture:** Approach A — inject an `ImageURLResolver` into handlers. URL building is a presentation concern and belongs in the handler/DTO-mapping layer.

## Architecture

```
Client
  1. POST /api/uploads/match-image  ->  { key, url, fileName }
  2. POST /api/matches  { images: ["matches/abc.jpg"] }
  3. GET  /api/matches   ->  { images: ["http://localhost:4566/smatch-matches/matches/abc.jpg"] }
  4. POST /api/auth/me/photo  ->  { user: { photoUrl: "<resolved>" } }
  5. GET  /api/auth/me         ->  { photoUrl: "<resolved>" }

Handler (holds ImageURLResolver)
  - mapXxxToDTO() resolve keys -> URLs at response time
  - uploads return key; handler builds url via resolver

Repository (raw keys)
  - matches.images TEXT[], users.photo_url TEXT store keys

UploadService
  - UploadMatchImage / UploadProfilePhoto -> return key only
  - UploadProfilePhoto deletes previous photo key after successful upload

platform/s3.Client
  - PutObject / DeleteObject / EnsureBuckets / EnsureBucketPublicRead (NEW)
  - LocalStack: endpoint http://localhost:4566, path-style
  - Real AWS: standard endpoint, public-read bucket policy
```

## Components

### 1. `ImageURLResolver` (new: `internal/handler/imageurl.go`)

Same package as handlers (co-located with mapping helpers, no import cycle).

```go
type ImageURLResolver struct {
    matchesBase string
    profileBase string
}

func NewImageURLResolver(matchesBase, profileBase string) ImageURLResolver

// Tolerant: if value already looks like a full URL (http://, https://), returns as-is.
// This avoids needing a data migration for existing rows that hold full URLs.
func (r ImageURLResolver) Match(key string) string
func (r ImageURLResolver) Profile(key string) string
```

- Empty key -> empty string (no base prepended).
- Trailing-slash normalization on base.

### 2. Config additions (`internal/config/config.go`)

Two optional env vars:
- `AWS_S3_PUBLIC_BASE_URL_MATCHES` (default: derived)
- `AWS_S3_PUBLIC_BASE_URL_PROFILE` (default: derived)

**Default derivation** (in `cmd/server/main.go` at wiring time when env var empty):
- If `AWS_ENDPOINT` set (LocalStack): `{endpoint}/{bucket}` -> `http://localhost:4566/smatch-matches`
- Else (real AWS): `https://{bucket}.s3.{region}.amazonaws.com`

The env vars are the future hook for CloudFront: set them to the CDN URL without touching code or data.

### 3. S3 public-read setup (`platform/s3/s3.go`)

New method:
```go
func (c *Client) EnsureBucketPublicRead(ctx context.Context, bucket string) error
```
Puts a bucket policy granting `s3:GetObject` to `*` on `arn:aws:s3:::{bucket}/*`. Called from `cmd/server/main.go` right after `EnsureBuckets` for the profile and matches buckets. LocalStack honors S3 bucket policies, so the same code works locally and in prod.

### 4. UploadService contract change (`internal/service/upload_service.go`)

- `UploadMatchImage` returns the **key** only (e.g. `matches/abc-123.jpg`), not a URL.
- New `UploadProfilePhoto(ctx, userID, oldKey string, file multipart.File, header *multipart.FileHeader) (string, error)`:
  - Validates ext (`jpg/jpeg/png`).
  - Key format: `profile/{userID}/{uuid}-{ts}.jpg` (per-user namespace).
  - Calls `s.s3.PutObject` (no SSE - public image).
  - After successful upload, if `oldKey != ""`, calls `s.s3.DeleteObject(ctx, s.bucket, oldKey)` (best-effort; log errors, don't fail the upload).
- `UploadDocument` (business docs) is left untouched - out of scope.
- `s3Uploader` interface gains `DeleteObject`.

### 5. Handler changes

**`AuthHandler`** (`internal/handler/auth_handler.go`):
- New dependencies: `profilePhotoUploader` interface + `ImageURLResolver`.
- Constructor: `NewAuthHandler(fb, userRepo, availRepo, upload profilePhotoUploader, images ImageURLResolver)`.
- `UploadPhoto` (replaces 501 stub):
  1. `user := middleware.UserFromContext(ctx)`
  2. If `h.upload == nil` -> 503 `UPLOAD_UNAVAILABLE`.
  3. `ParseMultipartForm(5MB)`; `FormFile("image")`.
  4. `oldKey := ""`; if `user.PhotoURL != nil`, `oldKey = *user.PhotoURL`.
  5. `key, err := h.upload.UploadProfilePhoto(ctx, user.ID, oldKey, file, header)`.
  6. `user, err := h.userRepo.UpdateProfile(ctx, user.ID, {"photo_url": key})`.
  7. `sendSuccess(w, ProfilePhotoUploadResponse{User: h.mapUserToDTO(user)}, 201)`.
- `mapUserToDTO` becomes a method `(h *AuthHandler) mapUserToDTO(u)`. Resolves `u.PhotoURL` via `h.images.Profile(...)` if non-nil. Affects `Me`, `UpdateMe`, `Verify`.

**`MatchHandler`** (`internal/handler/match_handler.go`):
- New dependency: `ImageURLResolver`.
- Constructor: `NewMatchHandler(mr, rs, hub, images ImageURLResolver)`.
- `mapMatchRowToDTO` becomes method `(h *MatchHandler)`. Resolves each `m.Images` element via `h.images.Match(key)`. Keep `[]string{}` non-nil default.
- `mapPlayerRowToDTO` becomes method. Resolves `p.UserPhotoURL` via `h.images.Profile(...)` if non-nil.
- `mapPlayersToDTO` becomes method (calls `h.mapPlayerRowToDTO`).
- `mapCurrentUserStatus` - no image fields, stays package-level.
- Affected call sites: `GetAllMatches`, `GetHostedMatches`, `GetJoinedMatches`, `GetMatch`, `RespondToJoinRequest`.

**`UploadHandler`** (`internal/handler/upload_handler.go`):
- New dependency: `ImageURLResolver`.
- Constructor: `NewUploadHandler(upload matchImageUploader, images ImageURLResolver)`.
- `UploadMatchImage`: service returns key; handler builds `url` via `images.Match(key)`. Response: `{ key, url, fileName }`.

### 6. DTO changes (`internal/dto/upload_dto.go`)

`ImageUploadResponse` gains a `Key` field:
```go
type ImageUploadResponse struct {
    Key      string `json:"key"`
    URL      string `json:"url"`
    FileName string `json:"fileName"`
}
```
`ProfilePhotoUploadResponse` - no change (already wraps `UserProfileResponse`).

### 7. `cmd/server/main.go` wiring

- Build `ImageURLResolver` from config (env override or derived base URLs).
- Pass resolver into `NewAuthHandler`, `NewMatchHandler`, `NewUploadHandler`.
- Pass `uploadSvc` (profile-photo uploader) into `NewAuthHandler`.
- After `EnsureBuckets`, call `EnsureBucketPublicRead` for profile + matches buckets.
- Pass `BucketBusinessDocs` to `s3pkg.New` for consistency (pre-existing omission).

### 8. docker-compose.yml: `localstack` service

```yaml
  localstack:
    image: localstack/localstack:3.4
    environment:
      - SERVICES=s3
      - LOCALSTACK_AUTH_TOKEN=${LOCALSTACK_AUTH_TOKEN}
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
The `backend` service gets `AWS_ENDPOINT=http://localstack:4566` + bucket env vars so the containerized backend reaches LocalStack by service name. (When running `go run ./cmd/server` on the host, use `AWS_ENDPOINT=http://localhost:4566` in `.env`.)

Buckets + public-read policies are applied by the Go server on startup, so `docker-compose up` + `go run ./cmd/server` just works.

## Existing-data Migration

**No migration required.** The tolerant `resolve()` returns full URLs as-is, so any existing rows holding `https://...` URLs keep working. New uploads store bare keys. Both resolve correctly. A future one-time backfill SQL could strip known prefixes to keys, but that is optional and out of scope.

## Endpoint Contract Changes (Frontend Impact)

| Endpoint | Before | After |
|---|---|---|
| `POST /api/uploads/match-image` | `{ url, fileName }` | `{ key, url, fileName }` - frontend stores `key` in match payloads |
| `POST /api/auth/me/photo` | 501 | `201 { user: UserProfileResponse }` - `user.photoUrl` is resolved URL |
| `GET /api/matches` | `images: ["https://..."]` | `images: ["<resolved>"]` |
| `GET /api/auth/me` | `photoUrl: "https://..."` | `photoUrl: "<resolved>"` |
| `POST /api/matches` | accepts `images: ["https://..."]` | accepts `images: ["matches/abc.jpg"]` (keys); legacy full URLs still work via tolerant resolver |

Soft migration for frontend: legacy full-URL payloads round-trip unchanged.

## Testing

### Unit tests (pure, no DB/S3 - `go test ./...`)

- **`internal/handler/imageurl_test.go`** (new): resolver prepend, tolerant passthrough, empty key, profile vs matches independence, trailing-slash normalization.
- **`internal/handler/upload_handler_test.go`** (update): constructor takes resolver; stub returns key; assert `Data.Key` and `Data.URL`.
- **`internal/handler/match_handler_test.go`** (update): `mapMatchRowToDTO`/`mapPlayerRowToDTO` are methods; assert resolved URLs; add tolerant passthrough case.
- **`internal/handler/auth_handler_test.go`** (new): `TestUploadPhoto_NilUploader` (503), `TestUploadPhoto_Success` (201, resolved `photoUrl`, `UpdateProfile` called with key), `TestUploadPhoto_ReplacesOldPhoto` (old key deleted).
- **`internal/service/upload_service_test.go`** (update): assert `UploadMatchImage` returns key not URL; add `TestUploadProfilePhoto_Success` and `TestUploadProfilePhoto_DeletesOldPhoto`.

### Local-dev verification (manual)

1. `docker-compose up -d localstack postgres redis pg_tileserv`
2. `cp .env.example .env`; set `AWS_ENDPOINT=http://localhost:4566`, `AWS_ACCESS_KEY_ID=test`, `AWS_SECRET_ACCESS_KEY=test`.
3. `go run ./cmd/server` - logs show buckets created + public-read policies applied.
4. `awslocal s3api get-bucket-policy smatch-matches` - shows `s3:GetObject` allow-all.
5. `go test ./...` - all unit tests pass.
6. curl flow: upload match image -> `key` + `url`; `curl {url}` returns bytes; upload profile photo -> `201` with resolved `photoUrl`; `GET /api/auth/me` -> resolved; create match with keys -> `GET /api/matches` -> resolved URLs.
7. Repeat from `backend` container to confirm `http://localstack:4566` routing inside Docker network.

## Out of Scope (Follow-ups)

- Terraform `aws_s3_bucket_policy` + `public_access_block` for `profile`/`matches` in `infra/terraform/s3.tf` (prod parity).
- Fixing `BucketBusinessDocs` not passed to server's `s3pkg.New` (pre-existing).
- Backfilling existing prod rows from full URLs to keys (optional; tolerant resolver makes it non-urgent).
- CloudFront in front of S3 (future; `AWS_S3_PUBLIC_BASE_URL_*` env vars are the hook).

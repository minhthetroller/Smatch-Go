# Image Upload & Retrieval Protocol

> Frontend integration guide for profile photos and match images.
> Supersedes the former `MATCH_IMAGE_UPLOAD.md`.

## What Changed (Frontend Changelog)

- **Match image upload response** now returns `{ key, url, fileName }` — was `{ url, fileName }`.
- **Store the `key`** (not the URL) in `match.images[]` when creating/updating matches.
- **New endpoint: `POST /api/auth/me/photo`** — upload profile photo (was a 501 stub).
- **Image URLs in API responses are resolved by the backend** — `GET /api/matches`, `GET /api/matches/{id}`, and `GET /api/auth/me` return fully-resolved public URLs. Just render them.
- **Buckets are public-read** — image URLs are permanent and cacheable. No auth token, no proxy needed to fetch an image.
- **Legacy full-URL payloads still work** — if you send a full URL in `match.images[]`, it round-trips unchanged. Soft migration.

## Upload Endpoints

### `POST /api/uploads/match-image`

```
POST /api/uploads/match-image
Authorization: Bearer <firebase-jwt>
Content-Type: multipart/form-data
```

| Rule | Value |
|---|---|
| Field name | `image` |
| Allowed types | `.jpg`, `.jpeg`, `.png` |
| Max file size | 5 MB |
| Auth | Firebase JWT (`RequireRegisteredUser`) |

**Response `201`:**

```json
{
  "success": true,
  "data": {
    "key": "matches/<uuid>-<ts>.jpg",
    "url": "http://localhost:4566/smatch-matches/matches/<uuid>-<ts>.jpg",
    "fileName": "photo.jpg"
  }
}
```

### `POST /api/auth/me/photo`

```
POST /api/auth/me/photo
Authorization: Bearer <firebase-jwt>
Content-Type: multipart/form-data
```

Same constraints as above (field `image`, 5 MB, jpg/jpeg/png). Replaces any existing profile photo — the old image is deleted from S3 automatically.

**Response `201`:**

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "...",
      "photoUrl": "http://localhost:4566/smatch-profiles/profile/<userId>/<uuid>-<ts>.jpg",
      "...": "..."
    }
  }
}
```

No key handling needed for profile photos — the backend persists the key to `users.photo_url` and returns the resolved URL inside the `user` object.

## Storage Rule (Critical)

| Payload field | Store what? | Example |
|---|---|---|
| `POST /api/matches` → `images[]` | **`key`** from upload response | `["matches/abc-123.jpg"]` |
| `PUT /api/matches/{id}` → `images[]` | **`key`** for each image | `["matches/abc-123.jpg"]` |
| Profile photo | Nothing — backend handles it | N/A |

Never construct URLs yourself. Never store the `url` field in create/update payloads — it is environment-specific and will break when you move between LocalStack and production.

## Retrieval (Read Paths)

The backend resolves keys to public URLs at response time. Frontend just renders them.

| Endpoint | Field | Contains |
|---|---|---|
| `GET /api/matches` | `data[].images[]` | Resolved public URLs |
| `GET /api/matches/{id}` | `data.images[]` | Resolved public URLs |
| `GET /api/matches/{id}` | `data.players[].userPhotoUrl` | Resolved public URL (nullable) |
| `GET /api/auth/me` | `data.photoUrl` | Resolved public URL (nullable) |

```tsx
<img src={match.images[0]} alt="match" />   // works — public, no token
<img src={user.photoUrl ?? fallback} />     // works — public, no token
```

## TypeScript Examples

### Upload match image

```typescript
interface MatchImageUploadResponse {
  key: string;       // store this in match.images[]
  url: string;       // use for immediate preview only
  fileName: string;
}

async function uploadMatchImage(file: File, token: string): Promise<MatchImageUploadResponse> {
  const form = new FormData();
  form.append("image", file);

  const res = await fetch("/api/uploads/match-image", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message ?? "Upload failed");
  }

  const data = await res.json();
  return data.data;  // { key, url, fileName }
}
```

### Upload profile photo

```typescript
interface User {
  id: string;
  photoUrl: string | null;
  /* ...other fields */
}

async function uploadProfilePhoto(file: File, token: string): Promise<User> {
  const form = new FormData();
  form.append("image", file);

  const res = await fetch("/api/auth/me/photo", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message ?? "Upload failed");
  }

  const data = await res.json();
  return data.data.user;  // updated user with resolved photoUrl
}
```

### Create match with images

```typescript
// Step 1 — upload images, collect KEYS (not URLs)
const uploaded = await Promise.all(files.map(f => uploadMatchImage(f, token)));
const keys = uploaded.map(u => u.key);

// Step 2 — create match with keys
await fetch("/api/matches", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    ...matchFields,
    images: keys,   // <-- keys, not URLs
  }),
});
```

### Update match images

```typescript
await fetch(`/api/matches/${matchId}`, {
  method: "PUT",
  headers: {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    images: retainedKeys,  // <-- keys of images to keep
  }),
});
```

## Error Responses

| Status | Code | Cause |
|---|---|---|
| `400` | `BAD_REQUEST` | No file / invalid form data / file too large |
| `400` | `BAD_REQUEST` | Unsupported extension (pdf, gif, etc.) |
| `401` | — | Missing or invalid Firebase JWT |
| `503` | `UPLOAD_UNAVAILABLE` | S3 not configured server-side |
| `500` | `UPLOAD_FAILED` | S3 write error |

## Environment Notes

| Environment | Image URL base | Notes |
|---|---|---|
| Local dev (LocalStack) | `http://localhost:4566/{bucket}/{key}` | Buckets created + public-read policy applied on server startup |
| Production (S3) | `https://{bucket}.s3.{region}.amazonaws.com/{key}` | Override via `AWS_S3_PUBLIC_BASE_URL_*` env vars |
| Production (CloudFront) | `https://cdn.smatch.app/{key}` | Future — set the env vars, no code/frontend change needed |

Treat image URLs as opaque strings. Do not parse them to detect environment or extract keys — use the `key` field from the upload response for storage and the `url`/resolved field from API responses for display.

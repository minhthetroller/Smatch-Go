# Smatch-Go

Smatch-Go is the backend service for a badminton court booking and match-making platform. It powers court discovery, nearby search, booking management, match creation and joining, payment processing, and authenticated user workflows for a connected sports community.

## Overview

The service is written in Go and follows a layered architecture with HTTP handlers, services, repositories, and platform-specific integrations. It exposes a JSON REST API, WebSocket endpoints for real-time updates, and a proxy endpoint for map tiles.

The application starts by loading configuration, connecting to PostgreSQL, optionally connecting to Redis, initializing Firebase authentication, setting up ZaloPay and S3 clients, and then serving the API over HTTP.

## Key Features

- Court listing, detail retrieval, creation, update, and deletion
- Nearby court search using PostgreSQL/PostGIS geospatial queries
- Booking creation, lookup, and cancellation
- Match creation, update, cancellation, join, leave, and join-request management
- Payment creation, callback handling, status lookup, and cancellation
- Firebase-based authentication and user profile management
- WebSocket-based real-time updates for payments and matches
- Admin-only endpoints for court management and search reindexing
- Redis-backed slot locking, caching, and rate-limited endpoints when Redis is available
- Map tile proxy support for court map visualization

## Tech Stack

- Go 1.23
- chi router
- PostgreSQL with PostGIS
- Redis
- Firebase Admin SDK
- ZaloPay integration
- AWS S3
- Gorilla WebSocket
- Zap logging
- robfig/cron background jobs

## Architecture

The codebase is organized into a small number of focused layers:

- `cmd/server` boots the application and wires dependencies together.
- `internal/config` loads environment variables and default values.
- `internal/handler` contains HTTP handlers for auth, courts, bookings, matches, payments, search, proxy, and WebSockets.
- `internal/service` contains business logic such as availability handling, Redis helpers, and scheduled background jobs.
- `internal/repository` contains database access logic.
- `internal/middleware` handles authentication and authorization.
- `internal/domain` contains domain entities and enums.
- `internal/dto` contains request and response models.
- `internal/websocket` contains the WebSocket hub and channel management.
- `platform` contains integration clients for PostgreSQL, Redis, Firebase, S3, and ZaloPay.
- `migrations` contains the SQL schema files.

## Project Structure

- `cmd/server` - application entry point
- `internal/config` - configuration loading and defaults
- `internal/domain` - core domain models and enums
- `internal/dto` - request and response payloads
- `internal/handler` - HTTP handlers and API endpoints
- `internal/middleware` - auth and access control middleware
- `internal/repository` - data access layer
- `internal/service` - business logic and scheduled jobs
- `internal/websocket` - real-time hub and WebSocket handling
- `platform` - external service clients and integrations
- `migrations` - SQL schema migrations

## Prerequisites

Before running the service, make sure you have:

- Go 1.23 or later
- PostgreSQL with PostGIS enabled
- Redis, if you want caching and Redis-based rate limiting
- A Firebase service account JSON file
- ZaloPay merchant credentials
- Optional AWS credentials for S3 support
- Optional tile server for map proxy requests

## Configuration

Create a `.env` file in the project root and set the required values for your environment.

### Required or commonly used variables

| Variable | Description | Default |
| --- | --- | --- |
| `PORT` | HTTP server port | `3000` |
| `NODE_ENV` | Application environment | `development` |
| `DATABASE_URL` | PostgreSQL connection string | none |
| `FIREBASE_CREDENTIALS_FILE` | Path to the Firebase Admin SDK JSON file | `smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json` |
| `ADMIN_SECRET` | Shared secret for admin-protected routes | none |
| `TILE_SERVER_URL` | Map tile server URL | `http://localhost:7800` |
| `TILE_LAYER_ID` | Tile layer name | `public.courts` |
| `SLOT_LOCK_TTL_SECONDS` | Slot lock lifetime in seconds | `600` |

### Redis

| Variable | Description | Default |
| --- | --- | --- |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | empty |
| `REDIS_TLS_ENABLED` | Enable TLS for Redis | `false` |

### ZaloPay

| Variable | Description | Default |
| --- | --- | --- |
| `ZALOPAY_APP_ID` | ZaloPay application ID | none |
| `ZALOPAY_KEY1` | ZaloPay secret key 1 | none |
| `ZALOPAY_KEY2` | ZaloPay secret key 2 | none |
| `ZALOPAY_ENDPOINT` | ZaloPay API endpoint | `https://sb-openapi.zalopay.vn` |
| `ZALOPAY_CALLBACK_URL` | Callback URL for payment notifications | none |

### AWS and S3

| Variable | Description | Default |
| --- | --- | --- |
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | AWS access key ID | `test` |
| `AWS_SECRET_ACCESS_KEY` | AWS secret access key | `test` |
| `AWS_ENDPOINT` | Optional S3-compatible endpoint | empty |
| `AWS_S3_BUCKET_PROFILE` | Bucket for profile images | `smatch-profiles` |
| `AWS_S3_BUCKET_MATCHES` | Bucket for match images | `smatch-matches` |

## Run Locally

1. Clone the repository and enter the project folder.
2. Install Go dependencies.
3. Prepare PostgreSQL with PostGIS and create the database referenced by `DATABASE_URL`.
4. Apply the SQL files in `migrations/` using your preferred PostgreSQL migration tool or client.
5. Create a `.env` file with the configuration values above.
6. Start the server.

Example commands:

```bash
go mod download
go run ./cmd/server
```

The server listens on the port defined by `PORT`, which defaults to `3000`.

## Run with Docker

The repository includes a multi-stage Dockerfile.

```bash
docker build -t smatch-go .
docker run --rm -p 3000:3000 --env-file .env smatch-go
```

Make sure the container can reach PostgreSQL, Redis, Firebase credentials, and any other external services it depends on.

## API Highlights

### Health and metadata

- `GET /health` - health check
- `GET /version` - version information

### Authentication

- `POST /api/auth/verify`
- `POST /api/auth/anonymous`
- `GET /api/auth/me`
- `PUT /api/auth/me`
- `DELETE /api/auth/me`
- `GET /api/auth/me/bookings`

### Courts and availability

- `GET /api/courts`
- `GET /api/courts/nearby`
- `GET /api/courts/{id}`
- `GET /api/courts/{courtId}/availability`
- `POST /api/courts` for admins
- `PUT /api/courts/{id}` for admins
- `DELETE /api/courts/{id}` for admins

### Bookings and payments

- `POST /api/bookings`
- `GET /api/bookings/{id}`
- `DELETE /api/bookings/{id}`
- `GET /api/bookings/{id}/payment`
- `POST /api/payments/create`
- `POST /api/payments/callback`
- `GET /api/payments/{id}`
- `GET /api/payments/{id}/status`
- `POST /api/payments/{id}/cancel`

### Matches

- `GET /api/matches`
- `GET /api/matches/{id}`
- `POST /api/matches`
- `PUT /api/matches/{id}`
- `DELETE /api/matches/{id}`
- `POST /api/matches/{id}/join`
- `DELETE /api/matches/{id}/leave`
- `GET /api/matches/{id}/requests`
- `PUT /api/matches/{id}/requests/{playerId}/respond`

### Search and admin

- `GET /api/search/autocomplete`
- `GET /api/search/courts`
- `GET /api/search/popular`
- `POST /api/admin/search/reindex`
- `GET /api/admin/search/stats`

### WebSockets and map tiles

- `GET /ws/payments`
- `GET /ws/matches`
- `GET /map-tiles/{z}/{x}/{y}.pbf`

## Testing

The repository includes Go tests under `internal/handler` and `internal/service`. Run the full suite with:

```bash
go test ./...
```

## Deployment Notes

- The server uses graceful shutdown on SIGINT and SIGTERM.
- Redis is optional, but features such as caching, rate limiting, and some match/payment flows work best when Redis is available.
- PostgreSQL should be provisioned with the PostGIS extension enabled before the application starts.
- The Docker image exposes port `3000`.
- Production deployments should provide a valid Firebase service account file and real payment credentials.

## Notes

- The application reads a `.env` file automatically when present.
- If Redis is unavailable, the application can still start, but some features degrade gracefully.
- The S3 client is initialized in the server bootstrap and is currently reserved for media-related features.

## License

This repository does not currently include an explicit license file. Add one if the project is intended for public distribution.
# smatch-backend-go

Badminton court booking platform backend. Go 1.23, chi router, PostgreSQL (pgx/v5), Redis, Firebase Auth, ZaloPay, Azure Blob Storage, pg_tileserv.

## Architecture

Strict layers: handler → service → repository → platform.

- `cmd/server/` — main API (port 3000)
- `cmd/admin-server/` — **stub with mock data** and its own ad-hoc config loading; does **not** use `internal/config`
- `internal/config/` — `godotenv.Load()` reads `.env`; never call `os.Getenv` elsewhere
- `internal/domain/` — sentinel errors + `AppError` struct
- `internal/handler/response.go` — standard envelope format; use `sendAppError` to map domain errors to HTTP codes
- `internal/repository/` — raw SQL with pgx (`pgx.CollectRows`, `pgx.RowToStructByName`); no ORM
- `platform/` — external client initialization (postgres, redis, firebase, blob, zalopay)

## Developer Commands

```bash
# Start only infrastructure deps locally, then run server outside Docker
docker-compose up postgres redis pg_tileserv
go run ./cmd/server

# Run tests (pure unit tests, no DB/Redis required)
go test ./...

# Run migrations (requires psql, not golang-migrate)
DATABASE_URL="postgresql://..." ./infra/scripts/migrate.sh

# Build service image (default server; override with --build-arg SERVICE=admin-server)
docker build --build-arg SERVICE=server -t smatch-server .
```

## Environment

Copy `.env.example` → `.env`. `internal/config/config.go` loads it automatically via `godotenv`.

Key gotchas:
- `FIREBASE_CREDENTIALS_FILE` defaults to `smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json` in repo root. The file is present but gitignored — **do not commit changes to it**.
- `AZURE_BLOB_ENDPOINT` is only needed for Azurite; leave blank for real Azure.
- `REDIS_TLS_ENABLED` must be `true` in production (Azure Cache for Redis enforces TLS).

## Migrations

- Custom bash runner: `infra/scripts/migrate.sh` iterates `migrations/*.up.sql` with `psql`.
- Always create `.up.sql` and `.down.sql` pairs.
- Sequential numbering: `000001_init.up.sql`, `000001_init.down.sql`.

## Local Development / Azurite

```bash
# One-time bootstrap: starts Postgres, Redis, Azurite, runs migrations, writes .env.azurite
bash infra/scripts/init.sh
```

Requires: `docker`, `psql`, `az`. After init, run locally against local services with:

```bash
cp .env.azurite .env
go run ./cmd/server
```

## Testing

- Co-located `_test.go` files; same package for white-box, `_test` suffix for black-box.
- Handler tests use `httptest`.
- Service tests mock repository interfaces (define interfaces in `repository/` for testability).
- No global state or external fixtures required.

## Code Conventions

- Errors: define sentinels in `internal/domain/errors.go`; handlers map them in `handler/response.go`.
- DTOs: one file per feature in `internal/dto/`. JSON tags required. Validation belongs in handlers before service calls.
- Service layer: business logic only, no HTTP concerns. Inject repository interfaces.
- Logging: `zap` via DI. Use structured fields, never `fmt.Sprintf`.

## Operational Notes

- Redis is **optional at runtime**: the server warns and continues without it; rate limiting (`go-chi/httprate`) is disabled when Redis is nil.
- Blob Storage client is initialized in `cmd/server/main.go` but currently discarded (`_ = blobClient`) — reserved for future image uploads.
- WebSocket hub (`internal/websocket`) wires `OnPaymentDisconnect` to auto-cancel payments via the payment handler.
- Admin endpoints require the `ADMIN_SECRET` via `X-Admin-Secret` header when Firebase custom claims are not set.
- `docker-compose.yml` runs an nginx reverse proxy on port `8080` routing `/api/map-tiles/*` to `pg_tileserv:7800` and everything else to `backend:3000`.

## Terraform

Target Azure region: `southeastasia`. Do not commit `terraform.tfstate`, `terraform.tfvars`, or `tfplan.out`.

## Agent Safety Rules

- **Don't ever make file deletions without asking the user or getting explicit permission.** If a deletion seems necessary, immediately pause the chat and ask.

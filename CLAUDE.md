# smatch-backend-go

Badminton court booking platform backend. Go 1.23, chi router, PostgreSQL (pgx/v5), Redis, Firebase Auth, ZaloPay, Azure Blob Storage, pg_tileserv for geospatial tile serving.

## Architecture

Layered architecture — strict dependency direction: handler → service → repository → platform.

```
cmd/
  server/          # main HTTP server (port 3000)
  admin-server/    # admin HTTP server (port 3001)
internal/
  config/          # env-based config via godotenv
  domain/          # pure domain types and errors (no external deps)
  dto/             # request/response structs per feature
  handler/         # HTTP handlers, route wiring
  middleware/      # Firebase JWT auth middleware
  repository/      # SQL queries via pgx/v5 pool
  service/         # business logic
  websocket/       # gorilla/websocket hub
platform/
  firebase/        # Admin SDK init
  postgres/        # pgx pool init
  redis/           # go-redis client init
  blob/             # Azure Blob Storage client init
  zalopay/         # ZaloPay HTTP client
infra/
  terraform/       # Azure infra (LB, VMSS, PostgreSQL, Redis, Storage, VNet)
  scripts/         # init.sh, migrate.sh
migrations/        # golang-migrate SQL files (up/down pairs)
nginx/             # nginx.dev.conf reverse proxy
```

## Development

```bash
# Start all services (postgres, redis, pg_tileserv, backend, admin, nginx)
docker-compose up --build

# Run only infrastructure deps, run server locally
docker-compose up postgres redis pg_tileserv

# Run tests
go test ./...

# Run migrations (requires DATABASE_URL)
./infra/scripts/migrate.sh

# Build specific service
docker build --build-arg SERVICE=server -t smatch-server .
docker build --build-arg SERVICE=admin-server -t smatch-admin .
```

## Environment Variables

Copy `.env.example` → `.env`. Key vars:

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `3000` | Main server port |
| `DATABASE_URL` | — | PostgreSQL DSN |
| `REDIS_HOST/PORT` | `localhost:6379` | |
| `REDIS_TLS_ENABLED` | `false` | Set `true` in prod |
| `FIREBASE_CREDENTIALS_FILE` | `smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json` | Path to service account JSON |
| `ZALOPAY_APP_ID/KEY1/KEY2` | — | ZaloPay sandbox/prod creds |
| `AZURE_STORAGE_ACCOUNT/KEY` | — | Blob Storage access |
| `AZURE_BLOB_ENDPOINT` | — | Set to Azurite URL for local blob |
| `AZURE_STORAGE_CONTAINER_PROFILE/MATCHES/BUSINESS_DOCS` | `smatch-profiles/smatch-matches/smatch-business-docs` | |
| `ADMIN_SECRET` | — | Bearer token for admin API |
| `SLOT_LOCK_TTL_SECONDS` | `600` | Redis slot lock duration |
| `TILE_SERVER_URL` | `http://localhost:7800` | pg_tileserv URL |

## Code Conventions

- **Errors**: define sentinel errors in `internal/domain/errors.go`. Handlers map domain errors to HTTP status codes in `handler/response.go`.
- **DTOs**: one file per feature in `internal/dto/`. Use `json` tags. Validation lives in the handler before calling service.
- **Repository**: raw SQL with pgx — no ORM. Use `pgx.CollectRows` / `pgx.RowToStructByName` for scanning.
- **Service**: inject repository interfaces (not concrete types) for testability. Business logic only — no HTTP concerns.
- **Logging**: `go.uber.org/zap` logger passed via dependency injection. Use structured fields, not `fmt.Sprintf`.
- **Config**: never read `os.Getenv` outside `internal/config/config.go`.
- **Migrations**: always create both `.up.sql` and `.down.sql`. Sequential numbering: `000003_...`.

## Testing

- Unit tests co-locate with source (`_test.go` suffix, same package for white-box or `_test` suffix for black-box).
- Service tests mock repository interfaces.
- Handler tests in `internal/handler/*_test.go` use `httptest`.
- No global test state; each test sets up and tears down its own fixtures.

## Infrastructure (Terraform)

Located in `infra/terraform/`. Azure target: southeastasia.

Key resources: VNet with public/private app/private data subnets, Load Balancer, VM Scale Sets (3: backend, admin, tileserv), Azure Database for PostgreSQL Flexible Server, Azure Cache for Redis, Blob Storage containers, Azure Container Registry, Key Vault.

```bash
cd infra/terraform
terraform init
terraform plan -out tfplan.out
terraform apply tfplan.out
```

Do not commit `terraform.tfstate`, `terraform.tfvars`, or `tfplan.out` — they may contain secrets.

## Security Notes

- `smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json` is a service account credential. Never commit changes to it; it is gitignored in production.
- Admin endpoints require `ADMIN_SECRET` bearer token.
- Rate limiting is applied via `go-chi/httprate` on public routes.
- Firebase JWT validation happens in `internal/middleware/auth.go`.

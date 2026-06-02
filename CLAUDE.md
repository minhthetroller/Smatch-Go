# smatch-backend-go

Badminton court booking platform backend. Go 1.23, chi router, PostgreSQL (pgx/v5), Redis, Firebase Auth, ZaloPay, AWS S3, pg_tileserv for geospatial tile serving.

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
  s3/              # AWS SDK v2 S3 client init
  zalopay/         # ZaloPay HTTP client
infra/
  terraform/       # AWS infra (ALB, ASG, RDS, ElastiCache, S3, VPC)
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
| `AWS_REGION/ACCESS_KEY_ID/SECRET_ACCESS_KEY` | — | S3 access |
| `AWS_ENDPOINT` | — | Set to LocalStack URL for local S3 |
| `AWS_S3_BUCKET_PROFILE/MATCHES` | `smatch-profiles/smatch-matches` | |
| `ADMIN_SECRET` | — | Bearer token for admin API |
| `PAYMENT_WS_TICKET_TTL_SECONDS` | `60` | Short-lived websocket subscription ticket TTL; payment validity is fixed at 300s |
| `TILE_SERVER_URL` | `http://localhost:7800` | pg_tileserv URL |

## Code Conventions

- **Errors**: define sentinel errors in `internal/domain/errors.go`. Handlers map domain errors to HTTP status codes in `handler/response.go`.
- **DTOs**: one file per feature in `internal/dto/`. Use `json` tags. Validation lives in the handler before calling service.
- **Repository**: raw SQL with pgx — no ORM. Use `pgx.CollectRows` / `pgx.RowToStructByName` for scanning.
- **Service**: inject repository interfaces (not concrete types) for testability. Business logic only — no HTTP concerns.
- **Logging**: `go.uber.org/zap` logger passed via dependency injection. Use structured fields, not `fmt.Sprintf`.
- **Config**: never read `os.Getenv` outside `internal/config/config.go`.
- **Migrations**: always create both `.up.sql` and `.down.sql`. Sequential numbering: `000003_...`.
- **Dead code**: when removing or replacing behavior, also remove deprecated routes, methods, unused helpers, dead code, and stale tests in the same change. Do not leave compatibility shims unless explicitly requested.

## Testing

- Unit tests co-locate with source (`_test.go` suffix, same package for white-box or `_test` suffix for black-box).
- Service tests mock repository interfaces.
- Handler tests in `internal/handler/*_test.go` use `httptest`.
- No global test state; each test sets up and tears down its own fixtures.

## Infrastructure (Terraform)

Located in `infra/terraform/`. AWS target: ap-southeast-1.

Key resources: VPC with public/private/data subnets, ALB, ASG (EC2 launch templates), RDS PostgreSQL + read replica, ElastiCache Redis, S3 buckets, pg_tileserv on its own ASG.

```bash
cd infra/terraform
terraform init
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```

Do not commit `terraform.tfstate`, `terraform.tfvars`, or `tfplan.out` — they may contain secrets.

## Security Notes

- `smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json` is a service account credential. Never commit changes to it; it is gitignored in production.
- Admin endpoints require `ADMIN_SECRET` bearer token.
- Rate limiting is applied via `go-chi/httprate` on public routes.
- Firebase JWT validation happens in `internal/middleware/auth.go`.

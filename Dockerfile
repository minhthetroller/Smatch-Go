# SERVICE selects which cmd/ binary to build.
# Default is the user-facing API server; pass --build-arg SERVICE=admin-server for admin.
ARG SERVICE=server

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
ARG SERVICE

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/service ./cmd/${SERVICE}

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 3000

ENTRYPOINT ["/app/service"]

#!/usr/bin/env bash
# Bootstrap script for pg_tileserv EC2 instances (Amazon Linux 2023).
# Installs nginx (path-rewrite proxy) and pg_tileserv (vector tile server).
set -euo pipefail

# ── Install dependencies ──────────────────────────────────────────────────────
dnf install -y nginx golang git

# ── Write nginx config ────────────────────────────────────────────────────────
# Rewrites /api/map-tiles/{z}/{x}/{y}.pbf → /public.courts/{z}/{x}/{y}.pbf
# then proxies to pg_tileserv on localhost:${tileserv_port}.
# NOTE: heredoc uses <<'NGINX' (single-quoted) so bash does NOT expand $vars or
# $$ (which bash expands to PID in unquoted heredocs). Terraform template vars
# (${tileserv_port} etc.) are substituted before the script runs, so they work
# fine inside a quoted heredoc.
cat > /etc/nginx/conf.d/tileserv.conf <<'NGINX'
server {
    listen ${tileserv_nginx_port};
    server_name _;

    # Rewrite tile path and proxy to pg_tileserv
    location ~ ^/api/map-tiles/([0-9]+)/([0-9]+)/([0-9]+)\.pbf$ {
        rewrite ^/api/map-tiles/(.*)$ /public.courts/$1 break;
        proxy_pass         http://127.0.0.1:${tileserv_port};
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_send_timeout 30s;
    }

    # Pass-through for pg_tileserv native paths (health, metadata, etc.)
    location / {
        proxy_pass         http://127.0.0.1:${tileserv_port};
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_send_timeout 30s;
    }
}
NGINX

# Remove default nginx site to avoid port conflicts
rm -f /etc/nginx/conf.d/default.conf

# Replace nginx.conf with minimal version — no default server block so our
# tileserv.conf in conf.d is the only server and handles all requests.
cat > /etc/nginx/nginx.conf <<'NGINXMAIN'
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log notice;
pid /run/nginx.pid;

include /usr/share/nginx/modules/*.conf;

events {
    worker_connections 1024;
}

http {
    log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"';

    access_log  /var/log/nginx/access.log  main;

    sendfile            on;
    tcp_nopush          on;
    keepalive_timeout   65;
    types_hash_max_size 4096;

    include             /etc/nginx/mime.types;
    default_type        application/octet-stream;

    include /etc/nginx/conf.d/*.conf;
}
NGINXMAIN

# ── Download and install pg_tileserv ─────────────────────────────────────────
# ── Build and install pg_tileserv from source ─────────────────────────────────
PG_TILESERV_VERSION="${pg_tileserv_version}"

export HOME=/root
export GOPATH=/tmp/gobuild
export GOCACHE=/tmp/gocache
export GOFLAGS=-mod=mod

go install "github.com/CrunchyData/pg_tileserv@v$${PG_TILESERV_VERSION}"
cp /tmp/gobuild/bin/pg_tileserv /usr/local/bin/pg_tileserv
chmod +x /usr/local/bin/pg_tileserv
rm -rf /tmp/gobuild /tmp/gocache

# ── Write pg_tileserv environment file ───────────────────────────────────────
# NOTE: DO NOT name this file pg_tileserv.* — pg_tileserv auto-discovers
# any file matching that pattern in /etc/ and tries to parse it as TOML config,
# which would corrupt the shell env format and cause a fatal startup error.
cat > /etc/tileserv.env <<EOF
DATABASE_URL=${database_url}
TS_HTTPPORT=${tileserv_port}
TS_BASEPATH=/
EOF
# Create service user BEFORE setting env file permissions so we can chown it
useradd -r -s /sbin/nologin pg_tileserv 2>/dev/null || true
chown root:pg_tileserv /etc/tileserv.env
chmod 640 /etc/tileserv.env

# ── Create dedicated service user ─────────────────────────────────────────────
# (already created above)

# ── Install pg_tileserv systemd unit ─────────────────────────────────────────
cat > /etc/systemd/system/pg_tileserv.service <<'UNIT'
[Unit]
Description=pg_tileserv vector tile server
After=network.target

[Service]
Type=simple
User=pg_tileserv
EnvironmentFile=/etc/tileserv.env
ExecStart=/usr/local/bin/pg_tileserv
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT

# ── Start services ────────────────────────────────────────────────────────────
systemctl daemon-reload
systemctl enable --now pg_tileserv
systemctl enable --now nginx

#!/usr/bin/env bash
# Bootstrap script injected into EC2 instances via launch template user data.
# Pulls and runs the smatch-backend Docker image from ECR.
set -euo pipefail

# ── Install Docker (Amazon Linux 2023) ────────────────────────────────────────
dnf install -y docker
systemctl enable --now docker

# ── Authenticate Docker with ECR ──────────────────────────────────────────────
aws ecr get-login-password --region ${aws_region} \
  | docker login --username AWS --password-stdin ${ecr_repo_url}

# ── Pull the latest backend image ─────────────────────────────────────────────
docker pull ${ecr_repo_url}:latest

# ── Write environment file (chmod 600 so only root can read secrets) ──────────
cat > /etc/smatch.env <<EOF
PORT=${backend_port}
DATABASE_URL=${database_url}
REDIS_HOST=${redis_host}
REDIS_PORT=${redis_port}
REDIS_PASSWORD=${redis_password}
REDIS_TLS_ENABLED=true
AWS_REGION=${aws_region}
AWS_S3_BUCKET_PROFILE=${s3_bucket_profile}
AWS_S3_BUCKET_MATCHES=${s3_bucket_matches}
FIREBASE_CREDENTIALS_FILE=${firebase_creds_file}
ZALOPAY_APP_ID=${zalopay_app_id}
ZALOPAY_KEY1=${zalopay_key1}
ZALOPAY_KEY2=${zalopay_key2}
ZALOPAY_ENDPOINT=${zalopay_endpoint}
ZALOPAY_CALLBACK_URL=${zalopay_callback_url}
TILE_SERVER_URL=${tile_server_url}
ADMIN_SECRET=${admin_secret}
RATE_LIMIT_TRUSTED_IPS=${rate_limit_trusted_ips}
EOF
chmod 600 /etc/smatch.env

# ── Run the backend container ─────────────────────────────────────────────────
docker run -d \
  --name smatch-backend \
  --restart unless-stopped \
  --env-file /etc/smatch.env \
  -p ${backend_port}:${backend_port} \
  ${ecr_repo_url}:latest

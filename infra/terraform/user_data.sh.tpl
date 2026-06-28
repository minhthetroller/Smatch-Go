#!/usr/bin/env bash
# Bootstrap script injected into EC2 instances via launch template user data.
# Pulls and runs the smatch-backend Docker image from ECR.
set -euo pipefail

# ── Verify baked runtime agents (Amazon Linux 2023) ──────────────────────────
command -v aws >/dev/null
command -v docker >/dev/null
test -x /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl

systemctl enable --now docker
systemctl enable --now amazon-ssm-agent || true

# ── Authenticate Docker with ECR ──────────────────────────────────────────────
aws ecr get-login-password --region ${aws_region} \
  | docker login --username AWS --password-stdin ${ecr_repo_url}

# ── Pull the backend image ───────────────────────────────────────────────────
docker pull ${ecr_repo_url}:${image_tag}

# ── Fetch Firebase credentials from Secrets Manager ──────────────────────────
aws secretsmanager get-secret-value \
  --region ${aws_region} \
  --secret-id smatch/firebase/credentials \
  --query SecretString \
  --output text > /etc/firebase-adminsdk.json
chmod 600 /etc/firebase-adminsdk.json

# ── Write environment file (chmod 600 so only root can read secrets) ──────────
cat > /etc/smatch.env <<EOF
PORT=${backend_port}
ADMIN_PORT=${backend_port}
ADMIN_WEB_ORIGIN=${admin_web_origin}
DATABASE_URL=${database_url}
REDIS_HOST=${redis_host}
REDIS_PORT=${redis_port}
REDIS_PASSWORD=${redis_password}
REDIS_TLS_ENABLED=true
AWS_REGION=${aws_region}
AWS_S3_BUCKET_PROFILE=${s3_bucket_profile}
AWS_S3_BUCKET_MATCHES=${s3_bucket_matches}
AWS_S3_BUCKET_BUSINESS_DOCS=${s3_bucket_business_docs}
FIREBASE_CREDENTIALS_FILE=/app/firebase-adminsdk.json
ZALOPAY_APP_ID=${zalopay_app_id}
ZALOPAY_KEY1=${zalopay_key1}
ZALOPAY_KEY2=${zalopay_key2}
ZALOPAY_ENDPOINT=${zalopay_endpoint}
ZALOPAY_CALLBACK_URL=${zalopay_callback_url}
TILE_SERVER_URL=${tile_server_url}
ADMIN_SECRET=${admin_secret}
RATE_LIMIT_TRUSTED_IPS=${rate_limit_trusted_ips}
LOAD_TEST_STRESS_ENABLED=${load_test_stress_enabled}
EOF
chmod 600 /etc/smatch.env

# ── Run the backend container ─────────────────────────────────────────────────
docker run -d \
  --name smatch-backend \
  --restart unless-stopped \
  --env-file /etc/smatch.env \
%{ if container_cpu_limit != "" ~}
  --cpus ${container_cpu_limit} \
%{ endif ~}
  -v /etc/firebase-adminsdk.json:/app/firebase-adminsdk.json:ro \
  -p ${backend_port}:${backend_port} \
  ${ecr_repo_url}:${image_tag}

# ── Stream Docker logs to CloudWatch Logs ────────────────────────────────────
cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json <<EOF
{
  "agent": {
    "run_as_user": "root"
  },
  "metrics": {
    "append_dimensions": {
      "AutoScalingGroupName": "\$${aws:AutoScalingGroupName}",
      "ImageId": "\$${aws:ImageId}",
      "InstanceId": "\$${aws:InstanceId}",
      "InstanceType": "\$${aws:InstanceType}"
    },
    "aggregation_dimensions": [
      ["AutoScalingGroupName"],
      ["InstanceId"]
    ],
    "metrics_collected": {
%{ if cloudwatch_cpu_metrics_enabled ~}
      "cpu": {
        "measurement": [
          "usage_idle",
          "usage_user",
          "usage_system",
          "usage_iowait"
        ],
        "metrics_collection_interval": ${cloudwatch_metrics_collection_interval_seconds},
        "resources": [
          "*"
        ],
        "totalcpu": true
      },
%{ endif ~}
      "disk": {
        "measurement": [
          "used_percent",
          "inodes_used"
        ],
        "metrics_collection_interval": ${cloudwatch_metrics_collection_interval_seconds},
        "resources": [
          "/"
        ]
      },
      "diskio": {
        "measurement": [
          "read_bytes",
          "write_bytes",
          "reads",
          "writes"
        ],
        "metrics_collection_interval": ${cloudwatch_metrics_collection_interval_seconds},
        "resources": [
          "*"
        ]
      },
      "mem": {
        "measurement": [
          "mem_used_percent",
          "mem_available_percent"
        ],
        "metrics_collection_interval": ${cloudwatch_metrics_collection_interval_seconds}
      }
    }
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/lib/docker/containers/*/*.log",
            "log_group_name": "${cloudwatch_log_group_name}",
            "log_stream_name": "{instance_id}/${service_name}",
            "timezone": "UTC"
          }
        ]
      }
    }
  }
}
EOF

/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a fetch-config \
  -m ec2 \
  -s \
  -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json

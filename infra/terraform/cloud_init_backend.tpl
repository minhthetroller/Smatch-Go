#cloud-config

bootcmd:
  - [ cloud-init-per, once, mask-nginx, systemctl, mask, nginx.service ]

packages:
  - docker.io
  - nginx
  - certbot
  - python3-certbot-nginx
  - openssl

write_files:
  - path: /etc/firebase-adminsdk.json
    permissions: '0600'
    encoding: b64
    content: ${firebase_credentials_b64}

  - path: /etc/smatch.env
    permissions: '0600'
    content: |
      PORT=3000
      DATABASE_URL=${database_url}
      REDIS_HOST=${redis_host}
      REDIS_PORT=${redis_port}
      REDIS_PASSWORD=${redis_password}
      REDIS_TLS_ENABLED=true
      AZURE_STORAGE_ACCOUNT=${storage_account_name}
      AZURE_STORAGE_KEY=${storage_account_key}
      AZURE_STORAGE_CONTAINER_PROFILE=${storage_container_profile}
      AZURE_STORAGE_CONTAINER_MATCHES=${storage_container_matches}
      AZURE_STORAGE_CONTAINER_BUSINESS_DOCS=${storage_container_business_docs}
      FIREBASE_CREDENTIALS_FILE=/app/firebase-adminsdk.json
      ZALOPAY_APP_ID=${zalopay_app_id}
      ZALOPAY_KEY1=${zalopay_key1}
      ZALOPAY_KEY2=${zalopay_key2}
      ZALOPAY_ENDPOINT=${zalopay_endpoint}
      ZALOPAY_CALLBACK_URL=${zalopay_callback_url}
      TILE_SERVER_URL=${tile_server_url}
      ADMIN_SECRET=${admin_secret}
      RATE_LIMIT_TRUSTED_IPS=${rate_limit_trusted_ips}

  - path: /usr/local/sbin/smatch-bootstrap.sh
    permissions: '0755'
    content: |
      #!/usr/bin/env bash
      set -euxo pipefail

      retry() { local n=0; until "$@"; do n=$((n+1)); [ $n -ge 5 ] && return 1; sleep $((n*5)); done; }

      DOMAIN="${cert_domain}"
      EMAIL="${letsencrypt_email}"

      # Directories
      mkdir -p /var/www/certbot /etc/nginx/ssl /etc/nginx/sites-available /etc/nginx/sites-enabled

      # Bootstrap self-signed cert so nginx can start with HTTPS server immediately
      openssl req -x509 -nodes -days 30 -newkey rsa:2048 \
        -keyout /etc/nginx/ssl/privkey.pem \
        -out    /etc/nginx/ssl/fullchain.pem \
        -subj   "/CN=$DOMAIN"

      # Write nginx config (double-quoted heredoc; nginx vars escaped with \$)
      cat > /etc/nginx/sites-available/smatch <<NGINX_CONF
      server {
          listen 3000;
          server_name $DOMAIN;

          location /.well-known/acme-challenge/ {
              root /var/www/certbot;
          }

          location /health {
              return 200 'ok';
              add_header Content-Type text/plain;
          }

          location / {
              return 301 https://\$host\$request_uri;
          }
      }

      server {
          listen 443 ssl;
          server_name $DOMAIN;
          ssl_certificate     /etc/nginx/ssl/fullchain.pem;
          ssl_certificate_key /etc/nginx/ssl/privkey.pem;
          ssl_protocols TLSv1.2 TLSv1.3;
          ssl_ciphers HIGH:!aNULL:!MD5;

          location / {
              proxy_pass http://127.0.0.1:3001;
              proxy_set_header Host \$host;
              proxy_set_header X-Real-IP \$remote_addr;
              proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
              proxy_set_header X-Forwarded-Proto https;
          }
      }
      NGINX_CONF

      # Activate nginx (unmasked because bootcmd masked it before package install)
      rm -f /etc/nginx/sites-enabled/default
      ln -sf /etc/nginx/sites-available/smatch /etc/nginx/sites-enabled/smatch
      systemctl unmask nginx
      nginx -t
      systemctl enable --now nginx

      # Obtain Let's Encrypt cert via HTTP-01 webroot (LB rule: 80 -> 3000 -> nginx)
      # Non-fatal: LE rate limits should not block docker container startup
      if [ -n "$DOMAIN" ]; then
        if [ -n "$EMAIL" ]; then
          retry certbot certonly --webroot -w /var/www/certbot \
            -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL" --keep-until-expiring || true
        else
          retry certbot certonly --webroot -w /var/www/certbot \
            -d "$DOMAIN" --non-interactive --agree-tos \
            --register-unsafely-without-email --keep-until-expiring || true
        fi
        # Only promote cert if LE issued one; otherwise nginx keeps the self-signed bootstrap cert
        if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
          cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem /etc/nginx/ssl/fullchain.pem
          cp /etc/letsencrypt/live/$DOMAIN/privkey.pem   /etc/nginx/ssl/privkey.pem
          systemctl reload nginx

          mkdir -p /etc/letsencrypt/renewal-hooks/deploy
          cat > /etc/letsencrypt/renewal-hooks/deploy/copy-and-reload.sh <<HOOK
      #!/usr/bin/env bash
      set -euo pipefail
      cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem /etc/nginx/ssl/fullchain.pem
      cp /etc/letsencrypt/live/$DOMAIN/privkey.pem   /etc/nginx/ssl/privkey.pem
      systemctl reload nginx
      HOOK
          chmod +x /etc/letsencrypt/renewal-hooks/deploy/copy-and-reload.sh
        fi
      fi

      systemctl enable --now certbot.timer

      # Docker setup
      systemctl enable --now docker
      retry docker login ${acr_name}.azurecr.io -u ${acr_name} -p ${acr_password}
      retry docker pull ${acr_name}.azurecr.io/backend:${image_tag}
      docker run -d --name smatch-backend --restart unless-stopped \
        --env-file /etc/smatch.env \
        -v /etc/firebase-adminsdk.json:/app/firebase-adminsdk.json:ro \
        -p 127.0.0.1:3001:3000 \
        ${acr_name}.azurecr.io/backend:${image_tag}

runcmd:
  - /usr/local/sbin/smatch-bootstrap.sh 2>&1 | tee /var/log/smatch-bootstrap.log

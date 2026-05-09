#cloud-config

packages:
  - docker.io
  - nginx

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

runcmd:
  # Generate self-signed cert (replaced by Let's Encrypt later)
  - mkdir -p /etc/nginx/ssl
  - openssl req -x509 -nodes -days 30 -newkey rsa:2048 -keyout /etc/nginx/ssl/privkey.pem -out /etc/nginx/ssl/fullchain.pem -subj "/CN=${cert_domain}"

  # Configure nginx
  - |
    cat > /etc/nginx/sites-available/smatch <<'NGX'
    server {
        listen 3000;
        server_name ${cert_domain};
        return 301 https://$host$request_uri;
    }
    server {
        listen 443 ssl;
        server_name ${cert_domain};
        ssl_certificate /etc/nginx/ssl/fullchain.pem;
        ssl_certificate_key /etc/nginx/ssl/privkey.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        location / {
            proxy_pass http://127.0.0.1:3001;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto https;
        }
    }
    NGX
  - rm -f /etc/nginx/sites-enabled/default
  - ln -sf /etc/nginx/sites-available/smatch /etc/nginx/sites-enabled/default
  - nginx -t && systemctl enable --now nginx

  # Docker setup
  - systemctl enable --now docker
  - docker login ${acr_name}.azurecr.io -u ${acr_name} -p ${acr_password}
  - docker pull ${acr_name}.azurecr.io/backend:${image_tag}
  - docker run -d --name smatch-backend --restart unless-stopped --env-file /etc/smatch.env -v /etc/firebase-adminsdk.json:/app/firebase-adminsdk.json:ro -p 127.0.0.1:3001:3000 ${acr_name}.azurecr.io/backend:${image_tag}

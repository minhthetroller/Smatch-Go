#cloud-config

bootcmd:
  - [ cloud-init-per, once, mask-nginx, systemctl, mask, nginx.service ]

packages:
  - nginx
  - wget
  - unzip

write_files:
  - path: /usr/local/sbin/tileserv-bootstrap.sh
    permissions: '0755'
    content: |
      #!/usr/bin/env bash
      set -euxo pipefail

      retry() { local n=0; until "$@"; do n=$((n+1)); [ $n -ge 5 ] && return 1; sleep $((n*5)); done; }

      # Write nginx config (Terraform substitutes port vars before script is written;
      # single-quoted heredoc so bash does not re-expand them at runtime)
      mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
      cat > /etc/nginx/sites-available/tileserv <<'NGINX_CONF'
      server {
          listen ${tileserv_nginx_port};
          location / {
              proxy_pass http://127.0.0.1:${tileserv_port};
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
          }
      }
      NGINX_CONF

      # Activate nginx
      rm -f /etc/nginx/sites-enabled/default
      ln -sf /etc/nginx/sites-available/tileserv /etc/nginx/sites-enabled/tileserv
      systemctl unmask nginx
      nginx -t
      systemctl enable --now nginx

      # Download pg_tileserv binary
      retry wget --tries=5 --timeout=30 \
        https://github.com/CrunchyData/pg_tileserv/releases/download/v${pg_tileserv_version}/pg_tileserv_linux_amd64.zip \
        -O /tmp/pg_tileserv.zip
      unzip -o /tmp/pg_tileserv.zip -d /usr/local/bin/
      chmod +x /usr/local/bin/pg_tileserv
      rm /tmp/pg_tileserv.zip

      # Install pg_tileserv as a systemd service
      # Terraform substitutes DATABASE_URL before script is written;
      # single-quoted heredoc so bash does not re-expand it at runtime
      cat > /etc/systemd/system/pg_tileserv.service <<'SVC'
      [Unit]
      Description=pg_tileserv
      After=network.target
      [Service]
      Type=simple
      Environment=DATABASE_URL=${database_url}
      ExecStart=/usr/local/bin/pg_tileserv
      Restart=always
      [Install]
      WantedBy=multi-user.target
      SVC
      systemctl daemon-reload
      systemctl enable --now pg_tileserv

runcmd:
  - /usr/local/sbin/tileserv-bootstrap.sh 2>&1 | tee /var/log/tileserv-bootstrap.log

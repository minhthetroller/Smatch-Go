#cloud-config
package_upgrade: true

packages:
  - nginx
  - wget

write_files:
  - path: /etc/nginx/sites-available/tileserv
    permissions: '0644'
    content: |
      server {
          listen ${tileserv_nginx_port};
          location / {
              proxy_pass http://127.0.0.1:${tileserv_port};
              proxy_set_header Host $host;
              proxy_set_header X-Real-IP $remote_addr;
          }
      }

runcmd:
  # Install pg_tileserv binary
  - wget -q https://github.com/CrunchyData/pg_tileserv/releases/download/v${pg_tileserv_version}/pg_tileserv_linux_amd64.zip -O /tmp/pg_tileserv.zip
  - unzip -o /tmp/pg_tileserv.zip -d /usr/local/bin/
  - chmod +x /usr/local/bin/pg_tileserv
  - rm /tmp/pg_tileserv.zip

  # Enable nginx
  - ln -sf /etc/nginx/sites-available/tileserv /etc/nginx/sites-enabled/default
  - systemctl enable --now nginx

  # Run pg_tileserv as a systemd service
  - |
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
  - systemctl daemon-reload
  - systemctl enable --now pg_tileserv

#!/usr/bin/env bash
# Install only LeaguePingo-owned files; the administrator installs nginx/python3/curl first.
set -euo pipefail
ref="${1:-}"
port="${2:-8080}"
[[ "$EUID" == 0 ]] || { echo 'Run this installer as root.' >&2; exit 1; }
[[ "$ref" =~ ^[a-f0-9]{40}$ && "$port" =~ ^[0-9]{2,5}$ && "$port" -ge 1024 && "$port" -le 65535 ]] || {
  echo 'Usage: bash install.sh PINNED_40_CHARACTER_COMMIT [PORT>=1024]' >&2; exit 1;
}
for tool in nginx python3 curl systemctl runuser useradd ss; do
  command -v "$tool" >/dev/null || { echo "Install required tool first: $tool" >&2; exit 1; }
done
root=/srv/leaguepingo
library=/usr/local/lib/leaguepingo
config=/etc/nginx/conf.d/leaguepingo-download.conf
service=/etc/systemd/system/leaguepingo-mirror.service
timer=/etc/systemd/system/leaguepingo-mirror.timer
for file in "$config" "$service" "$timer"; do
  if [[ -e "$file" ]] && ! head -n 1 "$file" | grep -qx '# Managed by LeaguePingo'; then
    echo "Refusing to replace an unmanaged file: $file" >&2; exit 1
  fi
done
for directory in "$root" "$library"; do
  if [[ -L "$directory" ]] || { [[ -d "$directory" && ! -f "$directory/.leaguepingo-managed" ]] && [[ -n "$(ls -A "$directory")" ]]; }; then
    echo "Refusing to use an unmanaged directory: $directory" >&2; exit 1
  fi
done
if ss -H -ltn "sport = :$port" | grep -q .; then
  if [[ ! -f "$config" ]] || ! grep -q "^    listen $port;$" "$config"; then
    echo "Port $port is occupied; choose another port." >&2; exit 1
  fi
fi
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
curl --fail --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 120 \
  "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/$ref/website/deploy/mirror.py" -o "$temporary/mirror.py"
id leaguepingo-mirror >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin leaguepingo-mirror
install -d -m 755 "$library" "$root"
install -m 755 "$temporary/mirror.py" "$library/mirror.py"
touch "$library/.leaguepingo-managed"
touch "$root/.leaguepingo-managed"
chown leaguepingo-mirror:leaguepingo-mirror "$root" "$root/.leaguepingo-managed"
runuser -u leaguepingo-mirror -- python3 /usr/local/lib/leaguepingo/mirror.py sync
cat > "$temporary/nginx.conf" <<EOF
# Managed by LeaguePingo
server {
    listen $port;
    listen [::]:$port;
    server_name _;
    root /srv/leaguepingo/current;
    index index.html;
    autoindex off;
    add_header X-Content-Type-Options nosniff always;
    location = /index.html { add_header Cache-Control "no-cache"; }
    location = /downloads.json { add_header Cache-Control "no-cache"; }
    location / { try_files \$uri \$uri/ =404; }
    location /releases/ {
        alias /srv/leaguepingo/releases/;
        default_type application/octet-stream;
        types { }
        add_header Content-Disposition "attachment";
        add_header X-Content-Type-Options nosniff always;
        add_header Cache-Control "public, max-age=31536000, immutable";
        limit_except GET { deny all; }
    }
    location ~ /\. { deny all; }
}
EOF
if [[ -f "$config" ]]; then cp "$config" "$temporary/previous.conf"; fi
restore_config() {
  if [[ -f "$temporary/previous.conf" ]]; then cp "$temporary/previous.conf" "$config"; else rm -f "$config"; fi
}
install -m 644 "$temporary/nginx.conf" "$config"
if ! nginx -t; then
  restore_config
  exit 1
fi
cat > "$service" <<'EOF'
# Managed by LeaguePingo
[Unit]
Description=Update verified LeaguePingo website and downloads
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
User=leaguepingo-mirror
Group=leaguepingo-mirror
ExecStart=/usr/bin/python3 /usr/local/lib/leaguepingo/mirror.py sync
TimeoutStartSec=15min
MemoryMax=192M
UMask=0022
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/srv/leaguepingo
EOF
cat > "$timer" <<'EOF'
# Managed by LeaguePingo
[Unit]
Description=Check LeaguePingo mirror every five minutes

[Timer]
OnBootSec=2min
OnUnitInactiveSec=5min
RandomizedDelaySec=30s
Unit=leaguepingo-mirror.service

[Install]
WantedBy=timers.target
EOF
systemctl daemon-reload
if systemctl is-active --quiet nginx; then
  if ! systemctl reload nginx; then restore_config; exit 1; fi
else
  if ! systemctl start nginx; then restore_config; exit 1; fi
fi
curl --noproxy 127.0.0.1 --fail --silent --show-error "http://127.0.0.1:$port/downloads.json" >/dev/null
systemctl enable --now leaguepingo-mirror.timer
if command -v ufw >/dev/null && ufw status | grep -q '^Status: active'; then ufw allow "$port/tcp"; fi
echo "Mirror ready locally. Open http://YOUR_SERVER_IP:$port/ after allowing TCP $port in the Vultr firewall."

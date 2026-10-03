#!/usr/bin/env bash
set -Eeuo pipefail

REPO_URL="${1:?repo URL required}"
REF="${2:?git ref required}"
KARETA_ROOT="${3:?install path required}"
LOCAL_URL="${4:?local URL required}"
RUNTIME_HEALTH_PATH="${5:?runtime health path required}"
READINESS_PATH="${6:?readiness path required}"
PUBLIC_HOSTNAME="${7:-_}"
TOKEN_FILE="${8:-/etc/cloudflared/token}"

export DEBIAN_FRONTEND=noninteractive

log() { printf '[KARETA bootstrap] %s\n' "$*"; }
fail() { log "FAIL: $*"; exit 1; }

if ! command -v apt-get >/dev/null 2>&1; then
  fail "this bootstrap currently supports Debian/Ubuntu WSL only"
fi

log "Installing runtime dependencies"
apt-get update -y
apt-get install -y ca-certificates curl git nginx php-cli php-fpm php-mysql
if ! command -v mysql >/dev/null 2>&1 && ! command -v mariadb >/dev/null 2>&1; then
  if ! apt-get install -y mysql-server; then
    apt-get install -y mariadb-server
  fi
fi

PHP_VERSION="$(php -r 'echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;')"
PHP_FPM_SERVICE="php${PHP_VERSION}-fpm.service"
PHP_FPM_SOCKET="/run/php/php${PHP_VERSION}-fpm.sock"

if systemctl list-unit-files mysql.service >/dev/null 2>&1; then
  DB_SERVICE="mysql.service"
elif systemctl list-unit-files mariadb.service >/dev/null 2>&1; then
  DB_SERVICE="mariadb.service"
else
  fail "neither mysql.service nor mariadb.service is available"
fi

systemctl enable --now "$PHP_FPM_SERVICE"
systemctl enable --now "$DB_SERVICE"
systemctl enable --now nginx.service

install -d -m 0755 "$(dirname "$KARETA_ROOT")"

if [ ! -d "$KARETA_ROOT/.git" ]; then
  if [ -e "$KARETA_ROOT" ] && [ "$(find "$KARETA_ROOT" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]; then
    fail "$KARETA_ROOT exists and is not an empty Git working tree"
  fi
  log "Cloning KARETA source"
  git clone "$REPO_URL" "$KARETA_ROOT"
else
  if [ -n "$(git -C "$KARETA_ROOT" status --porcelain --untracked-files=no)" ]; then
    fail "KARETA working tree has tracked local changes; refusing destructive update"
  fi
  log "Fetching KARETA source"
  git -C "$KARETA_ROOT" fetch --all --tags --prune
fi

if git -C "$KARETA_ROOT" show-ref --verify --quiet "refs/remotes/origin/$REF"; then
  git -C "$KARETA_ROOT" checkout -B "$REF" "origin/$REF"
  git -C "$KARETA_ROOT" pull --ff-only origin "$REF"
else
  git -C "$KARETA_ROOT" checkout --detach "$REF"
fi

if [ ! -f "$KARETA_ROOT/index.php" ]; then
  fail "KARETA root does not contain index.php"
fi

install -d -m 0755 /etc/kareta /usr/local/lib/kareta

cat >/etc/kareta/kareta.env <<EOF
KARETA_ROOT=$KARETA_ROOT
KARETA_URL=$LOCAL_URL
KARETA_RUNTIME_HEALTH=$RUNTIME_HEALTH_PATH
KARETA_READINESS=$READINESS_PATH
KARETA_PUBLIC_HOSTNAME=$PUBLIC_HOSTNAME
KARETA_CLOUDFLARE_TOKEN_FILE=$TOKEN_FILE
EOF
chmod 0600 /etc/kareta/kareta.env

cat >/usr/local/lib/kareta/wait-ready.sh <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
source /etc/kareta/kareta.env

for attempt in $(seq 1 60); do
  if curl -fsS --max-time 5 "${KARETA_URL}${KARETA_RUNTIME_HEALTH}" >/tmp/kareta-runtime-health.json 2>/dev/null; then
    code="$(curl -sS --max-time 10 -o /tmp/kareta-readiness.json -w '%{http_code}' "${KARETA_URL}${KARETA_READINESS}" || true)"
    if [ "$code" = "200" ] && grep -Eq '"ok"[[:space:]]*:[[:space:]]*true' /tmp/kareta-readiness.json; then
      exit 0
    fi
  fi
  sleep 2
done

echo "KARETA readiness timeout" >&2
[ -f /tmp/kareta-runtime-health.json ] && cat /tmp/kareta-runtime-health.json >&2 || true
[ -f /tmp/kareta-readiness.json ] && cat /tmp/kareta-readiness.json >&2 || true
exit 1
EOF
chmod 0755 /usr/local/lib/kareta/wait-ready.sh

cat >/usr/local/lib/kareta/run-cloudflared.sh <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
source /etc/kareta/kareta.env
bin="$(command -v cloudflared || true)"
[ -n "$bin" ] || { echo "cloudflared is not installed" >&2; exit 127; }
[ -s "$KARETA_CLOUDFLARE_TOKEN_FILE" ] || { echo "Cloudflare token file is missing" >&2; exit 78; }
exec "$bin" tunnel --no-autoupdate run --token-file "$KARETA_CLOUDFLARE_TOKEN_FILE"
EOF
chmod 0755 /usr/local/lib/kareta/run-cloudflared.sh

# Preserve a hand-managed nginx site. Only create/replace our own managed site.
NGINX_SITE=/etc/nginx/sites-available/kareta-vmp
if [ ! -f "$NGINX_SITE" ] || grep -q '^# managed-by-kareta-vmp$' "$NGINX_SITE"; then
cat >"$NGINX_SITE" <<EOF
# managed-by-kareta-vmp
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name $PUBLIC_HOSTNAME;
    root $KARETA_ROOT;
    index index.php;
    charset utf-8;

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "geolocation=(self), camera=(self), microphone=(self)" always;

    location ~ ^/(docs|storage/logs|storage/backups|tools)(/|$) { deny all; }
    location = /config.private.php { deny all; }
    location ~ /.(?!well-known/) { deny all; }

    location = /api/context/list { rewrite ^ /api/context.php?action=list last; }
    location = /api/context/current { rewrite ^ /api/context.php?action=current last; }
    location = /api/context/select { rewrite ^ /api/context.php?action=select last; }
    location = /api/capabilities/current { rewrite ^ /api/capabilities.php?action=current last; }
    location = /api/capabilities/check { rewrite ^ /api/capabilities.php?action=check last; }
    location = /api/identity/session { rewrite ^ /api/identity_session.php last; }

    location = /sw.js {
        add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
        try_files $uri =404;
    }
    location = /manifest.json {
        add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
        try_files $uri =404;
    }
    location = /index.php {
        add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:$PHP_FPM_SOCKET;
    }
    location ~* .(js|css)$ {
        add_header Cache-Control "no-store, no-cache, must-revalidate, max-age=0" always;
        try_files $uri =404;
    }
    location ~* .(png|jpe?g|webp|svg)$ {
        expires 30d;
        try_files $uri =404;
    }
    location ~ .php$ {
        try_files $uri =404;
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:$PHP_FPM_SOCKET;
    }
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
}
EOF
  rm -f /etc/nginx/sites-enabled/default
  ln -sfn "$NGINX_SITE" /etc/nginx/sites-enabled/kareta-vmp
else
  log "Preserving existing hand-managed nginx config: $NGINX_SITE"
fi

nginx -t
systemctl reload nginx.service

cat >/etc/systemd/system/kareta-ready.service <<EOF
[Unit]
Description=KARETA runtime readiness gate
After=network-online.target nginx.service $PHP_FPM_SERVICE $DB_SERVICE
Wants=network-online.target
Requires=nginx.service $PHP_FPM_SERVICE $DB_SERVICE

[Service]
Type=oneshot
EnvironmentFile=/etc/kareta/kareta.env
ExecStart=/usr/local/lib/kareta/wait-ready.sh
RemainAfterExit=yes
TimeoutStartSec=150

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/systemd/system/kareta-messaging-worker.service <<EOF
[Unit]
Description=KARETA Messaging Worker
After=kareta-ready.service
Requires=kareta-ready.service

[Service]
Type=simple
WorkingDirectory=$KARETA_ROOT
ExecStart=/usr/bin/php $KARETA_ROOT/tools/messaging_worker.php --watch --interval=2 --limit=25
Restart=always
RestartSec=3
User=www-data
Group=www-data
NoNewPrivileges=true
PrivateTmp=true
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/systemd/system/kareta-cloudflared.service <<EOF
[Unit]
Description=KARETA Cloudflare Tunnel
After=network-online.target kareta-ready.service
Wants=network-online.target
Requires=kareta-ready.service

[Service]
Type=simple
EnvironmentFile=/etc/kareta/kareta.env
ExecStart=/usr/local/lib/kareta/run-cloudflared.sh
Restart=always
RestartSec=5
TimeoutStopSec=20
NoNewPrivileges=true
PrivateTmp=true
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/systemd/system/kareta.target <<'EOF'
[Unit]
Description=KARETA.KZ Application Stack
Requires=kareta-ready.service
Wants=kareta-messaging-worker.service kareta-cloudflared.service
After=network-online.target kareta-ready.service

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable kareta-ready.service kareta-messaging-worker.service kareta-cloudflared.service kareta.target

if ! command -v cloudflared >/dev/null 2>&1; then
  log "WARN: cloudflared is not installed. KARETA_TUNNEL supply-chain integration is required before tunnel can start."
fi

log "Bootstrap completed"

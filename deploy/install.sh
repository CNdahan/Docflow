#!/bin/bash
#=============================================================================
# DocFlow 一键部署脚本
# 用法: sudo bash install.sh
# 支持: Ubuntu 22.04/24.04, Debian 12
#=============================================================================
set -euo pipefail

#------------- 配置区 (按需修改) -------------
DOMAIN="${DOCFLOW_DOMAIN:-docflow.example.com}"
DB_PASSWORD="${DOCFLOW_DB_PASSWORD:-$(openssl rand -hex 16)}"
JWT_SECRET="${DOCFLOW_JWT_SECRET:-$(openssl rand -hex 32)}"
ADMIN_USER="${DOCFLOW_ADMIN_USER:-admin}"
ADMIN_NAME="${DOCFLOW_ADMIN_NAME:-超级管理员}"
ADMIN_PASSWORD="${DOCFLOW_ADMIN_PASSWORD:-Admin@1234}"
USE_HTTPS="${DOCFLOW_USE_HTTPS:-no}"      # yes = certbot 申请证书, no = 纯 HTTP
#---------------------------------------------

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[DocFlow]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ---------- 0. 前置检查 ----------
[[ $EUID -ne 0 ]] && err "请以 root 用户运行: sudo bash install.sh"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

for f in docflow-server docflow-admin; do
    [[ -f "$SCRIPT_DIR/$f" ]] || err "缺少 $f，请先编译后端并放到脚本同目录"
done
[[ -d "$SCRIPT_DIR/migrations" ]] || err "缺少 migrations/ 目录"
[[ -d "$SCRIPT_DIR/frontend-dist" ]] || err "缺少 frontend-dist/ 目录 (前端 build 产物)"

log "开始部署 DocFlow"
log "  域名:     $DOMAIN"
log "  HTTPS:    $USE_HTTPS"
log "  管理员:   $ADMIN_USER"

# ---------- 1. 安装依赖 ----------
log "安装系统依赖..."
apt-get update -qq
apt-get install -y -qq postgresql nginx > /dev/null 2>&1

systemctl enable --now postgresql
systemctl enable --now nginx

# ---------- 2. 创建系统用户 ----------
if ! id docflow &>/dev/null; then
    useradd -r -s /bin/false -d /opt/docflow docflow
    log "创建系统用户 docflow"
fi

# ---------- 3. 创建目录 ----------
log "创建目录结构..."
mkdir -p /opt/docflow/migrations
mkdir -p /etc/docflow
mkdir -p /var/docflow/{files,tmp}
mkdir -p /var/log/docflow
mkdir -p /var/www/docflow
mkdir -p /backup/docflow

# ---------- 4. 复制文件 ----------
log "部署程序文件..."
cp "$SCRIPT_DIR/docflow-server" /opt/docflow/
cp "$SCRIPT_DIR/docflow-admin" /opt/docflow/
cp "$SCRIPT_DIR/migrations/"*.sql /opt/docflow/migrations/
chmod 755 /opt/docflow/docflow-server /opt/docflow/docflow-admin

cp -r "$SCRIPT_DIR/frontend-dist/"* /var/www/docflow/

if [[ -f "$SCRIPT_DIR/backup.sh" ]]; then
    cp "$SCRIPT_DIR/backup.sh" /opt/docflow/
    chmod 750 /opt/docflow/backup.sh
fi

chown -R docflow:docflow /opt/docflow /var/docflow /var/log/docflow

# ---------- 5. PostgreSQL ----------
log "初始化数据库..."
if sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='docflow'" | grep -q 1; then
    warn "数据库用户 docflow 已存在，跳过创建"
else
    sudo -u postgres psql -c "CREATE USER docflow WITH PASSWORD '$DB_PASSWORD';"
fi

if sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='docflow'" | grep -q 1; then
    warn "数据库 docflow 已存在，跳过创建"
else
    sudo -u postgres psql -c "CREATE DATABASE docflow OWNER docflow ENCODING 'UTF8';"
fi

# ---------- 6. 配置文件 ----------
log "写入配置文件..."
cat > /etc/docflow/config.yaml <<YAML
server:
  host: "0.0.0.0"
  port: 8080
  base_url: "https://$DOMAIN"

database:
  host: "127.0.0.1"
  port: 5432
  user: "docflow"
  password: "$DB_PASSWORD"
  dbname: "docflow"
  sslmode: "disable"
  max_open_conns: 25
  max_idle_conns: 5

storage:
  root: "/var/docflow/files"
  tmp_dir: "/var/docflow/tmp"
  max_file_size: 20971520
  max_inline_image_size: 5242880
  max_attachments_per_document: 10
  max_templates_per_document: 5
  max_attachments_per_submission: 10
  max_inline_images_per_document: 20

auth:
  jwt_secret: "$JWT_SECRET"
  access_token_ttl: 3600
  refresh_token_ttl: 604800
  bcrypt_cost: 12

initial_super:
  enabled: false

log:
  level: "info"
  format: "json"
  output: "/var/log/docflow/server.log"

cors:
  allow_origins:
    - "https://$DOMAIN"
    - "http://$DOMAIN"
YAML

cat > /etc/docflow/env <<ENV
DOCFLOW_DB_PASSWORD=$DB_PASSWORD
DOCFLOW_JWT_SECRET=$JWT_SECRET
ENV

chmod 600 /etc/docflow/config.yaml /etc/docflow/env
chown root:root /etc/docflow/config.yaml /etc/docflow/env

# ---------- 7. 数据库迁移 ----------
log "运行数据库迁移..."
sudo -u docflow /opt/docflow/docflow-server \
    -config /etc/docflow/config.yaml \
    -migrate \
    -migrations /opt/docflow/migrations \
    2>&1 | tail -3 || true

# ---------- 8. 创建管理员 ----------
log "创建管理员账号..."
echo "$ADMIN_PASSWORD" | sudo -u docflow /opt/docflow/docflow-admin \
    -config /etc/docflow/config.yaml \
    create-super \
    -username "$ADMIN_USER" \
    -realname "$ADMIN_NAME" \
    2>&1 | tail -3 || warn "管理员可能已存在，跳过"

# ---------- 9. systemd ----------
log "配置 systemd 服务..."
cat > /etc/systemd/system/docflow.service <<'UNIT'
[Unit]
Description=DocFlow Server
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=docflow
WorkingDirectory=/opt/docflow
EnvironmentFile=/etc/docflow/env
ExecStart=/opt/docflow/docflow-server -config /etc/docflow/config.yaml -migrations /opt/docflow/migrations
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/var/docflow /var/log/docflow

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable docflow
systemctl restart docflow
sleep 2

if systemctl is-active --quiet docflow; then
    log "docflow 服务启动成功"
else
    err "docflow 服务启动失败，请检查: journalctl -u docflow -n 30"
fi

# ---------- 10. Nginx ----------
log "配置 Nginx..."

if [[ "$USE_HTTPS" == "yes" ]]; then
    cat > /etc/nginx/sites-available/docflow <<NGINX
upstream docflow_backend {
    server 127.0.0.1:8080;
    keepalive 16;
}
server {
    listen 80;
    server_name $DOMAIN;
    return 301 https://\$host\$request_uri;
}
server {
    listen 443 ssl http2;
    server_name $DOMAIN;
    ssl_certificate     /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
    client_max_body_size 21m;
    root /var/www/docflow;
    index index.html;

    location /api/ {
        proxy_pass http://docflow_backend;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        proxy_request_buffering off;
        proxy_buffering off;
    }
    location = /healthz { proxy_pass http://docflow_backend; }
    location / { try_files \$uri \$uri/ /index.html; }
    location ~* \.(?:js|css|woff2?|ttf|svg|png|jpg|jpeg|gif|webp)$ {
        expires 30d;
        access_log off;
        add_header Cache-Control "public, immutable";
    }
    access_log /var/log/nginx/docflow_access.log;
    error_log  /var/log/nginx/docflow_error.log;
}
NGINX
    # 申请证书
    apt-get install -y -qq certbot python3-certbot-nginx > /dev/null 2>&1 || true
    certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --email "admin@$DOMAIN" || \
        warn "certbot 失败，请手动配置 HTTPS 证书"
else
    cat > /etc/nginx/sites-available/docflow <<NGINX
upstream docflow_backend {
    server 127.0.0.1:8080;
    keepalive 16;
}
server {
    listen 80;
    server_name $DOMAIN;
    client_max_body_size 21m;
    root /var/www/docflow;
    index index.html;

    location /api/ {
        proxy_pass http://docflow_backend;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        proxy_request_buffering off;
        proxy_buffering off;
    }
    location = /healthz { proxy_pass http://docflow_backend; }
    location / { try_files \$uri \$uri/ /index.html; }
    location ~* \.(?:js|css|woff2?|ttf|svg|png|jpg|jpeg|gif|webp)$ {
        expires 30d;
        access_log off;
        add_header Cache-Control "public, immutable";
    }
    access_log /var/log/nginx/docflow_access.log;
    error_log  /var/log/nginx/docflow_error.log;
}
NGINX
fi

ln -sf /etc/nginx/sites-available/docflow /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
nginx -t && systemctl reload nginx

# ---------- 11. 定时备份 ----------
log "配置定时备份..."
(crontab -l 2>/dev/null | grep -v docflow; echo "0 2 * * * /opt/docflow/backup.sh >> /var/log/docflow/backup.log 2>&1") | crontab -

# ---------- 12. 日志轮转 ----------
cat > /etc/logrotate.d/docflow <<'LOGROTATE'
/var/log/docflow/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 docflow docflow
    postrotate
        systemctl reload docflow > /dev/null 2>&1 || true
    endscript
}
LOGROTATE

# ---------- 13. 健康检查 ----------
log "验证部署..."
sleep 1
if curl -sf http://localhost:8080/healthz | grep -q '"ok":true'; then
    log "健康检查通过"
else
    warn "健康检查失败，请手动检查"
fi

# ---------- 完成 ----------
echo ""
echo "=============================================="
echo -e "${GREEN} DocFlow 部署完成!${NC}"
echo "=============================================="
echo ""
echo "  访问地址:  http://$DOMAIN"
echo "  管理员:    $ADMIN_USER"
echo "  管理密码:  $ADMIN_PASSWORD"
echo ""
echo "  数据库密码:  $DB_PASSWORD"
echo "  JWT 密钥:    $JWT_SECRET"
echo ""
echo "  请妥善保存以上信息!"
echo ""
echo "  常用命令:"
echo "    查看状态:  systemctl status docflow"
echo "    查看日志:  journalctl -u docflow -f"
echo "    重启服务:  systemctl restart docflow"
echo "    手动备份:  /opt/docflow/backup.sh"
echo "=============================================="

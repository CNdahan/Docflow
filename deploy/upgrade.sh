#!/bin/bash
#=============================================================================
# DocFlow 升级脚本
# 用法: sudo bash upgrade.sh
# 在发布包解压目录中运行
#=============================================================================
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[DocFlow]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

[[ $EUID -ne 0 ]] && err "请以 root 用户运行: sudo bash upgrade.sh"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

[[ -f "$SCRIPT_DIR/docflow-server" ]] || err "缺少 docflow-server"

# 1. 备份
log "升级前备份..."
if [[ -x /opt/docflow/backup.sh ]]; then
    /opt/docflow/backup.sh || warn "备份执行出错，继续升级"
fi

# 2. 停止服务
log "停止服务..."
systemctl stop docflow || warn "服务可能未在运行"

# 3. 备份旧二进制
log "备份旧版本..."
BACKUP_DATE=$(date +%Y%m%d-%H%M%S)
[[ -f /opt/docflow/docflow-server ]] && \
    cp /opt/docflow/docflow-server "/opt/docflow/docflow-server.bak.$BACKUP_DATE"

# 4. 替换二进制
log "更新程序文件..."
cp "$SCRIPT_DIR/docflow-server" /opt/docflow/
cp "$SCRIPT_DIR/docflow-admin" /opt/docflow/
chmod 755 /opt/docflow/docflow-server /opt/docflow/docflow-admin

# 5. 更新迁移脚本
if [[ -d "$SCRIPT_DIR/migrations" ]]; then
    log "更新迁移脚本..."
    cp "$SCRIPT_DIR/migrations/"*.sql /opt/docflow/migrations/
fi

# 6. 运行迁移
log "运行数据库迁移..."
sudo -u docflow /opt/docflow/docflow-server \
    -config /etc/docflow/config.yaml \
    -migrate \
    -migrations /opt/docflow/migrations \
    2>&1 | tail -3 || warn "迁移可能已是最新"

# 7. 更新前端
if [[ -d "$SCRIPT_DIR/frontend-dist" ]]; then
    log "更新前端文件..."
    rsync -a --delete "$SCRIPT_DIR/frontend-dist/" /var/www/docflow/
fi

# 8. 更新 systemd (如有变化)
if [[ -f "$SCRIPT_DIR/docflow.service" ]]; then
    cp "$SCRIPT_DIR/docflow.service" /etc/systemd/system/
    systemctl daemon-reload
fi

# 9. 启动服务
log "启动服务..."
systemctl start docflow
sleep 2

if systemctl is-active --quiet docflow; then
    log "服务启动成功"
else
    err "服务启动失败! 回滚: cp /opt/docflow/docflow-server.bak.$BACKUP_DATE /opt/docflow/docflow-server && systemctl start docflow"
fi

# 10. 健康检查
if curl -sf http://localhost:8080/healthz | grep -q '"ok":true'; then
    log "健康检查通过"
else
    warn "健康检查失败，请手动检查"
fi

echo ""
echo -e "${GREEN}升级完成!${NC}"
echo "  查看状态: systemctl status docflow"
echo "  查看日志: journalctl -u docflow -n 20"

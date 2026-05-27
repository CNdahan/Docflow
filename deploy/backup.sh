#!/bin/bash
# DocFlow 每日备份脚本
# 部署: /opt/docflow/backup.sh
# cron: 0 2 * * * /opt/docflow/backup.sh >> /var/log/docflow/backup.log 2>&1

set -euo pipefail

DATE=$(date +%Y%m%d-%H%M)
BACKUP_DIR="${BACKUP_DIR:-/backup/docflow}"
FILES_DIR="${FILES_DIR:-/var/docflow/files}"
DB_NAME="${DB_NAME:-docflow}"
DB_USER="${DB_USER:-docflow}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"

mkdir -p "$BACKUP_DIR"

echo "[$(date)] 开始备份"

# 1. PostgreSQL
echo "  - dumping database $DB_NAME"
pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_DIR/db_${DATE}.sql.gz"

# 2. 附件目录
echo "  - archiving files: $FILES_DIR"
tar -czf "$BACKUP_DIR/files_${DATE}.tar.gz" -C "$(dirname "$FILES_DIR")" "$(basename "$FILES_DIR")"

# 3. 清理过期
echo "  - pruning > ${RETENTION_DAYS} days"
find "$BACKUP_DIR" -name "*.gz" -mtime "+${RETENTION_DAYS}" -delete

echo "[$(date)] 备份完成 → $BACKUP_DIR"

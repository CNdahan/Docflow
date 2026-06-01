#!/bin/bash
#=============================================================================
# DocFlow 构建 + 打包脚本
# 用法: bash build.sh [linux/amd64|linux/arm64]
# 在开发机上运行，产出 release/ 目录和 tar.gz 包
#=============================================================================
set -euo pipefail

TARGET="${1:-linux/amd64}"
GOOS="${TARGET%%/*}"
GOARCH="${TARGET##*/}"
DATE=$(date +%Y%m%d)
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE="$ROOT/release"

echo "=== DocFlow Build ==="
echo "  Target: $GOOS/$GOARCH"
echo "  Date:   $DATE"
echo ""

# 清理
rm -rf "$RELEASE"
mkdir -p "$RELEASE"

# 1. 后端编译
echo "[1/4] 编译后端..."
cd "$ROOT/backend"
GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$RELEASE/docflow-server" ./cmd/server
GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$RELEASE/docflow-admin"  ./cmd/admin
echo "  -> docflow-server $(du -h "$RELEASE/docflow-server" | cut -f1)"
echo "  -> docflow-admin  $(du -h "$RELEASE/docflow-admin" | cut -f1)"

# 2. 前端编译
echo "[2/4] 编译前端..."
cd "$ROOT/frontend"
npm install --silent
npm run build --silent
cp -r dist "$RELEASE/frontend-dist"
echo "  -> frontend-dist/ $(du -sh "$RELEASE/frontend-dist" | cut -f1)"

# 3. 复制部署文件
echo "[3/4] 复制部署文件..."
cp -r "$ROOT/backend/migrations" "$RELEASE/migrations"
cp "$ROOT/backend/config.example.yaml" "$RELEASE/"
cp "$ROOT/deploy/install.sh" "$RELEASE/"
cp "$ROOT/deploy/backup.sh" "$RELEASE/"
cp "$ROOT/deploy/docflow.service" "$RELEASE/"
cp "$ROOT/deploy/nginx.conf.example" "$RELEASE/"
cp "$ROOT/deploy/DEPLOY.md" "$RELEASE/"
chmod +x "$RELEASE/install.sh" "$RELEASE/backup.sh"

# 4. 打包
echo "[4/4] 打包..."
ARCHIVE="docflow-${DATE}-${GOOS}-${GOARCH}.tar.gz"
cd "$ROOT"
tar -czf "$ARCHIVE" -C release .
echo "  -> $ARCHIVE $(du -h "$ARCHIVE" | cut -f1)"

echo ""
echo "=== 构建完成 ==="
echo ""
echo "发布包: $ROOT/$ARCHIVE"
echo ""
echo "部署方法:"
echo "  1. 上传 $ARCHIVE 到服务器"
echo "  2. tar -xzf $ARCHIVE -C /tmp/docflow-release"
echo "  3. cd /tmp/docflow-release && sudo bash install.sh"

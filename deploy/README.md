# DocFlow 部署指南

## 1. 服务器准备

- 系统:Ubuntu 22.04 LTS / Debian 12 / CentOS Stream 9 等
- 软件:
  - PostgreSQL 15+
  - Nginx 1.18+
  - 已安装 Go 1.22+ (仅编译时需要,部署只需二进制)
  - 已安装 Node.js 20+ (仅编译前端时需要)

## 2. 编译

### 后端
```bash
cd backend
# Linux 二进制
GOOS=linux GOARCH=amd64 go build -o docflow-server ./cmd/server
GOOS=linux GOARCH=amd64 go build -o docflow-admin  ./cmd/admin
```

### 前端
```bash
cd frontend
npm install
npm run build
# 产物在 frontend/dist/
```

## 3. 服务器目录布局

```
/opt/docflow/
├── docflow-server          # 二进制
├── docflow-admin           # CLI
└── migrations/             # SQL 迁移脚本
    └── 001_init.sql

/etc/docflow/
├── config.yaml             # 主配置 (敏感)
└── env                     # 环境变量 (敏感)

/var/docflow/
├── files/                  # 附件根目录
└── tmp/                    # 上传临时区

/var/log/docflow/           # 日志

/var/www/docflow/           # 前端静态文件 (= frontend/dist 的内容)
```

## 4. 部署步骤

### 4.1 创建系统用户
```bash
sudo useradd -r -s /bin/false docflow
sudo mkdir -p /opt/docflow /etc/docflow /var/docflow/files /var/docflow/tmp /var/log/docflow /var/www/docflow
sudo chown -R docflow:docflow /opt/docflow /var/docflow /var/log/docflow
```

### 4.2 拷贝二进制和迁移
```bash
sudo cp docflow-server docflow-admin /opt/docflow/
sudo cp -r backend/migrations /opt/docflow/
sudo chmod 755 /opt/docflow/docflow-server /opt/docflow/docflow-admin
```

### 4.3 拷贝前端
```bash
sudo cp -r frontend/dist/* /var/www/docflow/
```

### 4.4 初始化 PostgreSQL
```bash
sudo -u postgres psql <<EOF
CREATE USER docflow WITH PASSWORD '改成一个强密码';
CREATE DATABASE docflow OWNER docflow;
EOF
```

### 4.5 写配置
```bash
sudo cp backend/config.example.yaml /etc/docflow/config.yaml
sudo nano /etc/docflow/config.yaml
# 关键修改:
#   server.base_url = "https://docflow.example.com"
#   storage.root    = "/var/docflow/files"
#   storage.tmp_dir = "/var/docflow/tmp"
#   log.output      = "/var/log/docflow/server.log"
```

环境变量文件 `/etc/docflow/env`:
```
DOCFLOW_DB_PASSWORD=数据库密码
DOCFLOW_JWT_SECRET=32位以上随机串
```

生成 JWT secret:
```bash
openssl rand -hex 32
```

### 4.6 跑数据库迁移
```bash
sudo -u docflow /opt/docflow/docflow-server \
  -config /etc/docflow/config.yaml \
  -migrate \
  -migrations /opt/docflow/migrations
# 看到 "数据库迁移完成" 即可 Ctrl-C
```

### 4.7 创建初始 super 账号
```bash
sudo -u docflow /opt/docflow/docflow-admin \
  -config /etc/docflow/config.yaml \
  create-super \
  -username admin \
  -realname 超级管理员
# 交互式输入密码两次
```

### 4.8 配置 systemd
```bash
sudo cp deploy/docflow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable docflow
sudo systemctl start docflow
sudo systemctl status docflow
```

### 4.9 配置 Nginx
```bash
sudo cp deploy/nginx.conf.example /etc/nginx/sites-available/docflow
sudo nano /etc/nginx/sites-available/docflow
#   修改 server_name 和 ssl_certificate 路径
sudo ln -s /etc/nginx/sites-available/docflow /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

### 4.10 配置备份
```bash
sudo cp deploy/backup.sh /opt/docflow/
sudo chmod 750 /opt/docflow/backup.sh
sudo crontab -e
# 加一行:
#   0 2 * * * /opt/docflow/backup.sh >> /var/log/docflow/backup.log 2>&1
```

## 5. 验证

- 访问 `https://docflow.example.com/healthz` 应返回 `{"ok": true}`
- 访问首页应跳转到登录页
- 用 admin 账号登录,能看到顶级用户主页
- 走一遍 [设计文档 16.1 节] 的端到端验证脚本

## 6. 升级

```bash
sudo systemctl stop docflow
sudo cp new-docflow-server /opt/docflow/docflow-server
# 如有新迁移
sudo -u docflow /opt/docflow/docflow-server -config /etc/docflow/config.yaml -migrate -migrations /opt/docflow/migrations
sudo systemctl start docflow
```

前端单独升级:
```bash
sudo rsync -av --delete frontend/dist/ /var/www/docflow/
```

## 7. 故障排查

- 服务起不来 → `sudo journalctl -u docflow -n 50`
- 上传 413 → 检查 Nginx `client_max_body_size`,以及 Gin `UploadLimit` 中间件
- PDF 预览空白 → 浏览器控制台看 `/preview` 请求是否 200,以及 `Content-Type: application/pdf`
- 数据库连接超时 → 检查 PostgreSQL `pg_hba.conf` 是否允许本机连接

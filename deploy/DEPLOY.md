# DocFlow 部署文档

## 1. 环境要求

### 1.1 硬件最低配置

| 项目 | 最低 | 推荐 |
|------|------|------|
| CPU | 1 核 | 2 核+ |
| 内存 | 1 GB | 2 GB+ |
| 磁盘 | 10 GB | 50 GB+（取决于附件量） |

### 1.2 操作系统

- Ubuntu 22.04 / 24.04 LTS（推荐）
- Debian 12+
- CentOS Stream 9 / Rocky Linux 9

### 1.3 必需软件

| 软件 | 版本 | 用途 | 安装方式 |
|------|------|------|----------|
| PostgreSQL | 15+ | 数据库 | `apt install postgresql` |
| Nginx | 1.18+ | 反向代理 + 静态文件 | `apt install nginx` |
| Go | 1.22+ | 编译后端（仅构建机） | [go.dev/dl](https://go.dev/dl/) |
| Node.js | 20+ | 编译前端（仅构建机） | [nodejs.org](https://nodejs.org/) |

> 部署服务器本身不需要 Go 和 Node.js，只需要编译产物（二进制 + 静态文件）。

### 1.4 可选软件

| 软件 | 用途 |
|------|------|
| certbot | 自动申请 Let's Encrypt HTTPS 证书 |
| rsync | 增量同步部署文件 |
| logrotate | 日志轮转 |

---

## 2. 服务器目录布局

```
/opt/docflow/                # 程序目录
├── docflow-server           # Go 后端二进制
├── docflow-admin            # 管理员 CLI 工具
├── migrations/              # 数据库迁移脚本
│   ├── 001_init.sql
│   └── 002_multi_dept.sql
└── backup.sh                # 备份脚本

/etc/docflow/                # 配置 (敏感，权限 600)
├── config.yaml              # 主配置
└── env                      # 环境变量（数据库密码、JWT密钥）

/var/docflow/                # 运行时数据
├── files/                   # 附件存储根目录
│   ├── documents/           # 公文附件
│   ├── submissions/         # 上报附件
│   └── inline-images/       # 富文本内嵌图片
└── tmp/                     # 上传临时区

/var/log/docflow/            # 日志
├── server.log               # 后端日志
└── backup.log               # 备份日志

/var/www/docflow/            # 前端静态文件 (vue build 产物)

/backup/docflow/             # 备份文件
```

---

## 3. 构建

### 3.1 后端编译

```bash
cd backend

# Linux AMD64 (最常见的服务器架构)
GOOS=linux GOARCH=amd64 go build -o docflow-server ./cmd/server
GOOS=linux GOARCH=amd64 go build -o docflow-admin  ./cmd/admin

# Linux ARM64 (如果部署在 ARM 服务器)
# GOOS=linux GOARCH=arm64 go build -o docflow-server ./cmd/server
# GOOS=linux GOARCH=arm64 go build -o docflow-admin  ./cmd/admin
```

### 3.2 前端编译

```bash
cd frontend
npm install
npm run build
# 产物在 frontend/dist/
```

### 3.3 打包发布

```bash
# 创建发布包
mkdir -p release
cp backend/docflow-server backend/docflow-admin release/
cp -r backend/migrations release/
cp -r frontend/dist release/frontend-dist
cp backend/config.example.yaml release/
cp deploy/docflow.service deploy/nginx.conf.example deploy/backup.sh release/
tar -czf docflow-release-$(date +%Y%m%d).tar.gz -C release .
```

---

## 4. 首次部署（全新安装）

### 4.1 创建系统用户

```bash
sudo useradd -r -s /bin/false -d /opt/docflow docflow
```

### 4.2 创建目录

```bash
sudo mkdir -p /opt/docflow/migrations
sudo mkdir -p /etc/docflow
sudo mkdir -p /var/docflow/{files,tmp}
sudo mkdir -p /var/log/docflow
sudo mkdir -p /var/www/docflow
sudo mkdir -p /backup/docflow

sudo chown -R docflow:docflow /opt/docflow /var/docflow /var/log/docflow
sudo chmod 750 /etc/docflow
```

### 4.3 部署程序文件

```bash
# 解压发布包
tar -xzf docflow-release-*.tar.gz -C /tmp/docflow-release

# 后端
sudo cp /tmp/docflow-release/docflow-server /opt/docflow/
sudo cp /tmp/docflow-release/docflow-admin /opt/docflow/
sudo cp /tmp/docflow-release/migrations/* /opt/docflow/migrations/
sudo chmod 755 /opt/docflow/docflow-server /opt/docflow/docflow-admin

# 前端
sudo cp -r /tmp/docflow-release/frontend-dist/* /var/www/docflow/

# 备份脚本
sudo cp /tmp/docflow-release/backup.sh /opt/docflow/
sudo chmod 750 /opt/docflow/backup.sh
```

### 4.4 初始化 PostgreSQL

```bash
sudo -u postgres psql <<'EOF'
CREATE USER docflow WITH PASSWORD '替换为强密码';
CREATE DATABASE docflow OWNER docflow ENCODING 'UTF8';
EOF
```

### 4.5 编写配置

复制模板并编辑：

```bash
sudo cp /tmp/docflow-release/config.example.yaml /etc/docflow/config.yaml
sudo nano /etc/docflow/config.yaml
```

**必须修改的配置项**：

```yaml
server:
  base_url: "https://docflow.example.com"  # 改成你的域名

database:
  password: ""   # 留空，用环境变量提供

storage:
  root: "/var/docflow/files"
  tmp_dir: "/var/docflow/tmp"

auth:
  jwt_secret: ""  # 留空，用环境变量提供
  bcrypt_cost: 12  # 生产环境用 12

log:
  level: "info"
  format: "json"
  output: "/var/log/docflow/server.log"

cors:
  allow_origins:
    - "https://docflow.example.com"  # 改成你的域名
```

创建环境变量文件（存放敏感信息）：

```bash
# 生成 JWT 密钥
JWT_SECRET=$(openssl rand -hex 32)

sudo tee /etc/docflow/env > /dev/null <<EOF
DOCFLOW_DB_PASSWORD=你的数据库密码
DOCFLOW_JWT_SECRET=$JWT_SECRET
EOF

sudo chmod 600 /etc/docflow/env
sudo chown root:root /etc/docflow/env
```

### 4.6 运行数据库迁移

```bash
sudo -u docflow /opt/docflow/docflow-server \
  -config /etc/docflow/config.yaml \
  -migrate \
  -migrations /opt/docflow/migrations
```

### 4.7 创建初始管理员账号

```bash
sudo -u docflow /opt/docflow/docflow-admin \
  -config /etc/docflow/config.yaml \
  create-super \
  -username admin \
  -realname 超级管理员
# 按提示输入密码
```

### 4.8 部署 systemd 服务

```bash
sudo cp /tmp/docflow-release/docflow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable docflow
sudo systemctl start docflow

# 检查状态
sudo systemctl status docflow
sudo journalctl -u docflow -n 20
```

### 4.9 配置 Nginx

```bash
sudo cp /tmp/docflow-release/nginx.conf.example /etc/nginx/sites-available/docflow

# 编辑，修改 server_name 和 SSL 证书路径
sudo nano /etc/nginx/sites-available/docflow

sudo ln -sf /etc/nginx/sites-available/docflow /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

**如果使用 Let's Encrypt**：

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d docflow.example.com
```

**如果是内网部署（无 HTTPS）**，使用简化版 Nginx 配置：

```nginx
server {
    listen 80;
    server_name docflow.internal;
    client_max_body_size 21m;
    root /var/www/docflow;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 60s;
        proxy_request_buffering off;
    }

    location = /healthz {
        proxy_pass http://127.0.0.1:8080;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### 4.10 配置定时备份

```bash
sudo crontab -e -u root
# 添加以下行（每天凌晨 2 点备份）：
0 2 * * * /opt/docflow/backup.sh >> /var/log/docflow/backup.log 2>&1
```

### 4.11 配置日志轮转（可选）

```bash
sudo tee /etc/logrotate.d/docflow > /dev/null <<'EOF'
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
EOF
```

---

## 5. 验证清单

部署完成后依次检查：

- [ ] `curl http://localhost:8080/healthz` 返回 `{"ok":true}`
- [ ] 浏览器访问域名，显示登录页
- [ ] 用 admin 账号登录，进入管理后台
- [ ] 创建一个部门
- [ ] 创建一个普通用户，分配到该部门
- [ ] 发布一条公文（含 PDF 附件）
- [ ] 用普通用户登录，能看到公文
- [ ] PDF 在线预览正常
- [ ] 上传上报附件并提交
- [ ] 管理员查看纵览，数据正确

---

## 6. 升级流程

```bash
# 1. 停止服务
sudo systemctl stop docflow

# 2. 备份（保险起见）
sudo /opt/docflow/backup.sh

# 3. 替换二进制
sudo cp new-docflow-server /opt/docflow/docflow-server
sudo cp new-docflow-admin /opt/docflow/docflow-admin
sudo chmod 755 /opt/docflow/docflow-server /opt/docflow/docflow-admin

# 4. 复制新迁移脚本（如果有）
sudo cp new-migrations/*.sql /opt/docflow/migrations/

# 5. 运行迁移
sudo -u docflow /opt/docflow/docflow-server \
  -config /etc/docflow/config.yaml -migrate -migrations /opt/docflow/migrations

# 6. 更新前端
sudo rsync -av --delete new-frontend-dist/ /var/www/docflow/

# 7. 启动
sudo systemctl start docflow
sudo systemctl status docflow
```

---

## 7. 故障排查

| 现象 | 排查 |
|------|------|
| 服务起不来 | `sudo journalctl -u docflow -n 50` 看日志 |
| 502 Bad Gateway | 确认 docflow 进程在跑：`systemctl status docflow` |
| 上传返回 413 | Nginx `client_max_body_size` 是否 >= 21m |
| PDF 预览空白 | 浏览器 F12 看 `/api/v1/attachments/:id/preview` 是否 200 |
| 数据库连不上 | 检查 `pg_hba.conf` 是否允许本地连接，密码是否正确 |
| CORS 报错 | `config.yaml` 的 `cors.allow_origins` 是否包含你的域名 |
| 前端白屏 | Nginx `try_files` 是否配了 `/index.html` fallback |
| 登录后 401 | JWT secret 是否配置正确，`/etc/docflow/env` 中的变量是否加载 |

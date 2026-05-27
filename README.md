# DocFlow

轻量级公文下发与上报管理系统。Go + Vue 3,前后端分离,内网/公网均可部署。

## 特性

- **三层组织**:顶级用户 / 部门用户 / 普通用户
- **公文下发**:富文本正文 + 阅读附件 + 上报模板
- **闭环上报**:用户提交 → 部门审核 → 退回修改 → 重提覆盖
- **多维纵览**:顶级全局看,部门部门看,用户自己看
- **PDF 在线预览**:浏览器内直接看,不必下载
- **截止管理**:超期仍可上报,状态自动区分准时/逾期
- **附件双向多格式**:PDF / Office / 图片,单文件 ≤ 20MB

## 项目结构

```
docflow/
├── backend/          # Go 1.22 + Gin + GORM
├── frontend/         # Vue 3 + Vite + Element Plus
├── deploy/           # Nginx / systemd / 备份脚本
└── docs/             # 设计文档
```

## 快速开始 (本地开发)

### 1. 后端

```bash
cd backend
cp config.example.yaml config.yaml
# 编辑 config.yaml,主要填:
#   database.password
#   auth.jwt_secret
#   storage.root  (本地任意目录)

# 数据库就绪后跑迁移
go run ./cmd/server -config config.yaml -migrate

# 创建顶级用户
go run ./cmd/admin -config config.yaml create-super -username admin -realname 管理员

# 启动
go run ./cmd/server -config config.yaml
```

### 2. 前端

```bash
cd frontend
npm install
npm run dev
# 默认监听 http://localhost:5173,自动代理 /api 到后端 8080
```

打开浏览器访问 `http://localhost:5173`,用 admin 账号登录。

## 生产部署

详见 [deploy/README.md](deploy/README.md)。

## 设计文档

详细的业务流程、数据模型、API 契约、安全设计参见 [docs/design.md](docs/design.md)。

## 实施进度

- [x] **M1 - MVP 闭环** (本次)
  - 认证 + 用户/部门管理 + 公文发布 (DEPARTMENT/ALL_USERS 范围) + 上报 + 单公文纵览
- [ ] **M2 - 完整业务**
  - 部门用户角色 + OWN_DEPARTMENT 范围 + 退回机制 + 公文编辑 + 撤回 + 部门纵览 + 全局纵览
- [ ] **M3 - 体验与运维**
  - 站内通知 + 操作日志 + Excel 导出 + 健康检查 + 速率限制

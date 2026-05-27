# DocFlow —— 轻量级公文下发系统  设计与实施详细文档

> 版本: v0.9（设计阶段）
> 适用范围: 内网/小型组织的公文下发与上报场景
> 文档目标: 一份可以直接用来评审、立项、并指导编码的完整中文设计稿

---

## 0. 文档导航

| 章节 | 内容 |
|---|---|
| [1 业务背景与目标](#1-业务背景与目标) | 我们要解决什么问题 |
| [2 名词约定](#2-名词约定) | 防止歧义的术语对照 |
| [3 用户角色与权限矩阵](#3-用户角色与权限矩阵) | 三层角色完整能力清单 |
| [4 核心业务流程](#4-核心业务流程) | 发文 → 上报 → 退回 → 重提 完整时序 |
| [5 公文范围模型](#5-公文范围模型) | 三种 target_scope 的语义 |
| [6 上报状态机](#6-上报状态机) | 五种状态 + 完整迁移图 |
| [7 数据模型设计](#7-数据模型设计) | 表结构 + DDL + 索引 + 冗余说明 |
| [8 附件与存储设计](#8-附件与存储设计) | 目录布局 + 安全 + 预览 |
| [9 API 接口设计](#9-api-接口设计) | 全量接口契约 + 请求响应示例 |
| [10 前端页面设计](#10-前端页面设计) | 按角色拆解的页面线框 |
| [11 技术栈与依赖](#11-技术栈与依赖) | 选型理由 |
| [12 项目目录结构](#12-项目目录结构) | 后端/前端/部署的物理布局 |
| [13 配置与环境](#13-配置与环境) | YAML 配置项清单 |
| [14 安全设计](#14-安全设计) | 密码/JWT/上传/XSS/越权 |
| [15 实施分期](#15-实施分期-milestone) | 三个 milestone 详细任务 |
| [16 验证方案](#16-验证方案) | 功能/安全/部署的验收标准 |
| [17 部署运维](#17-部署运维) | Nginx + systemd + 备份 |
| [18 不在本次范围](#18-不在本次范围) | 明确说不做什么 |
| [19 待最终确认事项](#19-待最终确认事项) | 等用户拍板的细节 |

---

## 1. 业务背景与目标

### 1.1 背景

需要一个**面向小型/中型组织**的轻量级公文下发与上报管理系统，替代目前依赖微信/邮件/QQ 群的非结构化沟通方式。重点解决：

- **谁该看什么公文**没有清单，靠人盯
- **谁没交**只能挨个问，靠 Excel 维护
- **上报附件版本混乱**，群里翻聊天记录
- **逾期/退回**没有标准化流程
- **历史可追溯性差**，需要审计时翻不到底

### 1.2 目标

- **三层组织结构**：顶级、部门、普通用户，覆盖绝大多数小型单位
- **公文 + 附件**：富文本正文 + 阅读附件 + 上报模板
- **闭环上报**：用户提交 → 部门审核 → 退回修改 → 重新提交
- **多维纵览**：顶级全局看、部门部门看、用户自己看
- **PDF 在线预览**：避免反复下载查看
- **可部署在物理服务器上**：内网/公网域名皆可

### 1.3 非目标（明确说不做）

- 不做工作流引擎（多级审批/会签/转办）
- 不做附件版本历史（覆盖即可）
- 不做移动端 App（响应式 Web 足够）
- 不做国际化（仅简体中文）
- 不做全文检索（可后续接）

---

## 2. 名词约定

| 名词 | 含义 |
|---|---|
| **公文** (Document) | 系统中的一条下发记录，包含富文本正文 + 附件 + 目标范围 + 截止日期 |
| **阅读附件** (Reading Attachment) | 公文带的、给下级查看的参考材料 |
| **模板附件** (Template Attachment) | 公文带的、给下级下载后填写再上传的空白模板 |
| **上报附件** (Submission Attachment) | 下级用户上传的反馈材料 |
| **上报** (Submission) | 一条"用户对某条公文的响应"记录 |
| **退回** (Return) | 部门或顶级用户认为上报不合格，打回让用户重新提交 |
| **范围** (Target Scope) | 公文的接收对象类型：指定部门 / 全体用户 / 本部门 |
| **物化目标** (Document Target) | 发布时根据范围预先生成的具体接收人记录 |
| **纵览** (Overview / Statistics) | 上级查看下级上报情况的汇总视图 |

---

## 3. 用户角色与权限矩阵

### 3.1 三种角色

| 代号 | 名称 | 数量 | 主要职责 |
|---|---|---|---|
| `super` | 顶级用户 | 通常 1~3 人 | 系统总管，管部门、管所有用户、发全局公文、看全局纵览 |
| `dept` | 部门用户 | 每部门 1~N 人 | 本部门负责人，管本部门 user、向本部门发公文、审核退回本部门上报 |
| `user` | 普通用户 | 不限 | 接收公文、查看附件、上传上报、查自己的历史 |

### 3.2 完整权限矩阵

| # | 操作分组 | 操作 | super | dept | user |
|---|---|---|:-:|:-:|:-:|
| 1 | 部门管理 | 创建/编辑/禁用部门 | ✅ | ❌ | ❌ |
| 2 | 用户管理 | 创建/编辑/禁用 super 账号 | ✅ | ❌ | ❌ |
| 3 |  | 创建/编辑/禁用 dept 账号 | ✅ | ❌ | ❌ |
| 4 |  | 创建/编辑/禁用任意部门下 user 账号 | ✅ | ❌ | ❌ |
| 5 |  | 创建/编辑/禁用**本部门** user 账号 | ✅ | ✅ | ❌ |
| 6 |  | 重置任意用户密码 | ✅ | 本部门 | 仅自己 |
| 7 | 公文管理 | 发布公文（范围=指定部门） | ✅ | ❌ | ❌ |
| 8 |  | 发布公文（范围=全员） | ✅ | ❌ | ❌ |
| 9 |  | 发布公文（范围=本部门） | ✅ | ✅ | ❌ |
| 10 |  | 编辑自己发布的公文 | ✅ | ✅ | ❌ |
| 11 |  | 撤回自己发布的公文 | ✅ | ✅ | ❌ |
| 12 |  | 查看分配给自己的公文 | ✅ | ✅ | ✅ |
| 13 | 上报管理 | 上传/重新上报本人的上报附件 | ❌ | ❌ | ✅ |
| 14 |  | 退回任意上报 | ✅ | ❌ | ❌ |
| 15 |  | 退回本部门 user 的上报 | ✅ | ✅ | ❌ |
| 16 | 纵览统计 | 全局纵览（所有公文 × 所有部门） | ✅ | ❌ | ❌ |
| 17 |  | 部门纵览（本部门相关公文） | ✅ | ✅ | ❌ |
| 18 |  | 单公文上报详情 | ✅ | 本部门 | 仅自己 |
| 19 | 审计 | 查看公文修订日志 | ✅ | 本部门 | ❌ |
| 20 |  | 查看上报动作流水 | ✅ | 本部门 | 仅自己 |

### 3.3 关键边界场景

- **super 没有部门归属**：`users.department_id` 允许为 NULL；发文时如选择"本部门"是无效行为，前端禁用该选项。
- **dept 跨部门访问**：dept 用户无论如何不能看到/操作非本部门数据，即使知道 ID 也不行（中间件强制校验）。
- **user 跨用户访问**：user 不能看到他人的上报详情，只能看自己的。

---

## 4. 核心业务流程

### 4.1 发布公文流程（顶级 → 部门）

```
super                       系统                          user (技术部)
  │                          │                               │
  │ 1. 打开"发布公文"页      │                               │
  │ ──────────────────────► │                               │
  │                          │                               │
  │ 2. 选范围=DEPARTMENT     │                               │
  │    选目标部门=[技术部]    │                               │
  │    填正文(富文本)         │                               │
  │    传阅读附件 + 模板附件  │                               │
  │    设截止=2026-06-01     │                               │
  │ ──────────────────────► │                               │
  │                          │ 3. 事务开始                   │
  │                          │  - 写 documents               │
  │                          │  - 写 document_targets        │
  │                          │  - 为每个目标 user 写一条       │
  │                          │    submissions(PENDING)       │
  │                          │  - 移动附件到正式目录          │
  │                          │ 4. 事务提交                   │
  │ ◄──────── 201 Created ──┤                               │
  │                          │                               │
  │                          │ 5. (可选) 站内通知推送          │
  │                          │ ───────────────────────────► │
  │                          │                               │
  │                          │                               │ 6. 用户登录看到红点
  │                          │                               │    打开公文,看正文/预览PDF
```

### 4.2 上报流程（普通用户提交 → 部门审核 → 退回 → 重提）

```
user                        系统                          dept
  │                          │                               │
  │ 1. 打开公文详情            │                               │
  │ 2. 下载模板填写完          │                               │
  │ 3. 上传上报附件 + 备注    │                               │
  │ ─── POST /submissions ──►│                               │
  │                          │ 4. 状态机迁移                  │
  │                          │    PENDING → SUBMITTED        │
  │                          │   (或 OVERDUE→SUBMITTED_LATE)  │
  │                          │    写 submission_actions      │
  │ ◄────── 200 OK ─────────│                               │
  │                          │                               │
  │                          │       5. 部门查看纵览          │
  │                          │ ◄─────────────────────────── │
  │                          │                               │
  │                          │       6. 觉得不行,点退回      │
  │                          │       填原因="格式不对,请重做" │
  │                          │ ◄── POST /:id/return ────── │
  │                          │ 7. 状态机迁移                  │
  │                          │    SUBMITTED → RETURNED       │
  │                          │    return_count += 1          │
  │                          │    return_reason = "..."      │
  │                          │    (旧附件保留到重提时)        │
  │                          │ ─────── 200 OK ────────────► │
  │                          │                               │
  │ 8. 收到通知,看到退回原因  │                               │
  │ 9. 重新填表               │                               │
  │ 10. 重新提交 (覆盖旧附件) │                               │
  │ ─── POST /submissions ──►│                               │
  │                          │ 11. 删除旧附件文件             │
  │                          │     上传新附件                 │
  │                          │     RETURNED → SUBMITTED      │
  │                          │     (若已过 deadline 则        │
  │                          │      → SUBMITTED_LATE)        │
  │ ◄────── 200 OK ─────────│                               │
```

### 4.3 公文编辑流程（发布后修改）

```
publisher                   系统
  │                          │
  │ 1. 进入已发布公文         │
  │ 2. 改正文/加附件/改 ddl   │
  │ ─── PATCH /documents/:id│
  │                          │ 3. 事务开始
  │                          │  - 更新 documents
  │                          │  - 增减 attachments
  │                          │  - 写 document_revisions
  │                          │    (CONTENT/ATTACHMENT/DEADLINE)
  │                          │ 4. 事务提交
  │ ◄────── 200 OK ─────────│
  │                          │
  │                          │ 5. 已提交用户的状态不变
  │                          │    (修改不影响已提交记录)
  │                          │ 6. 给受影响用户推通知
  │                          │    "公文已更新,请查看"
```

**注意**：编辑公文**不会**重置已提交用户的状态。如果改动重大，发布者应该撤回后重发。

---

## 5. 公文范围模型

### 5.1 三种 `target_scope`

| 枚举值 | 中文 | 谁能发 | 实际接收人 | 纵览聚合方式 |
|---|---|---|---|---|
| `DEPARTMENT` | 指定部门 | super | 选中的一个/多个部门下所有 user | 按部门聚合：部门 → 已交/未交/逾期/退回数 |
| `ALL_USERS` | 全员广播 | super | 全平台所有未禁用的 user | 按用户聚合：用户 → 状态 |
| `OWN_DEPARTMENT` | 本部门 | dept | 该 dept 用户所在部门的所有 user | 按用户聚合：用户 → 状态 |

### 5.2 范围选择 UI 设计

```
┌──── 发布公文 ────────────────────────────┐
│  范围: ⦿ 指定部门  ◯ 全员广播           │
│                                          │
│  目标部门: [☑ 技术部] [☐ 财务部] [☐ ...] │
│           (多选,至少选一个)              │
│                                          │
└──────────────────────────────────────────┘
```

dept 用户登录后看到的发布页：

```
┌──── 发布公文 ────────────────────────────┐
│  范围: ⦿ 本部门 (技术部)                  │
│  ↑ 不可选,自动锁定                       │
└──────────────────────────────────────────┘
```

### 5.3 物化原因

发布时立即把"应收人列表"写入 `document_targets` 和 `submissions`，而不是查询时动态计算，原因：

1. **历史一致性**：日后用户调岗，统计时不会变样
2. **查询性能**：纵览查询只 JOIN 一张表，不用层层关联
3. **逻辑简单**：状态机只需要维护已存在的 submission 行

代价是发布瞬间多写 N 条记录（N = 接收人数），但对于小组织 N 通常 < 500，可接受。

---

## 6. 上报状态机

### 6.1 五种状态

| 代号 | 中文 | 含义 |
|---|---|---|
| `PENDING` | 待上报 | 已分配但未提交，且未过截止 |
| `SUBMITTED` | 准时已交 | 在截止前提交 |
| `SUBMITTED_LATE` | 逾期已交 | 截止后才提交 |
| `OVERDUE` | 逾期未交 | 已过截止，至今未提交（虚拟状态，**查询时实时计算**） |
| `RETURNED` | 已退回 | 部门/顶级退回，等待重提 |

### 6.2 完整迁移图

```
                                  退回(必填原因)
                              ┌──────────────────┐
                              ▼                  │
PENDING ──提交(<=ddl)────► SUBMITTED ────────────┤
   │                                             │
   │                                             │
   │ (时间到 ddl 仍未提交)                       │
   ▼                          退回(必填原因)     │
[OVERDUE 虚拟]            ┌──────────────────────┤
   │                      ▼                      │
   └──提交(>ddl)─► SUBMITTED_LATE ───────────────┤
                                                 │
                                                 ▼
                                              RETURNED
                                                 │
                                                 │ 重新提交(覆盖附件)
                                                 │
                              ┌──────────────────┴──┐
                              ▼                     ▼
                          (now<=ddl)            (now>ddl)
                          SUBMITTED          SUBMITTED_LATE
```

### 6.3 状态判定函数（实时计算 OVERDUE）

```
def display_status(sub, doc):
    if sub.current_status == 'PENDING' and doc.deadline and doc.deadline < now:
        return 'OVERDUE'      # 不入库,仅展示
    return sub.current_status
```

数据库里只存 `PENDING/SUBMITTED/SUBMITTED_LATE/RETURNED` 这 4 种**实状态**，`OVERDUE` 是基于 deadline 实时推导的虚拟状态。
好处：**不需要 cron 把 PENDING 改成 OVERDUE**，状态永远一致。

### 6.4 重提覆盖附件的细节

- 重新提交时，先把**旧的 submission attachments 物理删除**（文件+表记录）
- 再上传新附件
- 整个过程包在一个事务里，失败回滚不影响旧附件
- 已退回未重提期间，旧附件仍然可见（部门审核时可以重新看）

### 6.5 退回规则

- **谁能退回**：super 可退任意，dept 只能退本部门 user 的上报
- **何时能退**：仅 `SUBMITTED` 或 `SUBMITTED_LATE` 状态可退
- **原因必填**：最少 5 字，最多 500 字
- **退回次数**：累计到 `submissions.return_count`，便于统计反复退回的情况

---

## 7. 数据模型设计

### 7.1 ER 概览

```
departments ──┬──< users ──┬──< documents (publisher_id)
              │            │
              │            └──< submissions (user_id)
              │
              └──< document_targets (department_id)

documents ──┬──< document_targets
            ├──< attachments (owner_type=DOCUMENT)
            ├──< document_revisions
            └──< submissions

submissions ──┬──< attachments (owner_type=SUBMISSION)
              └──< submission_actions
```

### 7.2 完整 DDL（PostgreSQL）

```sql
-- 部门
CREATE TABLE departments (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(64) NOT NULL UNIQUE,
    disabled    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 用户
CREATE TABLE users (
    id              SERIAL PRIMARY KEY,
    username        VARCHAR(64) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(16) NOT NULL CHECK (role IN ('super','dept','user')),
    department_id   INT REFERENCES departments(id),
    real_name       VARCHAR(64),
    disabled        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_role_dept ON users(role, department_id);

-- 公文
CREATE TABLE documents (
    id              SERIAL PRIMARY KEY,
    title           VARCHAR(200) NOT NULL,
    content_html    TEXT NOT NULL,
    publisher_id    INT NOT NULL REFERENCES users(id),
    publisher_dept  INT REFERENCES departments(id),
    target_scope    VARCHAR(16) NOT NULL CHECK (target_scope IN ('DEPARTMENT','ALL_USERS','OWN_DEPARTMENT')),
    deadline        TIMESTAMPTZ,
    status          VARCHAR(16) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RECALLED')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_documents_publisher ON documents(publisher_id);
CREATE INDEX idx_documents_scope_deadline ON documents(target_scope, deadline);

-- 公文目标（物化的应收人）
CREATE TABLE document_targets (
    document_id    INT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    department_id  INT REFERENCES departments(id),
    user_id        INT NOT NULL REFERENCES users(id),
    PRIMARY KEY (document_id, user_id)
);
CREATE INDEX idx_targets_user ON document_targets(user_id);

-- 附件
CREATE TABLE attachments (
    id           SERIAL PRIMARY KEY,
    owner_type   VARCHAR(16) NOT NULL CHECK (owner_type IN ('DOCUMENT','SUBMISSION','INLINE')),
    owner_id     INT NOT NULL,
    purpose      VARCHAR(16) CHECK (purpose IN ('READING','TEMPLATE')),
    file_name    VARCHAR(255) NOT NULL,
    stored_path  VARCHAR(500) NOT NULL,
    mime_type    VARCHAR(100) NOT NULL,
    size_bytes   BIGINT NOT NULL,
    uploader_id  INT NOT NULL REFERENCES users(id),
    uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_attachments_owner ON attachments(owner_type, owner_id);

-- 上报
CREATE TABLE submissions (
    id              SERIAL PRIMARY KEY,
    document_id     INT NOT NULL REFERENCES documents(id),
    user_id         INT NOT NULL REFERENCES users(id),
    department_id   INT REFERENCES departments(id),
    current_status  VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                    CHECK (current_status IN ('PENDING','SUBMITTED','SUBMITTED_LATE','RETURNED')),
    submitted_at    TIMESTAMPTZ,
    return_reason   TEXT,
    return_count    INT NOT NULL DEFAULT 0,
    note            TEXT,
    last_action_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_id, user_id)
);
CREATE INDEX idx_submissions_doc_status ON submissions(document_id, current_status);
CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_submissions_dept ON submissions(department_id);

-- 上报动作流水
CREATE TABLE submission_actions (
    id              SERIAL PRIMARY KEY,
    submission_id   INT NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    action_type     VARCHAR(16) NOT NULL CHECK (action_type IN ('SUBMIT','RETURN','RESUBMIT')),
    operator_id     INT NOT NULL REFERENCES users(id),
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sub_actions_submission ON submission_actions(submission_id);

-- 公文修订日志
CREATE TABLE document_revisions (
    id            SERIAL PRIMARY KEY,
    document_id   INT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    editor_id     INT NOT NULL REFERENCES users(id),
    change_type   VARCHAR(16) NOT NULL CHECK (change_type IN ('CONTENT','ATTACHMENT','DEADLINE','META')),
    diff_summary  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_doc_revisions_doc ON document_revisions(document_id);
```

### 7.3 冗余字段说明

| 表 | 冗余字段 | 来源 | 为什么冗余 |
|---|---|---|---|
| `documents` | `publisher_dept` | `users.department_id` | 鉴权时不用 JOIN users |
| `submissions` | `department_id` | `users.department_id`（发布时） | 用户调岗后历史归属不变；纵览快查 |

### 7.4 初始化数据

```sql
-- 系统启动后由配置或 CLI 创建第一个 super
INSERT INTO users (username, password_hash, role, real_name)
VALUES ('admin', '<bcrypt>', 'super', '超级管理员');
```

---

## 8. 附件与存储设计

### 8.1 物理目录布局

```
<storage_root>/                # 配置项 storage.root,例: /var/docflow/files
├── documents/
│   └── 1/                     # doc_id=1
│       ├── reading/
│       │   └── 17__通知正文.pdf       # 17=attachment_id
│       └── template/
│           └── 18__申报模板.xlsx
├── submissions/
│   └── 42/                    # submission_id=42
│       ├── 33__我的申报表.pdf
│       └── 34__补充材料.docx
├── inline-images/             # 富文本内嵌
│   └── 2026-05/
│       └── 4f8a...uuid.png
└── tmp/                       # 上传临时区,定期清理
```

### 8.2 文件名安全策略

- 数据库存原始文件名（含中文，用于下载时显示）
- 磁盘文件名 = `<id>__<safe_name>`，其中 safe_name：
  - 去掉 `..` `/` `\` 控制字符
  - 替换 Windows 保留名（CON、AUX 等）
  - 长度截断到 200 字节
- 防穿越：所有路径拼接前都用 `filepath.Clean` 并校验是否在 `storage_root` 之下

### 8.3 上传白名单

| 用途 | 允许 MIME | 允许扩展名 |
|---|---|---|
| 阅读附件 | PDF, Word(docx/doc), Excel(xlsx/xls), PPT(pptx/ppt), 图片(jpg/png) | 同左 |
| 模板附件 | PDF, Word, Excel, PPT, ZIP | 同左 |
| 上报附件 | PDF, Word, Excel, PPT, 图片 | 同左 |
| 内嵌图片 | jpg/png/gif/webp | 同左 |

**双重校验**：
1. 前端按扩展名拦截
2. 后端用 `mimetype.Detect` 读 magic number 校验真实类型，与扩展名不匹配则拒绝

### 8.4 大小与数量限制

| 项 | 上限 |
|---|---|
| 单文件 | 20MB |
| 单公文阅读附件 | 10 个 |
| 单公文模板附件 | 5 个 |
| 单次上报附件 | 10 个 |
| 内嵌图片单图 | 5MB |
| 富文本内嵌图片总数 | 20 张/篇 |

### 8.5 PDF 预览

- 后端响应 `Content-Type: application/pdf`、`Content-Disposition: inline; filename="..."`
- 前端 `<vue-pdf-embed :source="url" />` 直接渲染
- 不需要任何转换服务
- 非 PDF 文件预览接口返回 415

---

## 9. API 接口设计

> 所有接口统一前缀 `/api/v1`
> 认证：除登录外都需要 `Authorization: Bearer <jwt>`
> 错误格式：`{ "code": "ERR_CODE", "message": "...", "details": {...} }`
> 时间字段统一 ISO 8601（如 `2026-05-27T10:00:00+08:00`）

### 9.1 认证

#### POST `/auth/login`
```json
// 请求
{ "username": "admin", "password": "******" }

// 响应 200
{
  "access_token": "eyJ...",
  "refresh_token": "...",
  "expires_in": 3600,
  "user": { "id": 1, "username": "admin", "role": "super", "real_name": "..." }
}
```

#### POST `/auth/refresh`
```json
// 请求
{ "refresh_token": "..." }
// 响应同 login
```

#### POST `/auth/logout`
让 token 进入黑名单（Redis 或内存）。

### 9.2 部门

#### GET `/departments`
```json
// 响应
{ "items": [ { "id": 1, "name": "技术部", "user_count": 12, "disabled": false } ] }
```

#### POST `/departments`
```json
// 请求
{ "name": "财务部" }
// 响应 201 同上 item
```

#### PATCH `/departments/:id`
```json
{ "name": "新名字", "disabled": false }
```

### 9.3 用户

#### GET `/users?role=user&department_id=1&page=1&size=20`
```json
{
  "items": [
    { "id": 5, "username": "zhangsan", "real_name": "张三",
      "role": "user", "department_id": 1, "department_name": "技术部",
      "disabled": false, "created_at": "..." }
  ],
  "total": 12, "page": 1, "size": 20
}
```

#### POST `/users`
```json
{
  "username": "lisi", "password": "init1234",
  "role": "user", "department_id": 1, "real_name": "李四"
}
```

#### PATCH `/users/:id`
```json
{ "real_name": "李四四", "disabled": false, "department_id": 2 }
```

#### POST `/users/:id/reset-password`
```json
{ "new_password": "..." }
```

### 9.4 公文

#### GET `/documents?role_view=publish|inbox&page=1`
- `role_view=publish`: 我发布的（super/dept）
- `role_view=inbox`: 我收到的（user）
- super 不传则全部
```json
{
  "items": [
    {
      "id": 10, "title": "关于 X 的通知",
      "publisher": { "id": 1, "real_name": "管理员" },
      "target_scope": "DEPARTMENT", "deadline": "...",
      "status": "ACTIVE", "created_at": "...",
      "stats": { "total": 12, "submitted": 8, "late": 1, "pending": 3, "returned": 0 }
    }
  ],
  "total": 25
}
```

#### POST `/documents`
```json
{
  "title": "关于 X 的通知",
  "content_html": "<p>请各位...</p>",
  "target_scope": "DEPARTMENT",
  "target_department_ids": [1, 2],       // 仅 scope=DEPARTMENT 时必填
  "deadline": "2026-06-01T18:00:00+08:00", // 可选
  "reading_attachment_ids": [17, 18],    // 之前通过 /attachments 上传的 ID
  "template_attachment_ids": [19]
}
// 响应 201
{ "id": 10, ... }
```

#### GET `/documents/:id`
```json
{
  "id": 10, "title": "...", "content_html": "...",
  "publisher": {...}, "target_scope": "...",
  "target_departments": [...], "deadline": "...",
  "status": "ACTIVE",
  "reading_attachments": [...], "template_attachments": [...],
  "my_submission": {                     // 当前 user 的上报情况(user 视角)
    "id": 42, "current_status": "RETURNED",
    "return_reason": "格式不对",
    "attachments": [...]
  }
}
```

#### PATCH `/documents/:id`
```json
{
  "title": "...",
  "content_html": "...",
  "deadline": "...",
  "add_reading_attachment_ids": [20],
  "remove_attachment_ids": [17]
}
// 响应包含修订记录
```

#### POST `/documents/:id/recall`
撤回，所有未提交的 submission 标记为 `RECALLED`（或直接软删 documents）。

#### GET `/documents/:id/revisions`
```json
{ "items": [
  { "id": 1, "editor": {...}, "change_type": "CONTENT",
    "diff_summary": "修改了正文", "created_at": "..." }
] }
```

### 9.5 附件

#### POST `/attachments` (multipart/form-data)
```
form fields:
  owner_type:  DOCUMENT_DRAFT | SUBMISSION | INLINE
  purpose:     READING | TEMPLATE  (仅 owner_type=DOCUMENT_DRAFT 时)
  file:        <file>
```
注意：上传公文附件时还没有 document_id，所以用 `DOCUMENT_DRAFT` 临时挂载，发布公文时将其转挂到正式 owner_id。

```json
// 响应 201
{
  "id": 17, "file_name": "通知.pdf", "size_bytes": 123456,
  "mime_type": "application/pdf",
  "preview_url": "/api/v1/attachments/17/preview",
  "download_url": "/api/v1/attachments/17/download"
}
```

#### GET `/attachments/:id/download`
强制 `Content-Disposition: attachment`，流式下载。

#### GET `/attachments/:id/preview`
仅 PDF 可用，`Content-Disposition: inline`。

#### DELETE `/attachments/:id`
权限：上传者本人，且对应业务对象处于可编辑状态。

#### POST `/inline-images` (multipart)
专给富文本编辑器用，返回 `{ url: "..." }` 让 wangEditor 直接 src 引用。

### 9.6 上报

#### POST `/submissions/:document_id`
```json
// 请求(submit 或 resubmit 都用这个接口,服务端自动判定)
{
  "note": "已按要求填写",
  "attachment_ids": [33, 34]   // 上报附件 ID
}
// 响应 200
{ "id": 42, "current_status": "SUBMITTED", "submitted_at": "..." }
```

#### POST `/submissions/:id/return`
```json
{ "reason": "附件格式错误,请用 PDF" }
// 响应 200
{ "id": 42, "current_status": "RETURNED", "return_count": 1 }
```

#### GET `/submissions/mine?status=PENDING|SUBMITTED|RETURNED&page=1`
普通用户查看自己的上报列表。

#### GET `/submissions/:id/actions`
查看某条上报的动作流水（部门/顶级 + 上报本人可看）。

### 9.7 纵览统计

#### GET `/stats/global`（super 专用）
```json
{
  "total_documents": 25,
  "active_documents": 20,
  "by_department": [
    { "department_id": 1, "name": "技术部",
      "total": 240, "submitted": 200, "late": 10, "pending": 25, "overdue": 5, "returned": 0 }
  ]
}
```

#### GET `/stats/documents/:id`
单公文上报详情。
```json
{
  "document": {...},
  "summary": { "total": 12, "submitted": 8, "late": 1, "overdue": 0, "pending": 3, "returned": 0 },
  "by_user": [
    { "user_id": 5, "real_name": "张三", "department_id": 1, "department_name": "技术部",
      "current_status": "SUBMITTED", "submitted_at": "...",
      "submission_id": 42, "attachments": [...] }
  ],
  "by_department": [   // 仅当 scope=DEPARTMENT 时返回
    { "department_id": 1, "submitted": 8, "pending": 4, ... }
  ]
}
```

#### GET `/stats/departments/:id`
某部门视角：本部门相关的所有公文。
```json
{
  "department": {...},
  "documents": [
    { "document_id": 10, "title": "...", "publisher": "...",
      "total": 12, "submitted": 8, "pending": 4, ... }
  ]
}
```

---

## 10. 前端页面设计

### 10.1 路由树

```
/login                            登录页

/super                            (role=super)
├── /dashboard                    全局看板
├── /departments                  部门管理
├── /users                        用户管理
├── /documents                    公文列表(我发布的)
├── /documents/new                发布公文
├── /documents/:id                公文详情 + 纵览
└── /audit                        审计日志

/dept                             (role=dept)
├── /dashboard                    本部门看板
├── /users                        本部门用户管理
├── /documents                    我发布的 + 我部门收到的
├── /documents/new                发布公文(本部门)
└── /documents/:id                公文详情 + 部门纵览

/user                             (role=user)
├── /inbox                        待办
├── /outbox                       已办/已退回
└── /documents/:id                公文详情 + 我的上报
```

### 10.2 关键页面线框

#### 顶级 - 发布公文页

```
┌────────────────────────────────────────────────────────────────┐
│  发布公文                                              [取消][发布]│
├────────────────────────────────────────────────────────────────┤
│  标题: [关于 XX 的通知                                      ]   │
│                                                                │
│  发送范围: ⦿ 指定部门  ◯ 全员                                 │
│  目标部门: [✓ 技术部] [ ] 财务部 [ ] 行政部                    │
│                                                                │
│  截止日期: [2026-06-01  ▼] [18:00 ▼]   ☐ 不设截止             │
│                                                                │
│  正文:                                                         │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ B I U 标题 列表 表格 图片 ...   [富文本工具条]            │ │
│  ├──────────────────────────────────────────────────────────┤ │
│  │                                                          │ │
│  │  请各位收到通知后, 在 6 月 1 日前提交...                  │ │
│  │                                                          │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                │
│  阅读附件: [+ 上传]                                            │
│  ├ 通知正文.pdf       1.2MB    [预览][删除]                   │
│  └ 政策依据.pdf       540KB    [预览][删除]                   │
│                                                                │
│  上报模板: [+ 上传]                                            │
│  └ 申报表.xlsx        45KB     [下载][删除]                   │
└────────────────────────────────────────────────────────────────┘
```

#### 普通用户 - 公文详情页

```
┌────────────────────────────────────────────────────────────────┐
│  ← 返回    关于 XX 的通知              发布人: 管理员           │
│            截止: 2026-06-01 18:00   状态: [已退回]              │
├────────────────────────────────────────────────────────────────┤
│  ⚠ 您的上报已被退回                                            │
│    退回原因: 附件格式错误,请上传 PDF                            │
├────────────────────────────────────────────────────────────────┤
│  公文正文:                                                     │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 请各位收到通知后, 在 6 月 1 日前提交...                   │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                │
│  阅读附件:                                                     │
│  ├ 通知正文.pdf [预览] [下载]                                  │
│  └ 政策依据.pdf [预览] [下载]                                  │
│                                                                │
│  上报模板 (请下载填写):                                        │
│  └ 申报表.xlsx [下载]                                          │
├────────────────────────────────────────────────────────────────┤
│  我的上报:                                                     │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ 附件: [+ 上传]                                           │ │
│  │  └ 我的申报表.pdf [预览][删除]                            │ │
│  │                                                          │ │
│  │ 备注: [已按要求填写完成                            ]      │ │
│  │                                                          │ │
│  │                                            [重新提交]    │ │
│  └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

#### 顶级 - 公文纵览页

```
┌────────────────────────────────────────────────────────────────┐
│  关于 XX 的通知  纵览                                          │
│  发布: 2026-05-27   截止: 2026-06-01   范围: 技术部、财务部     │
├────────────────────────────────────────────────────────────────┤
│  整体: ███████░░░ 67%   8/12 已交  (准时 7  逾期 1  退回 0)    │
├────────────────────────────────────────────────────────────────┤
│  按部门:                                                       │
│  ┌────────┬───────┬─────┬─────┬──────┬──────┐                  │
│  │ 部门    │ 应交  │ 已交│ 逾期│ 待交 │ 退回 │                  │
│  ├────────┼───────┼─────┼─────┼──────┼──────┤                  │
│  │ 技术部  │ 8     │ 6   │ 1   │ 1    │ 0    │  [展开]         │
│  │ 财务部  │ 4     │ 2   │ 0   │ 2    │ 0    │  [展开]         │
│  └────────┴───────┴─────┴─────┴──────┴──────┘                  │
│                                                                │
│  ▼ 技术部明细:                                                 │
│  ┌────────┬─────────────┬──────────┬──────────────┐            │
│  │ 用户    │ 状态         │ 提交时间  │ 操作         │            │
│  ├────────┼─────────────┼──────────┼──────────────┤            │
│  │ 张三    │ 准时已交     │ 05-28    │ [查看][退回] │            │
│  │ 李四    │ 待上报       │ -        │ -            │            │
│  └────────┴─────────────┴──────────┴──────────────┘            │
└────────────────────────────────────────────────────────────────┘
```

### 10.3 状态标签颜色规范

| 状态 | 颜色 | 例 |
|---|---|---|
| `PENDING` 待上报 | 灰色 | ░ |
| `SUBMITTED` 准时已交 | 绿色 | ✓ |
| `SUBMITTED_LATE` 逾期已交 | 橙色 | ⚠ |
| `OVERDUE` 逾期未交 | 红色 | ✗ |
| `RETURNED` 已退回 | 紫色 | ↺ |

---

## 11. 技术栈与依赖

### 11.1 后端

| 类别 | 选择 | 理由 |
|---|---|---|
| 语言 | Go 1.22+ | 单二进制部署、并发性能好 |
| Web 框架 | gin-gonic/gin | 国内最常见、文档全、中间件生态成熟 |
| ORM | gorm.io/gorm + driver/postgres | 国内主流 ORM，支持事务/钩子/软删 |
| JWT | golang-jwt/jwt/v5 | 官方维护 |
| 校验 | go-playground/validator/v10 | gin 内置集成 |
| 日志 | rs/zerolog | 结构化、高性能 |
| 配置 | spf13/viper | YAML + 环境变量混合 |
| 文件类型探测 | gabriel-vasile/mimetype | 读 magic number |
| HTML 清洗 | microcosm-cc/bluemonday | 防 XSS |
| 数据库迁移 | pressly/goose | SQL 文件式，便于审查 |
| UUID | google/uuid | 内嵌图片用 |

### 11.2 前端

| 类别 | 选择 | 理由 |
|---|---|---|
| 框架 | Vue 3 + TypeScript | 国内生态、组合式 API |
| 构建 | Vite 5 | 启动快、HMR |
| UI 库 | Element Plus | 表格/表单/上传组件齐全 |
| 状态 | Pinia | Vue 3 官方推荐 |
| 路由 | vue-router 4 | + 角色守卫 |
| HTTP | axios | 拦截器统一处理 401/403 |
| 富文本 | @wangeditor/editor + @wangeditor/editor-for-vue | 中文场景成熟，表格/缩进/对齐齐全 |
| PDF 预览 | vue-pdf-embed | 基于 pdf.js，纯前端无后端依赖 |
| 时间 | dayjs | 轻量 |
| 图标 | @element-plus/icons-vue | |

### 11.3 基础设施

| 类别 | 选择 |
|---|---|
| 反向代理 | Nginx |
| 进程管理 | systemd |
| 数据库 | PostgreSQL 15+ |
| 备份 | pg_dump + rsync |
| HTTPS | Let's Encrypt（公网域名）|

---

## 12. 项目目录结构

```
d:/docflow/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go              # HTTP 服务入口
│   │   └── admin/main.go               # 初始化 super 账号的 CLI
│   ├── internal/
│   │   ├── api/                        # HTTP handlers
│   │   │   ├── auth.go
│   │   │   ├── users.go
│   │   │   ├── departments.go
│   │   │   ├── documents.go
│   │   │   ├── attachments.go
│   │   │   ├── submissions.go
│   │   │   ├── stats.go
│   │   │   └── router.go               # 路由装配
│   │   ├── service/
│   │   │   ├── document_service.go     # 含发布物化逻辑
│   │   │   ├── submission_service.go   # 状态机集中点
│   │   │   ├── attachment_service.go
│   │   │   └── stats_service.go
│   │   ├── model/                      # GORM 模型,一表一文件
│   │   │   ├── user.go
│   │   │   ├── department.go
│   │   │   ├── document.go
│   │   │   ├── submission.go
│   │   │   └── attachment.go
│   │   ├── middleware/
│   │   │   ├── jwt.go
│   │   │   ├── rbac.go
│   │   │   ├── upload_limit.go
│   │   │   ├── recover.go
│   │   │   └── access_log.go
│   │   ├── storage/
│   │   │   ├── interface.go            # type Storage interface { ... }
│   │   │   └── local.go                # 本地磁盘实现
│   │   ├── auth/
│   │   │   ├── jwt.go
│   │   │   └── password.go
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── db/
│   │   │   └── db.go
│   │   └── errors/
│   │       └── errors.go
│   ├── migrations/
│   │   ├── 001_init.up.sql
│   │   └── 001_init.down.sql
│   ├── config.example.yaml
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── public/
│   ├── src/
│   │   ├── api/
│   │   │   ├── client.ts               # axios 实例
│   │   │   ├── auth.ts
│   │   │   ├── users.ts
│   │   │   ├── documents.ts
│   │   │   ├── submissions.ts
│   │   │   └── stats.ts
│   │   ├── stores/
│   │   │   ├── auth.ts
│   │   │   └── ...
│   │   ├── router/
│   │   │   ├── index.ts
│   │   │   └── guards.ts               # 角色守卫
│   │   ├── views/
│   │   │   ├── login/LoginView.vue
│   │   │   ├── super/
│   │   │   │   ├── Dashboard.vue
│   │   │   │   ├── DepartmentManage.vue
│   │   │   │   ├── UserManage.vue
│   │   │   │   ├── DocumentPublish.vue
│   │   │   │   ├── DocumentList.vue
│   │   │   │   └── DocumentOverview.vue
│   │   │   ├── dept/...
│   │   │   ├── user/
│   │   │   │   ├── Inbox.vue
│   │   │   │   ├── Outbox.vue
│   │   │   │   └── DocumentDetail.vue
│   │   │   └── error/...
│   │   ├── components/
│   │   │   ├── PdfPreview.vue
│   │   │   ├── RichEditor.vue
│   │   │   ├── AttachmentList.vue
│   │   │   ├── AttachmentUploader.vue
│   │   │   ├── StatusTag.vue
│   │   │   ├── DepartmentSelector.vue
│   │   │   └── ProgressBar.vue
│   │   ├── types/                      # TypeScript 类型
│   │   ├── utils/
│   │   ├── App.vue
│   │   └── main.ts
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── deploy/
│   ├── nginx.conf.example
│   ├── docflow.service                 # systemd unit
│   ├── backup.sh                       # 备份脚本
│   └── README.md
├── docs/
│   ├── api.md
│   ├── deploy.md
│   └── design.md                       # 本文档
├── .gitignore
├── LICENSE
└── README.md
```

---

## 13. 配置与环境

### 13.1 `config.example.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  base_url: "https://docflow.example.com"  # 用于生成附件 URL

database:
  host: "127.0.0.1"
  port: 5432
  user: "docflow"
  password: "${DOCFLOW_DB_PASSWORD}"        # 支持环境变量覆盖
  dbname: "docflow"
  sslmode: "disable"
  max_open_conns: 25
  max_idle_conns: 5

storage:
  root: "/var/docflow/files"               # 附件根目录
  tmp_dir: "/var/docflow/tmp"              # 上传临时区
  max_file_size: 20971520                  # 20MB
  max_inline_image_size: 5242880           # 5MB

auth:
  jwt_secret: "${DOCFLOW_JWT_SECRET}"      # 32 字节随机串
  access_token_ttl: 3600                   # 1 小时
  refresh_token_ttl: 604800                # 7 天
  bcrypt_cost: 12

initial_super:                              # 首次启动若无 super 则自动创建
  enabled: true
  username: "admin"
  password: "${DOCFLOW_INITIAL_PASSWORD}"
  real_name: "超级管理员"

log:
  level: "info"                            # debug/info/warn/error
  format: "json"                           # json/console
  output: "/var/log/docflow/server.log"

cors:
  allow_origins: ["https://docflow.example.com"]
```

### 13.2 环境变量（敏感项）

```bash
DOCFLOW_DB_PASSWORD=...        # 数据库密码
DOCFLOW_JWT_SECRET=...         # JWT 签名密钥
DOCFLOW_INITIAL_PASSWORD=...   # 首次启动初始 super 的密码
```

`.gitignore` 中明确排除 `config.yaml` `.env`，仅提交 `.example`。

---

## 14. 安全设计

### 14.1 密码

- bcrypt cost=12（约 250ms/次，平衡安全与性能）
- 不允许小于 8 位
- 不强制复杂度（小型组织过强反而记不住），但建议提示
- 密码重置由 super/dept 发起，重置后下次登录强制修改（可选 M3 实现）

### 14.2 JWT

- HS256（对称，单机部署够用）
- access_token TTL 1 小时
- refresh_token TTL 7 天
- payload: `{ user_id, role, department_id, exp, iat, jti }`
- 登出走黑名单（Redis 或进程内 LRU）

### 14.3 RBAC 中间件

两层校验：
1. **角色级**：路由组指定 `RequireRole("super")`
2. **资源级**：在 handler 内部用 service 层校验所有权（如 dept 退回时校验 `submission.department_id == operator.department_id`）

### 14.4 上传安全

```
┌─ 前端 → Nginx (client_max_body_size 21m) → Gin MaxBytesReader(20m)
├─ 校验扩展名白名单
├─ 校验 MIME magic number
├─ 文件名清洗 (去 .. / \ 控制字符 / Windows 保留名)
├─ 路径拼接后强制位于 storage_root 之下
└─ 写入磁盘 (UUID 防冲突)
```

### 14.5 XSS

- 富文本入库前用 `bluemonday.UGCPolicy()` 清洗
- 允许的标签白名单：`p, br, strong, em, u, h1-h6, ul/ol/li, table/tr/td/th, img(src白名单), a(href), code, pre, blockquote`
- 移除 `<script>` `<iframe>` `on*` 事件属性 `javascript:` URL
- 出库渲染前再做一次清洗（防御性）

### 14.6 CSRF

- 因为前后端分离 + JWT 走 Authorization 头，天然不受 CSRF 影响
- 不使用 cookie 认证

### 14.7 其他

- HTTPS 仅在 Nginx 终止
- 数据库密码/JWT secret 通过环境变量注入，不写入仓库
- 错误响应不泄漏堆栈（仅在 log 中打）
- 速率限制（M3 可加）：登录接口 5 次/分钟/IP

---

## 15. 实施分期 Milestone

### M1 —— MVP 闭环（约 1.5~2 周）

**目标**：完成"顶级发文 → 用户上报 → 顶级看汇总"主链路。

**后端任务**：
- [ ] 项目骨架 + go.mod + viper 配置
- [ ] PostgreSQL 迁移 001（含全部表）
- [ ] JWT 中间件 + RBAC 中间件
- [ ] `/auth/login` `/auth/refresh` `/auth/logout`
- [ ] `/departments` CRUD（super only）
- [ ] `/users` CRUD（super only，含 reset-password）
- [ ] `/attachments` 上传 + 下载 + 预览 + 删除
- [ ] `/documents` 发布（仅支持 `DEPARTMENT` scope）+ 详情 + 列表
- [ ] `/submissions/:doc_id` 上报（PENDING → SUBMITTED）
- [ ] `/stats/documents/:id` 单公文纵览
- [ ] `cmd/admin` 初始化 super 账号 CLI

**前端任务**：
- [ ] 项目骨架 + 路由 + axios 拦截器
- [ ] 登录页
- [ ] 顶级：部门管理页、用户管理页
- [ ] 顶级：发布公文页（含富文本 + 附件上传）
- [ ] 顶级：公文列表 + 公文详情纵览
- [ ] 普通用户：待办列表 + 公文详情 + PDF 预览 + 上报上传
- [ ] 401/403 拦截

**验收**：完成 [16.1 功能验证脚本](#161-功能验证最小可用)。

---

### M2 —— 完整业务（约 1~1.5 周）

**目标**：补齐部门角色、退回机制、全员广播、公文编辑等。

- [ ] 部门用户角色（dept）+ 本部门 user 管理
- [ ] 公文范围扩展：`OWN_DEPARTMENT` + `ALL_USERS`
- [ ] 退回机制：`/submissions/:id/return` + 状态机完整迁移
- [ ] 重提覆盖附件
- [ ] 截止日期 + `OVERDUE`/`SUBMITTED_LATE` 状态
- [ ] 公文编辑（PATCH）+ `document_revisions` 日志
- [ ] 模板附件 `purpose=TEMPLATE` 区分
- [ ] 部门纵览页（dept 视角 + super 查看某部门）
- [ ] 全局多公文纵览（super 看板）
- [ ] 公文撤回

---

### M3 —— 体验与运维（约 1 周）

- [ ] 站内通知中心
  - 新公文分配
  - 即将截止（提前 24h）
  - 被退回
- [ ] 操作日志页（super 看 `submission_actions` + `document_revisions`）
- [ ] 上报情况导出 Excel
- [ ] 部署文档完善（nginx.conf、systemd unit、备份脚本）
- [ ] 备份脚本（pg_dump + 附件目录）
- [ ] 健康检查 `/healthz`
- [ ] 登录速率限制
- [ ] README 完善

---

## 16. 验证方案

### 16.1 功能验证（最小可用）

按以下脚本走一遍 M1 端到端：

1. 用 admin CLI 初始化 super 账号 `admin/init1234`
2. 启动服务，浏览器打开 `https://localhost`
3. 用 admin 登录 → 跳转到 super 主页
4. 创建部门"技术部"
5. 在技术部下创建 user 账号 `zhangsan/init1234`，真名"张三"
6. 切到"发布公文"，填：
   - 标题：测试公文
   - 范围：指定部门 → 技术部
   - 截止：3 天后
   - 正文（富文本）：粘一段带粗体和列表的文字
   - 阅读附件：传一个 PDF
   - 模板附件：传一个 xlsx
7. 发布成功，回到列表能看到这条公文
8. 退出登录，用 `zhangsan` 登录 → 跳转到 user 待办
9. 看到"测试公文"红点，点进详情
10. 点 PDF 预览，能在页面内看到 PDF 内容
11. 下载 xlsx 模板，本地填几个字
12. 上传"我的申报.pdf"，写备注，点提交
13. 退出，用 `admin` 登录
14. 进入公文纵览，能看到 "技术部 1/1 已交"
15. 点开张三的上报，能预览他的 PDF
16. 点"退回"，输入原因"请补充附图"
17. 退出，用 `zhangsan` 登录
18. 看到红色"已退回"标签和退回原因
19. 重新上传新 PDF，提交
20. 切回 admin，纵览状态变为"已重新提交"

### 16.2 安全验证

| 用例 | 期望 |
|---|---|
| 不带 token 调 `/api/v1/users` | 401 |
| user token 调 `POST /api/v1/users` | 403 |
| dept A 用 dept B 的 submission_id 调 return | 403 |
| 上传 `.exe` 文件 | 拒绝 |
| 上传 25MB 文件 | 拦截（Nginx 或 Gin） |
| 富文本写 `<script>alert(1)</script>` | 入库后 script 被清洗 |
| URL 改 `attachment_id` 越权访问他人附件 | 403 |
| 重复登录失败 6 次 | 锁定（M3） |

### 16.3 部署验证

- `systemctl restart docflow` 后服务自动起来
- 重启服务器后服务自动随 systemd 启动
- 附件目录权限设置允许 docflow 用户读写
- Nginx 反代 + HTTPS 工作正常
- `pg_dump` 备份脚本能在 cron 中执行并产出文件
- 数据库连接池在长时间运行后不耗尽

---

## 17. 部署运维

### 17.1 部署拓扑

```
              Internet / 内网
                    │
                    ▼
            ┌───────────────┐
            │     Nginx     │  HTTPS, 静态前端, 反代 /api → 8080
            └───────┬───────┘
                    │
                    ▼
            ┌───────────────┐
            │ docflow (Go)  │  systemd 守护
            │  :8080        │
            └───┬───────┬───┘
                │       │
                ▼       ▼
        ┌──────────┐ ┌──────────────────┐
        │PostgreSQL│ │ /var/docflow/files│
        │  :5432   │ │  (附件根目录)     │
        └──────────┘ └──────────────────┘
```

### 17.2 Nginx 关键配置

```nginx
server {
    listen 443 ssl http2;
    server_name docflow.example.com;

    ssl_certificate     /etc/ssl/.../fullchain.pem;
    ssl_certificate_key /etc/ssl/.../privkey.pem;

    client_max_body_size 21m;        # 略大于 20MB

    root /var/www/docflow;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 60s;
    }

    location / {
        try_files $uri $uri/ /index.html;   # Vue history 模式
    }
}
```

### 17.3 systemd 服务

```ini
[Unit]
Description=DocFlow Server
After=network.target postgresql.service

[Service]
Type=simple
User=docflow
EnvironmentFile=/etc/docflow/env
ExecStart=/opt/docflow/docflow-server -config /etc/docflow/config.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### 17.4 备份脚本

```bash
#!/bin/bash
# /opt/docflow/backup.sh
set -e
DATE=$(date +%Y%m%d)
BACKUP_DIR=/backup/docflow

# 数据库
pg_dump -U docflow docflow | gzip > $BACKUP_DIR/db_$DATE.sql.gz

# 附件
tar -czf $BACKUP_DIR/files_$DATE.tar.gz -C /var/docflow files

# 保留 30 天
find $BACKUP_DIR -name "*.gz" -mtime +30 -delete
```

`crontab -e`：
```
0 2 * * * /opt/docflow/backup.sh >> /var/log/docflow/backup.log 2>&1
```

---

## 18. 不在本次范围

明确列出**当前版本不做**的功能，避免范围蔓延：

- 工作流引擎（多级审批、会签、转办）→ 用户明确说"审批流程后期再定"
- 附件版本历史（重提覆盖即可）
- 多租户/组织隔离
- 移动端 App / 小程序
- 公文全文检索
- 国际化 / 多语言
- 单点登录 SSO
- 短信/邮件外发通知
- 公文模板库 / 套红头公文打印
- 数字签名 / 电子印章

---

## 19. 待最终确认事项

设计稿基本完整，落地前请你最后拍板这两个细节：

### 19.1 初始 super 账号创建方式

| 方案 | 说明 | 推荐 |
|---|---|---|
| A. CLI 工具一次性创建 | `./docflow-admin create-super --username admin` | ⭐ 推荐，安全 |
| B. 配置文件 + 首次启动自动建 | `config.yaml` 写明，服务启动时若无 super 则建 | 方便但密码可能在日志中泄漏 |

### 19.2 富文本内嵌图片配额

| 方案 | 说明 |
|---|---|
| A. 单独限额：内嵌图片单图 ≤5MB，全文 ≤20 张 | ⭐ 推荐，与附件解耦 |
| B. 计入"10 个附件 / 20MB 单文件" | 用户上传多图时容易超 |

### 19.3 其他可选确认

- 是否在 M1 阶段就**预留**站内通知的数据表（即使不实现 UI）？我倾向是，能避免后期改表。
- 公文撤回是**物理删除**还是**标记 RECALLED**？我倾向标记，便于审计。

请确认以上三项后我就 ExitPlanMode，开始落地编码。

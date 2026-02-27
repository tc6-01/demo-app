# MWS365 Demo App

MWS365 平台第三方应用接入参考实现，演示 OAuth2 SSO 登录、OpenAPI 调用、Webhook 事件接收的完整流程。

## 功能概览

- **OAuth2 SSO 登录** — 授权码模式登录、Token 刷新、登出
- **OpenAPI 调用** — 获取 tenant_access_token，查询用户/部门/群组/角色
- **Webhook 事件接收** — 签名验证、事件去重、按类型分发处理
- **回调接收** — 处理 MWS 平台回调推送（如通知按钮点击）
- **用户全量同步** — local 模式下支持通过 OpenAPI 拉取全部用户到本地

## 项目结构

```
├── main.go                  # 入口，路由注册
├── config.yaml              # 配置文件
├── schema.sql               # Demo App 建表脚本
├── client/
│   ├── oauth_client.go      # OAuth2 客户端（授权码交换、刷新、UserInfo）
│   └── openapi_client.go    # OpenAPI 客户端（Token 管理、通讯录接口）
├── handler/
│   ├── oauth.go             # OAuth2 登录/回调/登出/刷新
│   ├── openapi.go           # OpenAPI 操作 HTTP 接口
│   ├── webhook.go           # Webhook 事件/回调处理
│   └── page.go              # 页面渲染（首页、Dashboard）
├── model/
│   └── types.go             # 配置、请求/响应、事件模型定义
├── store/
│   ├── db.go                # 数据库初始化
│   ├── user.go              # 本地用户 CRUD（local 模式）
│   └── event_log.go         # 事件日志存储、去重
├── signature/
│   └── verify.go            # Webhook 签名验证
├── sync/
│   └── contact.go           # 用户全量同步
├── templates/
│   ├── index.html           # 首页
│   └── dashboard.html       # Dashboard
├── cmd/
│   └── setup/main.go        # 【MWS 平台侧】初始化工具
└── scripts/
    └── setup.sql            # 【MWS 平台侧】初始化 SQL 模板
```

---

## Part 1：接入方操作指南

> 以下操作由**第三方应用开发者**完成。

### 前置条件

- Go 1.21+
- MySQL 5.7+ / 8.0+
- 从 MWS 平台管理员处获取以下信息：
  - `app_id` — 应用 ID
  - `app_secret` — 应用密钥
  - `tenant_uuid` — 租户 UUID
  - `encrypt_key` — 事件签名加密密钥
  - `mws_base_url` — MWS 平台服务地址

### 第一步：创建 Demo App 数据库

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mws365_demo DEFAULT CHARSET utf8mb4"
mysql -u root -p mws365_demo < schema.sql
```

建表脚本会创建：
- `event_logs` — 事件日志表（两种模式都需要）
- `users` — 本地用户表（仅 `local` 模式需要）

### 第二步：配置文件

创建 `config.yaml`，填入从 MWS 平台管理员处获取的凭证：

```yaml
server:
  port: 8080
  base_url: "http://<你的IP>:8080"

app:
  user_mode: "mws"  # "mws" = 仅通过 OAuth2 认证, "local" = 同步用户到本地

mysql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/mws365_demo?charset=utf8mb4&parseTime=true&loc=Local"

mws:
  base_url: "https://your-mws-server.com"  # MWS 平台地址（由管理员提供）

openapi:
  app_id: "your_app_id"            # 由管理员提供
  app_secret: "your_app_secret"    # 由管理员提供
  tenant_uuid: "your_tenant_uuid"  # 由管理员提供
  encrypt_key: "your_encrypt_key"  # 由管理员提供

oauth2:
  client_id: "your_app_id"         # 与 openapi.app_id 相同
  client_secret: "your_app_secret" # 与 openapi.app_secret 相同
  redirect_uri: "http://<你的IP>:8080/oauth2/callback"
  scopes: "openid profile email"
```

> **重要**：`base_url` 和 `redirect_uri` 中的地址必须是 MWS 服务端能够访问到的地址。如果 MWS 运行在远程服务器/k8s 集群中，不能使用 `localhost`，需要使用内网 IP 或公网地址。

### 第三步：启动应用

```bash
go run main.go

# 指定配置文件路径
go run main.go -config path/to/config.yaml

# 启动时全量同步用户（仅 local 模式）
go run main.go -sync
```

启动后访问 `http://<你的IP>:8080`，点击登录按钮跳转到 MWS 授权页面完成 OAuth2 认证。

### 第四步：告知管理员配置 Webhook URL

启动应用后，将以下地址提供给 MWS 平台管理员，由管理员在平台侧配置：

- **事件推送地址**：`http://<你的IP>:8080/webhook/events`
- **回调推送地址**：`http://<你的IP>:8080/webhook/callbacks`
- **OAuth2 回调地址**：`http://<你的IP>:8080/oauth2/callback`

---

## Part 2：MWS 平台侧操作指南

> 以下操作由 **MWS 平台管理员**完成，接入方无需关心。
> 这些操作需要 MWS 平台数据库的写入权限。

### 方式 A：使用初始化工具

```bash
# 使用已有租户
go run cmd/setup/main.go \
  --mws-dsn "root:123456@tcp(127.0.0.1:3306)/mws365" \
  --tenant-id 1 \
  --app-base-url "http://<接入方IP>:8080"

# 自动创建新租户
go run cmd/setup/main.go \
  --mws-dsn "root:123456@tcp(127.0.0.1:3306)/mws365" \
  --tenant-name "My Tenant" \
  --app-base-url "http://<接入方IP>:8080"

# 仅预览 SQL，不实际执行
go run cmd/setup/main.go --dry-run
```

工具会自动完成：
1. 在 `applications` 表中创建应用记录
2. 在 `oauth_clients` 表中创建 OAuth2 客户端
3. 在 `tenant_applications` 表中关联应用到租户
4. 订阅事件和回调
5. 为租户用户创建 `app_users` 映射
6. 输出 `config.yaml`（交给接入方使用）

**参数列表：**

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--mws-dsn` | MWS 平台数据库 DSN（必填） | — |
| `--mws-base-url` | MWS 服务地址 | `https://uccp-dev.shimorelease.com` |
| `--tenant-id` | 使用已有租户 ID（不传则自动创建） | — |
| `--tenant-name` | 新建租户名称 | `Demo Tenant` |
| `--tenant-domain` | 新建租户域名（可选） | — |
| `--app-name` | 应用名称 | `MWS365 Demo App` |
| `--app-base-url` | 接入方应用外部访问地址 | `http://localhost:8080` |
| `--demo-dsn` | Demo App 数据库 DSN（写入 config.yaml） | `root:123456@tcp(...)` |
| `--port` | 监听端口 | `8080` |
| `--output` | 输出 config.yaml 路径（空则 stdout） | — |
| `--dry-run` | 仅打印 SQL，不执行 | `false` |

### 方式 B：手动执行 SQL

参考 `scripts/setup.sql`，替换模板变量后在 MWS 平台数据库中执行。主要包括：

1. 在 `applications` 表中插入应用记录（含 `event_url`、`callback_url`、`encrypt_key`）
2. 在 `oauth_clients` 表中插入 OAuth2 客户端（含 `redirect_uri`）
3. 在 `tenant_applications` 表中关联应用到租户

### 更新 Webhook / OAuth2 回调地址

接入方提供了实际可达的地址后，更新数据库中的 URL：

```sql
-- 更新 Webhook 地址
UPDATE applications
SET event_url = 'http://<接入方IP>:8080/webhook/events',
    callback_url = 'http://<接入方IP>:8080/webhook/callbacks'
WHERE app_id = '<app_id>';

-- 更新 OAuth2 回调地址
UPDATE oauth_clients
SET redirect_uri = 'http://<接入方IP>:8080/oauth2/callback'
WHERE client_id = '<app_id>';
```

### 交付给接入方的信息

初始化完成后，将以下信息提供给接入方：

| 信息 | 说明 | 来源 |
|------|------|------|
| `app_id` | 应用 ID | 初始化工具生成 |
| `app_secret` | 应用密钥 | 初始化工具生成 |
| `tenant_uuid` | 租户 UUID | 从 `tenants` 表查询 |
| `encrypt_key` | 事件签名密钥 | 初始化工具生成 |
| `mws_base_url` | MWS 平台地址 | 部署地址 |

---

## 接入流程详解

### OAuth2 SSO 登录

```
浏览器                     Demo App                      MWS 平台
  │                          │                              │
  │── GET /oauth2/login ────>│                              │
  │                          │── 302 重定向 ───────────────>│
  │                          │   /oauth2/authorize          │
  │                          │   ?client_id=...             │
  │                          │   &redirect_uri=...          │
  │                          │   &response_type=code        │
  │                          │   &scope=openid profile email│
  │                          │   &state=随机值              │
  │                          │                              │
  │<──────── 授权页面 ────────────────────────────────────── │
  │── 用户授权 ──────────────────────────────────────────── >│
  │                          │                              │
  │<── 302 回调 ─────────────│<── code + state ─────────────│
  │    /oauth2/callback      │                              │
  │    ?code=xxx&state=xxx   │                              │
  │                          │── POST /oauth2/token ───────>│
  │                          │   (授权码换 Token)            │
  │                          │<── access_token, id_token ───│
  │                          │                              │
  │                          │── GET /oauth2/userinfo ─────>│
  │                          │   (获取用户信息)              │
  │                          │<── sub, name, email... ──────│
  │                          │                              │
  │<── 302 /dashboard ───────│   创建 Session               │
```

**接口说明：**

| 路径 | 方法 | 说明 |
|------|------|------|
| `/oauth2/login` | GET | 跳转到 MWS 授权页面 |
| `/oauth2/callback` | GET | 授权回调，用 code 换取 Token 和 UserInfo |
| `/oauth2/logout` | GET | 清除 Session，跳转首页 |
| `/oauth2/refresh` | GET | 刷新 Access Token |

**注意事项：**
- MWS 的 Token 和 UserInfo 接口返回包装格式 `{"code":"OK","data":{...}}`，需要从 `data` 字段中提取实际数据，代码已做兼容处理
- `state` 参数用于防 CSRF，存储在内存中，应用重启后失效
- Session 存储在内存中，重启后用户需要重新登录

### OpenAPI 调用

通过 `tenant_access_token` 调用 MWS 通讯录接口：

```
Demo App                                MWS 平台
  │                                       │
  │── POST /openapi/v1/auth/              │
  │   tenant_access_token ───────────────>│
  │   {app_id, app_secret, tenant_uuid}   │
  │<── tenant_access_token ───────────────│
  │                                       │
  │── GET /openapi/v1/contact/users ─────>│
  │   Authorization: Bearer <token>       │
  │<── 用户列表 ──────────────────────────│
```

**Dashboard 提供的操作：**

| 路径 | 说明 |
|------|------|
| `/api/tenant-token` | 获取 tenant_access_token |
| `/api/users` | 查询用户（支持 `uids` 参数按 union_uid 过滤） |
| `/api/departments` | 查询子部门（`department_uuid` 参数，默认根部门 `0`） |
| `/api/groups` | 查询群组列表 |
| `/api/group-users` | 查询群组成员（`group_uuid` 参数） |
| `/api/role-members` | 查询角色成员（`role_uuid` 参数） |
| `/api/events` | 查询事件日志 |
| `/api/sync` | 触发全量用户同步（仅 local 模式） |

**Token 管理机制：**
- 自动缓存 `tenant_access_token`，提前 5 分钟刷新
- 收到 401 响应时自动清除缓存并重新获取 Token 后重试

### Webhook 事件接收

MWS 平台通过 HTTP POST 推送事件到 Demo App：

```
MWS 平台                          Demo App
  │                                  │
  │── POST /webhook/events ─────────>│
  │   X-MWS-Request-Timestamp: ...   │  1. 签名验证
  │   X-MWS-Request-Nonce: ...       │  2. 事件落库 + 去重
  │   X-MWS-Signature: ...           │  3. 按类型分发处理
  │   Body: {metadata, event}        │
  │<── 200 OK ───────────────────────│
  │                                  │
  │── POST /webhook/callbacks ──────>│
  │   Body: {metadata, callback}     │  回调处理
  │<── 200 OK ───────────────────────│
```

**签名验证算法：**

```
signature = sha256(timestamp + nonce + encrypt_key + body)
```

其中 `timestamp`、`nonce`、`signature` 从请求头获取，`encrypt_key` 在配置文件中设置。

### 支持的事件类型

| 类型 | 事件 | 说明 |
|------|------|------|
| 用户 | `contact.user.create` | 新建用户 |
| 用户 | `contact.user.update` | 更新用户 |
| 群组 | `contact.group.create` | 新建群组 |
| 群组 | `contact.group.update` | 更新群组 |
| 群组 | `contact.group.delete` | 删除群组 |
| 群组成员 | `contact.group.add_users` | 群组添加成员 |
| 群组成员 | `contact.group.remove_users` | 群组移除成员 |
| 部门 | `contact.department.create` | 新建部门 |
| 部门 | `contact.department.update` | 更新部门 |
| 部门 | `contact.department.delete` | 删除部门 |
| 部门成员 | `contact.department.add_users` | 部门添加成员 |
| 部门成员 | `contact.department.remove_users` | 部门移除成员 |
| 角色 | `roles.add_users` | 角色添加成员 |
| 角色 | `roles.remove_users` | 角色移除成员 |
| 应用 | `app.update` | 应用配置更新 |
| 回调 | `notify.button.click` | 通知按钮点击 |

---

## 用户模式

### mws 模式（默认）

- 通过 OAuth2 SSO 登录获取用户身份
- 通过 OpenAPI 实时查询通讯录数据
- 不在本地存储用户数据
- 适合轻量级集成场景

### local 模式

- 通过 OpenAPI 全量同步用户到本地 `users` 表
- 通过 Webhook 事件增量更新本地用户数据
- 适合需要离线查询或复杂业务逻辑的场景

启用方式：在 `config.yaml` 中设置 `app.user_mode: "local"`

```bash
# 启动时执行全量同步
go run main.go -sync

# 或在 Dashboard 中点击"全量同步用户"按钮
```

---

## 常见问题

### OAuth2 登录后报 401

MWS 的 OAuth2 Token 和 UserInfo 接口返回包装格式 `{"code":"OK","data":{...}}`，需要从 `data` 字段中提取实际数据。本项目已处理该兼容性问题。

### Webhook 收不到事件

1. 确认 MWS 平台侧的 `event_url` 已配置为接入方应用可达的地址
2. 如果 MWS 运行在远程环境（如 k8s），`event_url` 不能是 `localhost`，需要使用接入方的内网 IP 或公网地址
3. 确认防火墙允许 MWS 服务端访问接入方应用的端口
4. 联系 MWS 平台管理员确认配置是否正确

### 按 union_uid 查不到用户

OAuth2 返回的 `sub` 和 OpenAPI 返回的 `union_uid` 格式可能不同。`sub` 通常是 UUID 格式（如 `660e8400...`），而 `union_uid` 是 `uu_` 前缀格式（如 `uu_019c8f...`）。查询用户时应使用 OpenAPI 返回的 `union_uid`。

### 浏览器重启后 callback 一直失败

应用使用内存存储 Session 和 OAuth2 state。重启应用后所有 state 失效，浏览器缓存的旧 callback URL 会报"state 验证失败"。解决方法：打开新的浏览器窗口（或无痕模式）重新从 `/oauth2/login` 开始登录。

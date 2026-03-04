# MWS365 Demo App

MWS365 平台第三方应用接入参考实现，演示 OAuth2 SSO 登录、OpenAPI 调用、Webhook 事件接收的完整流程。

---

## Part 1：MWS 平台侧（管理员操作）

> 以下操作由 MWS 平台管理员在 **MWS 平台数据库（mws365）** 中完成。
> 完成后将生成的凭证交给接入方。

### 第一步：确定租户

查询已有租户，记下要接入的 `TENANT_ID` 和 `TENANT_UUID`：

```sql
SELECT id, LOWER(INSERT(INSERT(INSERT(INSERT(HEX(uuid),9,0,'-'),14,0,'-'),19,0,'-'),24,0,'-')) AS tenant_uuid, name
FROM tenants WHERE deleted_at = 0;
```

### 第二步：生成凭证

在 MySQL 中生成以下随机值，后续 SQL 会用到：

```sql
-- 生成 APP_ID（demo_ 前缀 + 16 位随机十六进制）
SELECT CONCAT('demo_', LEFT(MD5(UUID()), 16)) AS APP_ID;

-- 生成 APP_SECRET（64 位随机十六进制）
SELECT SHA2(CONCAT(UUID(), RAND(), NOW(6)), 256) AS APP_SECRET;

-- 生成 ENCRYPT_KEY（32 位随机十六进制）
SELECT LEFT(SHA2(CONCAT(UUID(), RAND()), 256), 32) AS ENCRYPT_KEY;
```

记录下这三个值，后续 SQL 和交付给接入方都需要。

### 第三步：在 MWS 数据库中插入数据

将下面 SQL 中的 `{{变量}}` 替换为实际值后执行：

| 变量 | 含义 | 示例 |
|------|------|------|
| `{{APP_ID}}` | 第二步生成的应用 ID | `demo_742863b0b1ec721f` |
| `{{APP_SECRET}}` | 第二步生成的应用密钥 | `530db785857c5f73...` |
| `{{ENCRYPT_KEY}}` | 第二步生成的加密密钥 | `8a0d78a8ea107c23...` |
| `{{TENANT_ID}}` | 第一步查到的租户 ID | `1` |
| `{{APP_BASE_URL}}` | 接入方应用的外部可达地址 | `http://172.16.23.100:8080` |

```sql
SET @NOW_MS = UNIX_TIMESTAMP() * 1000;

-- 1. 创建应用
INSERT INTO applications (
    uuid, app_id, union_id, name, app_secret,
    `desc`, type, scope, status,
    event_url, callback_url, home_url, encrypt_key,
    created_at, updated_at
) VALUES (
    UNHEX(REPLACE(UUID(), '-', '')),
    '{{APP_ID}}',
    0,
    'MWS365 Demo App',
    '{{APP_SECRET}}',
    'MWS365 Demo App - 第三方应用接入参考实现',
    2, 'openid profile email', 1,
    '{{APP_BASE_URL}}/webhook/events',
    '{{APP_BASE_URL}}/webhook/callbacks',
    '{{APP_BASE_URL}}',
    '{{ENCRYPT_KEY}}',
    @NOW_MS, @NOW_MS
);

-- 2. 创建 OAuth2 客户端
INSERT INTO oauth_clients (
    uuid, client_id, tenant_id, name, secret,
    redirect_uri, is_public, audience, logout_url,
    status, created_at, updated_at
) VALUES (
    UNHEX(REPLACE(UUID(), '-', '')),
    '{{APP_ID}}',
    {{TENANT_ID}},
    'MWS365 Demo App',
    '{{APP_SECRET}}',
    '{{APP_BASE_URL}}/oauth2/callback',
    0, '{{APP_BASE_URL}}', '{{APP_BASE_URL}}/oauth2/logout',
    1, @NOW_MS, @NOW_MS
);

-- 3. 安装应用到租户
INSERT INTO tenant_applications (
    uuid, app_id, tenant_id, status, visible_to_all,
    created_at, updated_at
) VALUES (
    UNHEX(REPLACE(UUID(), '-', '')),
    '{{APP_ID}}',
    {{TENANT_ID}},
    1, 1,
    @NOW_MS, @NOW_MS
);
```

**验证插入结果：**

```sql
SELECT app_id, name, status, event_url FROM applications WHERE app_id = '{{APP_ID}}';
SELECT client_id, tenant_id, redirect_uri FROM oauth_clients WHERE client_id = '{{APP_ID}}';
SELECT app_id, tenant_id, status FROM tenant_applications WHERE app_id = '{{APP_ID}}';
```

### 第四步：交付给接入方

将以下 5 个值提供给接入方：

| 信息 | 值 | 说明 |
|------|---|------|
| `app_id` | 第二步生成 | 应用 ID |
| `app_secret` | 第二步生成 | 应用密钥 |
| `tenant_uuid` | 第一步查到 | 租户 UUID（带横线格式） |
| `encrypt_key` | 第二步生成 | Webhook 签名密钥 |
| `mws_base_url` | 部署地址 | MWS 平台服务地址，如 `https://uccp-dev.shimorelease.com` |

### 后续：更新接入方地址

如果接入方的 IP/域名变更，需要更新三处 URL：

```sql
-- 更新 Webhook 地址
UPDATE applications
SET event_url    = 'http://<新地址>:8080/webhook/events',
    callback_url = 'http://<新地址>:8080/webhook/callbacks'
WHERE app_id = '{{APP_ID}}';

-- 更新 OAuth2 回调地址
UPDATE oauth_clients
SET redirect_uri = 'http://<新地址>:8080/oauth2/callback'
WHERE client_id = '{{APP_ID}}';
```

---

## Part 2：接入方操作（开发者）

> 以下操作由第三方应用开发者完成。

### 你需要从管理员获取的信息

| 信息 | 说明 | 填入配置项 |
|------|------|-----------|
| `app_id` | 应用 ID | `openapi.app_id` + `oauth2.client_id` |
| `app_secret` | 应用密钥 | `openapi.app_secret` + `oauth2.client_secret` |
| `tenant_uuid` | 租户 UUID | `openapi.tenant_uuid` |
| `encrypt_key` | Webhook 签名密钥 | `openapi.encrypt_key` |
| `mws_base_url` | MWS 平台地址 | `mws.base_url` |

### 你需要提供给管理员的信息

应用启动后，将以下地址发给管理员配置：

| 地址 | 用途 |
|------|------|
| `http://<你的IP>:8080/webhook/events` | 事件推送地址 |
| `http://<你的IP>:8080/webhook/callbacks` | 回调推送地址 |
| `http://<你的IP>:8080/oauth2/callback` | OAuth2 回调地址 |

> 这些地址必须是 MWS 平台可达的，不能用 `localhost`。

### 第一步：启动 MySQL & 建库建表

```bash
# macOS
brew services start mysql

# 建库
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mws365_demo DEFAULT CHARSET utf8mb4"

# 建表
mysql -u root -p mws365_demo < schema.sql
```

`schema.sql` 创建以下表：

| 表名 | 用途 | 模式要求 |
|------|------|---------|
| `event_logs` | Webhook 事件日志（去重用 `event_uuid` 唯一索引） | 所有模式 |
| `users` | 本地用户缓存（通过 OpenAPI 同步） | 仅 `local` 模式 |

### 第二步：填写配置文件

创建 `config.yaml`：

```yaml
server:
  port: 8080
  base_url: "http://<你的IP>:8080"           # ← 你自己的 IP

app:
  user_mode: "mws"                            # "mws" 或 "local"

mysql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/mws365_demo?charset=utf8mb4&parseTime=true&loc=Local"
                                              # ← 你自己的 MySQL

mws:
  base_url: "https://uccp-dev.shimorelease.com"  # ← 管理员提供

openapi:
  app_id: "demo_742863b0b1ec721f"             # ← 管理员提供
  app_secret: "530db785..."                   # ← 管理员提供
  tenant_uuid: "990e8400-e29b-..."            # ← 管理员提供
  encrypt_key: "8a0d78a8..."                  # ← 管理员提供

oauth2:
  client_id: "demo_742863b0b1ec721f"          # ← 与 app_id 相同
  client_secret: "530db785..."                # ← 与 app_secret 相同
  redirect_uri: "http://<你的IP>:8080/oauth2/callback"  # ← 你自己的 IP
  scopes: "openid profile email"
```

### 第三步：启动

```bash
go run main.go

# 指定配置文件
go run main.go -config path/to/config.yaml

# 启动时全量同步用户（仅 local 模式）
go run main.go -sync
```

### 第四步：验证

1. 访问 `http://<你的IP>:8080`
2. 点击「使用 MWS 账号登录」
3. 在 MWS 授权页面完成登录
4. 跳转到 Dashboard，可以测试 OpenAPI 调用和查看 Webhook 事件

---

## 用户模式

| 模式 | 用户数据来源 | 适用场景 |
|------|------------|---------|
| `mws`（默认） | OAuth2 SSO 登录 + OpenAPI 实时查询 | 轻量集成，不存本地用户 |
| `local` | OpenAPI 全量同步 + Webhook 增量更新 | 需要离线查询或复杂业务 |

切换方式：`config.yaml` 中设置 `app.user_mode: "local"`，启动时加 `-sync` 全量同步。

---

## 接口一览

### OAuth2

| 路径 | 方法 | 说明 |
|------|------|------|
| `/oauth2/login` | GET | 跳转 MWS 授权页面 |
| `/oauth2/callback` | GET | 授权回调，code 换 Token + UserInfo |
| `/oauth2/logout` | GET | 清除 Session，跳转首页 |
| `/oauth2/refresh` | GET | 刷新 Access Token |

### Webhook

| 路径 | 方法 | 说明 |
|------|------|------|
| `/webhook/events` | POST | 接收事件推送（签名验证 + 去重 + 分发） |
| `/webhook/callbacks` | POST | 接收回调推送 |

### API

#### 认证

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/tenant-token` | GET | 获取 tenant_access_token |

#### 通讯录

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/users` | GET | 查询用户（`uids` 参数按 union_uid 过滤） |
| `/api/departments` | GET | 查询子部门（`department_uuid`，默认 `0`） |
| `/api/groups` | GET | 查询群组列表 |
| `/api/group-users` | GET | 查询群组成员（`group_uuid`） |
| `/api/role-members` | GET | 查询角色成员（`role_uuid`） |

#### 全量接口（大分页优化）

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/all-users` | GET | 获取所有用户（500条/页，用于全量同步） |
| `/api/all-departments` | GET | 获取所有部门（500条/页） |

#### 应用与租户

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/tenant-info` | GET | 获取租户完整信息（应用、AI配置、License） |
| `/api/visibility-users` | GET | 获取应用可见性用户列表 |

#### 同步与事件

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/sync` | POST | 全量用户同步（仅 local 模式） |
| `/api/events` | GET | 查询事件日志 |

### 支持的事件类型

| 类型 | 事件 |
|------|------|
| 用户 | `contact.user.create` · `contact.user.update` |
| 群组 | `contact.group.create` · `update` · `delete` · `add_users` · `remove_users` |
| 部门 | `contact.department.create` · `update` · `delete` · `add_users` · `remove_users` |
| 角色 | `roles.add_users` · `roles.remove_users` |
| 应用 | `app.update` · `app.install` · `app.uninstall` |
| 应用可见性 | `app.visibility.add_users` · `app.visibility.remove_users` |
| 租户配置 | `tenant.config.update` |
| 回调 | `notify.button.click` |

---

## 接入流程图

### OAuth2 SSO 登录

```
浏览器                     Demo App                      MWS 平台
  │── GET /oauth2/login ──>│                              │
  │                        │── 302 → /oauth2/authorize ──>│
  │<── 授权页面 ────────────────────────────────────────── │
  │── 用户授权 ───────────────────────────────────────── >│
  │                        │<── code + state ─────────────│
  │                        │── POST /oauth2/token ───────>│
  │                        │<── access_token, id_token ───│
  │                        │── GET /oauth2/userinfo ─────>│
  │                        │<── sub, name, email... ──────│
  │<── 302 /dashboard ─────│                              │
```

### Webhook 签名验证

```
signature = sha256(timestamp + nonce + encrypt_key + body)
```

Header：`X-MWS-Request-Timestamp`、`X-MWS-Request-Nonce`、`X-MWS-Signature`

---

## 新增功能说明

### 全量同步优化

使用大分页接口（`/api/all-users`、`/api/all-departments`）替代普通接口，同步效率提升 5-25 倍：

| 对比项 | 普通接口 | 大分页接口 |
|--------|---------|----------|
| 分页大小 | 20-100 条 | 500 条 |
| 5000 用户同步 | 50-250 次请求 | 10 次请求 |
| 适用场景 | 实时查询、按需过滤 | 全量同步、批量导出 |

### 租户上下文接口

**GET /api/tenant-info** 一次性获取：
- 当前应用的安装信息（状态、安装时间）
- 租户的 AI 配置（模型、API Key）
- License 信息（类型、有效期、席位配额）

### 应用可见性

**GET /api/visibility-users** 获取哪些用户可以看到和使用当前应用。

可见性规则：
- `visible_to_all = true`：所有用户可见
- `visible_to_all = false`：按规则（用户/群组/部门）过滤

相关事件：
- `app.visibility.add_users`：添加可见性用户
- `app.visibility.remove_users`：移除可见性用户

### 应用生命周期事件

| 事件 | 说明 | 业务场景 |
|------|------|---------|
| `app.install` | 应用被安装到租户 | 初始化租户数据、发送欢迎消息 |
| `app.uninstall` | 应用从租户卸载 | 清理租户数据、停止后台任务 |
| `app.update` | 应用信息更新 | 更新本地缓存的应用名称、描述 |

### 租户配置更新事件

**tenant.config.update** 通知第三方应用租户配置变更（如 AI 配置更新），应用可重新获取最新配置。

---

## 项目结构

```
├── main.go                  # 入口，路由注册
├── config.yaml              # 配置文件
├── schema.sql               # Demo App 建表脚本
├── client/
│   ├── oauth_client.go      # OAuth2 客户端
│   └── openapi_client.go    # OpenAPI 客户端（每次实时获取 Token、401 重试）
├── handler/
│   ├── oauth.go             # OAuth2 登录/回调/登出/刷新
│   ├── openapi.go           # OpenAPI HTTP 接口
│   ├── webhook.go           # Webhook 事件/回调处理
│   └── page.go              # 页面渲染
├── model/
│   └── types.go             # 配置、请求/响应、事件模型
├── store/
│   ├── db.go                # 数据库初始化
│   ├── user.go              # 本地用户 CRUD（local 模式）
│   └── event_log.go         # 事件日志存储、去重
├── signature/
│   └── verify.go            # Webhook 签名验证
├── sync/
│   └── contact.go           # 用户全量同步
└── templates/
    ├── index.html            # 首页
    └── dashboard.html        # Dashboard
```

---

## 常见问题

**OAuth2 登录后报 401**
MWS 的 Token/UserInfo 接口返回包装格式 `{"code":"OK","data":{...}}`，本项目已兼容处理。

**Webhook 收不到事件**
确认 `event_url` 是 MWS 平台可达的地址（不能是 `localhost`），检查防火墙。

**按 union_uid 查不到用户**
OAuth2 `sub` 是 UUID 格式（`660e8400...`），OpenAPI `union_uid` 是 `uu_` 前缀格式（`uu_019c8f...`），两者不同。

**浏览器 callback 一直失败**
应用使用内存存储 Session 和 state，重启后失效。用无痕模式重新从 `/oauth2/login` 开始。

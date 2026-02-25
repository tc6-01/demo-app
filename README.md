# MWS365 Demo App

MWS365 平台第三方应用接入参考实现，演示 OAuth2 SSO 登录、OpenAPI 调用、Webhook 事件接收的完整流程。

## 功能

- **OAuth2 SSO 登录** — 授权码模式登录、Token 刷新、登出
- **OpenAPI 调用** — 获取 tenant_access_token，查询用户/部门/群组/角色
- **Webhook 事件接收** — 签名验证、事件去重、按类型分发处理
- **回调接收** — 处理 MWS 平台回调推送（如通知按钮点击）
- **用户全量同步** — local 模式下支持通过 OpenAPI 拉取全部用户到本地

## 项目结构

```
├── main.go                  # 入口，路由注册
├── config.yaml              # 配置文件
├── schema.sql               # 建表脚本
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
└── cmd/
    └── setup/main.go        # 初始化工具（注册应用、插入 MWS 数据库记录）
```

## 快速开始

### 1. 准备配置

复制并修改配置文件：

```yaml
server:
  port: 8080
  base_url: "http://localhost:8080"

app:
  user_mode: "mws"  # "mws" = 仅通过 OAuth2 认证, "local" = 同步用户到本地

mysql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/mws365_demo?charset=utf8mb4&parseTime=true&loc=Local"

mws:
  base_url: "https://uccp-dev.shimorelease.com"

openapi:
  app_id: "your_app_id"
  app_secret: "your_app_secret"
  tenant_uuid: "your_tenant_uuid"
  encrypt_key: "your_encrypt_key"

oauth2:
  client_id: "your_app_id"
  client_secret: "your_app_secret"
  redirect_uri: "http://localhost:8080/oauth2/callback"
  scopes: "openid profile email"
```

### 2. 初始化数据库

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mws365_demo DEFAULT CHARSET utf8mb4"
mysql -u root -p mws365_demo < schema.sql
```

### 3. 注册应用（可选）

使用初始化工具自动在 MWS 数据库中注册应用：

```bash
go run cmd/setup/main.go --mws-dsn "root:123456@tcp(127.0.0.1:3306)/mws365" --tenant-id 1

# 仅预览 SQL，不实际执行
go run cmd/setup/main.go --dry-run
```

### 4. 启动

```bash
go run main.go
```

访问 `http://localhost:8080`，点击登录将跳转到 MWS 授权页面完成 OAuth2 认证。

## 接入流程

### OAuth2 SSO

1. 用户访问 `/oauth2/login`，自动跳转到 MWS 授权页面
2. 用户在 MWS 完成登录授权
3. MWS 回调 `/oauth2/callback`，demo app 用授权码换取 Token 并获取 UserInfo
4. 创建 Session，跳转到 Dashboard

### OpenAPI

1. 通过 `app_id` + `app_secret` + `tenant_uuid` 获取 `tenant_access_token`
2. 携带 Token 调用通讯录接口：用户查询、部门查询、群组查询、角色查询
3. Token 自动缓存，过期自动刷新，401 自动重试

### Webhook

1. 在 MWS 应用配置中设置 `event_url` 和 `callback_url`
2. demo app 接收推送后进行签名验证（`encrypt_key`）
3. 事件落库并去重（基于 `event_uuid`）
4. 按事件类型分发处理

## 支持的事件类型

| 类型 | 事件 |
|------|------|
| 用户 | `contact.user.create`, `contact.user.update` |
| 群组 | `contact.group.create`, `contact.group.update`, `contact.group.delete` |
| 群组成员 | `contact.group.add_users`, `contact.group.remove_users` |
| 部门 | `contact.department.create`, `contact.department.update`, `contact.department.delete` |
| 部门成员 | `contact.department.add_users`, `contact.department.remove_users` |
| 角色 | `roles.add_users`, `roles.remove_users` |
| 应用 | `app.update` |
| 回调 | `notify.button.click` |

## 用户模式

- **mws 模式**（默认）：通过 OAuth2 SSO 认证，不本地存储用户数据
- **local 模式**：通过 OpenAPI 全量同步用户到本地 `users` 表，支持增量事件更新

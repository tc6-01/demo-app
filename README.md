# MWS365 Demo App

MWS365 OpenAPI + OAuth2 SSO 对接参考实现。

模拟第三方应用对接 MWS365 的完整流程，包括：
- **OAuth2 SSO 登录**：授权码流程、Token 刷新、登出
- **OpenAPI 调用**：获取 tenant_access_token、查询通讯录用户
- **事件 Webhook**：接收事件推送、签名验证、幂等去重
- **用户同步**（可选）：全量拉取 + 事件增量更新

## 快速开始

### 1. 准备数据库

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mws365_demo DEFAULT CHARSET utf8mb4;"
mysql -u root -p mws365_demo < schema.sql
```

### 2. 修改配置

编辑 `config.yaml`，填入实际的凭证信息：

```yaml
mws:
  base_url: "https://mws365.ru"       # MWS 服务地址

openapi:
  app_id: "your-app-id"               # 联系管理员获取
  app_secret: "your-app-secret"
  tenant_uuid: "your-tenant-uuid"
  encrypt_key: ""                      # 可选，用于验证事件签名

oauth2:
  client_id: "your-client-id"          # 联系管理员获取
  client_secret: "your-client-secret"
  redirect_uri: "http://localhost:8080/oauth2/callback"
```

### 3. 启动

```bash
go run main.go

# 启动时自动全量同步用户（仅 local 模式）
go run main.go --sync

# 指定配置文件
go run main.go --config /path/to/config.yaml
```

### 4. 访问

打开浏览器访问 http://localhost:8080

## 两种用户模式

通过 `config.yaml` 中 `app.user_mode` 配置：

| 模式 | 值 | 说明 |
|------|------|------|
| 复用 MWS 用户 | `mws` | 不建本地用户表，用户信息实时调 OpenAPI 获取 |
| 独立用户体系 | `local` | 本地维护 users 表，全量同步 + 事件增量更新 |

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 首页 |
| GET | `/oauth2/login` | 发起 OAuth2 登录 |
| GET | `/oauth2/callback` | OAuth2 回调 |
| GET | `/oauth2/logout` | 登出 |
| GET | `/oauth2/refresh` | 刷新 Token |
| GET | `/dashboard` | 仪表盘 |
| POST | `/webhook/events` | 事件接收端点 |
| POST | `/webhook/callbacks` | 回调接收端点 |
| GET | `/api/tenant-token` | 获取 tenant_access_token |
| GET | `/api/users?uids=xx` | 查询用户（mws 模式需 uids 参数） |
| POST | `/api/sync` | 全量同步用户（仅 local 模式） |
| GET | `/api/events` | 查看事件日志 |

## Webhook 配置

在 MWS365 应用管理中配置：

- **event_url**: `http://your-domain/webhook/events`
- **callback_url**: `http://your-domain/webhook/callbacks`

### 事件签名验证

如果配置了 `encrypt_key`，MWS365 推送事件时会携带签名头：

- `X-MWS-Request-Timestamp`: 时间戳
- `X-MWS-Request-Nonce`: 随机字符串
- `X-MWS-Signature`: 签名值

签名算法：`SHA256(timestamp + nonce + encryptKey + body)`

## 项目结构

```
├── main.go              # 入口
├── config.yaml          # 配置
├── schema.sql           # 建表脚本
├── client/
│   ├── openapi_client.go   # OpenAPI 客户端
│   └── oauth_client.go     # OAuth2 客户端
├── handler/
│   ├── oauth.go         # OAuth2 登录处理
│   ├── openapi.go       # OpenAPI 演示
│   ├── webhook.go       # Webhook 接收
│   └── page.go          # 页面渲染
├── store/               # 数据库操作
├── sync/                # 用户同步
├── model/               # 类型定义
├── signature/           # 签名验证
└── templates/           # HTML 模板
```

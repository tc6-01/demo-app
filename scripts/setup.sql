-- ============================================================
-- MWS365 Demo App 接入初始化 SQL 模板
-- ============================================================
-- 使用方法：
--   1. 替换下方所有 {{变量}} 为实际值
--   2. 在 MWS 平台数据库（mws365）中执行
-- ============================================================

-- ==================== 变量说明 ====================
-- {{APP_UUID}}        : 应用 UUID（32 位十六进制，如 SELECT HEX(UUID_TO_BIN(UUID()))）
-- {{OAUTH_UUID}}      : OAuth 客户端 UUID
-- {{TENANT_APP_UUID}} : 租户应用 UUID
-- {{APP_ID}}          : 应用唯一标识（如 demo_app_xxx）
-- {{APP_SECRET}}      : 应用密钥（建议 64 位十六进制随机字符串）
-- {{APP_NAME}}        : 应用显示名称（如 MWS365 Demo App）
-- {{ENCRYPT_KEY}}     : 事件签名加密密钥（32 位十六进制随机字符串）
-- {{TENANT_ID}}       : 目标租户 ID（整数，从 tenants 表查询）
-- {{APP_BASE_URL}}    : Demo App 外部访问地址（如 http://localhost:8080）
-- {{NOW_MS}}          : 当前时间戳（毫秒），可用 UNIX_TIMESTAMP() * 1000

-- ==================== 生成随机值（参考） ====================
-- 生成 UUID:     SELECT HEX(UUID_TO_BIN(UUID()));
-- 生成密钥:      SELECT SHA2(CONCAT(UUID(), RAND(), NOW(6)), 256);
-- 当前时间戳(ms): SELECT UNIX_TIMESTAMP() * 1000;

-- ==================== 查询现有租户 ====================
-- SELECT id, HEX(uuid) AS uuid_hex, name, domain FROM tenants WHERE deleted_at = 0;

START TRANSACTION;

-- 1. 创建应用
INSERT INTO applications (
    uuid, app_id, union_id, name, app_secret,
    `desc`, type, scope, status,
    event_url, callback_url, home_url, encrypt_key,
    created_at, updated_at
) VALUES (
    UNHEX('{{APP_UUID}}'),
    '{{APP_ID}}',
    0,
    '{{APP_NAME}}',
    '{{APP_SECRET}}',
    'MWS365 Demo App - 第三方应用对接参考实现',
    2,  -- type=2: 租户应用
    'openid profile email',
    1,  -- status=1: 启用
    '{{APP_BASE_URL}}/webhook/events',
    '{{APP_BASE_URL}}/webhook/callbacks',
    '{{APP_BASE_URL}}',
    '{{ENCRYPT_KEY}}',
    {{NOW_MS}}, {{NOW_MS}}
);

-- 2. 创建 OAuth2 客户端
INSERT INTO oauth_clients (
    uuid, client_id, tenant_id, name, secret,
    redirect_uri, is_public, audience, logout_url,
    status, created_at, updated_at
) VALUES (
    UNHEX('{{OAUTH_UUID}}'),
    '{{APP_ID}}',         -- client_id = app_id
    {{TENANT_ID}},
    '{{APP_NAME}}',
    '{{APP_SECRET}}',     -- secret = app_secret
    '{{APP_BASE_URL}}/oauth2/callback',
    0,  -- is_public=0: 机密客户端
    '{{APP_BASE_URL}}',
    '{{APP_BASE_URL}}/oauth2/logout',
    1,  -- status=1: 启用
    {{NOW_MS}}, {{NOW_MS}}
);

-- 3. 安装应用到租户
INSERT INTO tenant_applications (
    uuid, app_id, tenant_id, status, visible_to_all,
    installed_at, created_at, updated_at
) VALUES (
    UNHEX('{{TENANT_APP_UUID}}'),
    '{{APP_ID}}',
    {{TENANT_ID}},
    1,  -- status=1: 启用
    1,  -- visible_to_all=1: 对租户所有用户可见
    {{NOW_MS}}, {{NOW_MS}}, {{NOW_MS}}
);

COMMIT;

-- ==================== 验证 ====================
-- SELECT app_id, name, status FROM applications WHERE app_id = '{{APP_ID}}';
-- SELECT client_id, tenant_id, redirect_uri FROM oauth_clients WHERE client_id = '{{APP_ID}}';
-- SELECT app_id, tenant_id, status FROM tenant_applications WHERE app_id = '{{APP_ID}}';

-- ==================== 生成 config.yaml 所需值 ====================
-- app_id:      {{APP_ID}}
-- app_secret:  {{APP_SECRET}}
-- tenant_uuid: SELECT LOWER(INSERT(INSERT(INSERT(INSERT(HEX(uuid),9,0,'-'),14,0,'-'),19,0,'-'),24,0,'-')) FROM tenants WHERE id = {{TENANT_ID}};
-- client_id:   {{APP_ID}}         (与 app_id 相同)
-- client_secret: {{APP_SECRET}}   (与 app_secret 相同)
-- encrypt_key: {{ENCRYPT_KEY}}

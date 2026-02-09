-- MWS365 Demo App 建表脚本
-- 使用前请先创建数据库: CREATE DATABASE IF NOT EXISTS mws365_demo DEFAULT CHARSET utf8mb4;

-- 公共表：事件日志（两种模式都需要）
CREATE TABLE IF NOT EXISTS event_logs (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_uuid  VARCHAR(64)  NOT NULL COMMENT '事件唯一 ID（用于去重）',
    event_type  VARCHAR(64)  NOT NULL COMMENT '事件类型，如 contact.user.create',
    tenant_uuid VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '租户 UUID',
    app_id      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '应用 ID',
    payload     JSON         NOT NULL COMMENT '完整事件载荷',
    processed   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否已处理',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_event_uuid (event_uuid),
    KEY idx_event_type (event_type),
    KEY idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='接收到的事件日志';

-- 模式 B 额外表：本地用户（仅 user_mode=local 时需要）
CREATE TABLE IF NOT EXISTS users (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    union_uid   VARCHAR(64)  NOT NULL COMMENT '用户唯一标识，格式 uu_xxx',
    name        VARCHAR(128) NOT NULL DEFAULT '' COMMENT '姓名',
    nickname    VARCHAR(128) NOT NULL DEFAULT '' COMMENT '昵称',
    email       VARCHAR(256) NOT NULL DEFAULT '' COMMENT '邮箱',
    mobile      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '手机号',
    avatar      VARCHAR(512) NOT NULL DEFAULT '' COMMENT '头像 URL',
    status      TINYINT      NOT NULL DEFAULT 1 COMMENT '状态',
    synced_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后同步时间',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_union_uid (union_uid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='本地用户（从 MWS365 同步）';

package model

import "time"

// ==================== 配置 ====================

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	App     AppConfig     `yaml:"app"`
	MySQL   MySQLConfig   `yaml:"mysql"`
	MWS     MWSConfig     `yaml:"mws"`
	OpenAPI OpenAPIConfig `yaml:"openapi"`
	OAuth2  OAuth2Config  `yaml:"oauth2"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type AppConfig struct {
	UserMode string `yaml:"user_mode"` // "mws" or "local"
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type MWSConfig struct {
	BaseURL string `yaml:"base_url"`
}

type OpenAPIConfig struct {
	AppID      string `yaml:"app_id"`
	AppSecret  string `yaml:"app_secret"`
	TenantUUID string `yaml:"tenant_uuid"`
	EncryptKey string `yaml:"encrypt_key"`
}

type OAuth2Config struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
	Scopes       string `yaml:"scopes"`
}

// ==================== OpenAPI 响应 ====================

// TenantTokenReq 获取 tenant_access_token 请求
type TenantTokenReq struct {
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	TenantUUID string `json:"tenant_uuid"`
}

// TenantTokenResp 获取 tenant_access_token 响应
type TenantTokenResp struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	ExpiresIn         int64  `json:"expires_in"` // 毫秒
}

// OpenAPIUser OpenAPI 返回的用户结构
type OpenAPIUser struct {
	UnionUID  string `json:"union_uid"`
	Name      string `json:"name"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
	Gender    int    `json:"gender"`
	Avatar    string `json:"avatar"`
	Status    int8   `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	Timezone  string `json:"timezone"`
}

// BatchGetUsersResp 批量获取用户响应
type BatchGetUsersResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool          `json:"has_more"`
		PageToken string        `json:"page_token"`
		Items     []OpenAPIUser `json:"items"`
	} `json:"data"`
}

// ==================== OAuth2 ====================

// OAuth2TokenResp OAuth2 token 响应
type OAuth2TokenResp struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // 秒
}

// OAuth2UserInfo OAuth2 userinfo 响应
type OAuth2UserInfo struct {
	Sub      string `json:"sub"`
	Name     string `json:"name"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Avatar   string `json:"avatar"`
}

// ==================== Session ====================

// Session 内存中的用户会话
type Session struct {
	ID           string
	UserInfo     *OAuth2UserInfo
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// ==================== 事件 ====================

// EventPayload MWS365 推送的事件载荷（schema 2.0）
type EventPayload struct {
	Schema   string        `json:"schema"`
	Metadata EventMetadata `json:"metadata"`
	Event    any           `json:"event"`
}

// EventMetadata 事件元数据
type EventMetadata struct {
	EventUUID  string `json:"event_uuid"`
	Token      string `json:"token"`
	CreatedAt  int64  `json:"created_at"`
	EventType  string `json:"event_type"`
	TenantUUID string `json:"tenant_uuid"`
	AppID      string `json:"app_id"`
}

// EventUser 用户事件载荷（contact.user.create/update）
type EventUser struct {
	UnionUID   string `json:"union_uid"`
	Name       string `json:"name"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Nickname   string `json:"nickname,omitempty"`
	Email      string `json:"email,omitempty"`
	WorkEmail  string `json:"work_email,omitempty"`
	Mobile     string `json:"mobile,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	Status     int8   `json:"status"`
	IsExternal bool   `json:"is_external"`
}

// ==================== 回调 ====================

// CallbackPayload MWS365 推送的回调载荷
type CallbackPayload struct {
	Schema   string           `json:"schema"`
	Metadata CallbackMetadata `json:"metadata"`
	Callback any              `json:"callback"`
}

// CallbackMetadata 回调元数据
type CallbackMetadata struct {
	CallbackUUID string `json:"callback_uuid"`
	CallbackType string `json:"callback_type"`
	CreateTime   int64  `json:"create_time"`
	TenantUUID   string `json:"tenant_uuid"`
	AppID        string `json:"app_id"`
}

// ==================== 数据库模型 ====================

// DBUser users 表模型
type DBUser struct {
	ID        int64     `json:"id"`
	UnionUID  string    `json:"union_uid"`
	Name      string    `json:"name"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	Mobile    string    `json:"mobile"`
	Avatar    string    `json:"avatar"`
	Status    int8      `json:"status"`
	SyncedAt  time.Time `json:"synced_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DBEventLog event_logs 表模型
type DBEventLog struct {
	ID         int64     `json:"id"`
	EventUUID  string    `json:"event_uuid"`
	EventType  string    `json:"event_type"`
	TenantUUID string    `json:"tenant_uuid"`
	AppID      string    `json:"app_id"`
	Payload    string    `json:"payload"`
	Processed  bool      `json:"processed"`
	CreatedAt  time.Time `json:"created_at"`
}

// ==================== API 响应 ====================

// APIResponse 通用 API 响应
type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

package model

import (
	"encoding/json"
	"time"
)

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
	Code any    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TenantAccessToken string `json:"tenant_access_token"`
		ExpiresIn         int64  `json:"expires_in"` // 毫秒
	} `json:"data"`
}

// IsOK 检查响应是否成功
func (r *TenantTokenResp) IsOK() bool {
	switch v := r.Code.(type) {
	case float64:
		return v == 0
	case string:
		return v == "OK" || v == "0"
	}
	return false
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
	Code any    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		HasMore   bool          `json:"has_more"`
		PageToken string        `json:"page_token"`
		Items     []OpenAPIUser `json:"items"`
	} `json:"data"`
}

// OpenAPIDepartment OpenAPI 返回的部门结构
type OpenAPIDepartment struct {
	UUID        string `json:"uuid"`
	ParentUUID  string `json:"parent_uuid"`
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	MemberCount int    `json:"member_count"`
	Status      int8   `json:"status"`
}

// TenantInfoResp 获取租户信息响应
type TenantInfoResp struct {
	Code any    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TenantUUID string            `json:"tenant_uuid"`
		TenantName string            `json:"tenant_name"`
		App        *TenantAppItem    `json:"app"`
		AIConfig   *TenantAIConfig   `json:"ai_config"`
		License    *TenantLicenseItem `json:"license"`
	} `json:"data"`
}

// TenantAppItem 租户安装的应用信息
type TenantAppItem struct {
	AppID       string `json:"app_id"`
	AppName     string `json:"app_name"`
	AppDesc     string `json:"app_desc"`
	Status      int8   `json:"status"`
	IconURL     string `json:"icon_url"`
	InstalledAt int64  `json:"installed_at"`
}

// TenantAIConfig 租户 AI 配置
type TenantAIConfig struct {
	Model          string `json:"model"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	OutputMaxToken int64  `json:"output_max_token"`
}

// TenantLicenseItem 租户 License 信息
type TenantLicenseItem struct {
	UUID           string           `json:"uuid"`
	LicenseName    string           `json:"license_name"`
	TenantUUID     string           `json:"tenant_uuid"`
	AdminEmail     string           `json:"admin_email"`
	DeploymentType int32            `json:"deployment_type"`
	LicenseType    string           `json:"license_type"`
	IssuedAt       int64            `json:"issued_at"`
	ValidFrom      int64            `json:"valid_from"`
	ValidTo        int64            `json:"valid_to"`
	AppSeatMap     map[string]int32 `json:"app_seat_map"`
}

// VisibilityUsersResp 应用可见性用户列表响应
type VisibilityUsersResp struct {
	Code any    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UserUnionUIDs []string `json:"user_union_uids"`
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
	Schema   string          `json:"schema"`
	Metadata EventMetadata   `json:"metadata"`
	Event    json.RawMessage `json:"event"`
}

// EventMetadata 事件元数据
type EventMetadata struct {
	EventUUID  string `json:"event_uuid"`
	Token      string `json:"token"`
	CreatedAt  int64  `json:"created_at"`
	EventType  string `json:"event_type"`
	TenantUUID string `json:"tenant_uuid"`
	AppID      string `json:"app_id,omitempty"`
}

// ExternalEventUser 用户事件载荷（contact.user.create/update）
type ExternalEventUser struct {
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

// EventContact 群组事件载荷（contact.group.create/update/delete）
type EventContact struct {
	Uuid        string `json:"uuid,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        uint   `json:"type,omitempty"`
	IsPublic    int8   `json:"is_public,omitempty"`
	UserCount   uint   `json:"user_count,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Status      int8   `json:"status,omitempty"`
}

// EventDepartment 部门事件载荷（contact.department.create/update/delete）
type EventDepartment struct {
	Uuid        string `json:"uuid,omitempty"`
	ParentUuid  string `json:"parent_uuid,omitempty"`
	Name        string `json:"name,omitempty"`
	SortOrder   uint   `json:"sort_order,omitempty"`
	MemberCount uint   `json:"member_count,omitempty"`
	Status      int8   `json:"status,omitempty"`
}

// ExternalEventGroup 群组用户变更事件载荷（contact.group.add_users/remove_users）
type ExternalEventGroup struct {
	GroupUuid     string   `json:"group_uuid"`
	UserUnionUIDs []string `json:"user_union_uids"`
}

// ExternalEventDepartmentUsers 部门用户变更事件载荷（contact.department.add_users/remove_users）
type ExternalEventDepartmentUsers struct {
	DepartmentUuid string   `json:"department_uuid"`
	UserUnionUIDs  []string `json:"user_union_uids"`
}

// ExternalEventRole 角色用户变更事件载荷（roles.add_users/remove_users）
type ExternalEventRole struct {
	RoleUuid      string   `json:"role_uuid"`
	RoleType      int      `json:"role_type"`
	UserUnionUIDs []string `json:"user_union_uids"`
}

// EventAppUpdate 应用更新事件载荷（app.update）
type EventAppUpdate struct {
	AppID   string `json:"app_id"`
	AppName string `json:"app_name,omitempty"`
	AppDesc string `json:"app_desc,omitempty"`
}

// EventAppInstall 应用安装事件载荷（app.install）
type EventAppInstall struct {
	AppID   string `json:"app_id"`
	AppName string `json:"app_name"`
}

// EventAppUninstall 应用卸载事件载荷（app.uninstall）
type EventAppUninstall struct {
	AppID   string `json:"app_id"`
	AppName string `json:"app_name"`
}

// EventAppVisibilityUsers 应用可见性用户变更事件载荷（app.visibility.add_users/remove_users）
type EventAppVisibilityUsers struct {
	UserUnionUIDs []string `json:"user_union_uids"`
}

// EventTenantConfigUpdate 租户配置更新事件载荷（tenant.config.update）
type EventTenantConfigUpdate struct {
	ConfigKey  string          `json:"config_key"`
	ConfigData json.RawMessage `json:"config_data"`
}

// ==================== 回调 ====================

// CallbackPayload MWS365 推送的回调载荷
type CallbackPayload struct {
	Schema   string           `json:"schema"`
	Metadata CallbackMetadata `json:"metadata"`
	Callback json.RawMessage  `json:"callback"`
}

// CallbackMetadata 回调元数据
type CallbackMetadata struct {
	CallbackUUID string `json:"callback_uuid"`
	Token        string `json:"token"`
	CreatedAt    int64  `json:"created_at"`
	CallbackType string `json:"callback_type"`
	TenantUUID   string `json:"tenant_uuid"`
	AppID        string `json:"app_id"`
}

// CallbackNotificationButtonClick 通知按钮点击回调载荷
type CallbackNotificationButtonClick struct {
	MsgUuid             string `json:"msg_uuid"`
	ButtonId            string `json:"button_id"`
	ButtonActionPayload string `json:"button_action_payload"`
	UnionUid            string `json:"union_uid"`
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

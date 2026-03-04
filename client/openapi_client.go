package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"mws365-demo-app/model"
)

type OpenAPIClient struct {
	baseURL    string
	appID      string
	appSecret  string
	tenantUUID string
	httpClient *http.Client
}

func NewOpenAPIClient(cfg *model.Config) *OpenAPIClient {
	return &OpenAPIClient{
		baseURL:    cfg.MWS.BaseURL,
		appID:      cfg.OpenAPI.AppID,
		appSecret:  cfg.OpenAPI.AppSecret,
		tenantUUID: cfg.OpenAPI.TenantUUID,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// GetTenantAccessToken 获取 tenant_access_token（每次都调用接口）
func (c *OpenAPIClient) GetTenantAccessToken() (string, error) {
	log.Println("[OpenAPI] 获取 tenant_access_token...")

	reqBody := model.TenantTokenReq{
		AppID:      c.appID,
		AppSecret:  c.appSecret,
		TenantUUID: c.tenantUUID,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := c.httpClient.Post(
		c.baseURL+"/openapi/v1/auth/tenant_access_token",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("请求 tenant_access_token 失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var tokenResp model.TenantTokenResp
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w, body: %s", err, string(respBody))
	}

	if !tokenResp.IsOK() {
		return "", fmt.Errorf("获取失败: code=%v, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	log.Printf("[OpenAPI] tenant_access_token 获取成功，expires_in: %d ms", tokenResp.Data.ExpiresIn)
	return tokenResp.Data.TenantAccessToken, nil
}

// CallAPI 通用 API 调用（自动携带 tenant_access_token，401 自动重试）
func (c *OpenAPIClient) CallAPI(method, path string, reqBody any) ([]byte, error) {
	token, err := c.GetTenantAccessToken()
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if reqBody != nil {
		b, _ := json.Marshal(reqBody)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[OpenAPI] %s %s", method, path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("[OpenAPI] 收到 401，重新获取 token 并重试")

		token, err = c.GetTenantAccessToken()
		if err != nil {
			return nil, err
		}

		req, _ = http.NewRequest(method, c.baseURL+path, bodyReader)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("重试请求失败: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ = io.ReadAll(resp.Body)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ==================== 通讯录接口 ====================

// GetUsers 批量获取用户
// GET /openapi/v1/contact/users?uid_type=union_uid&uids=uid1&uids=uid2
func (c *OpenAPIClient) GetUsers(query url.Values) (json.RawMessage, error) {
	q := url.Values{"uid_type": {"union_uid"}}
	for _, uid := range query["uids"] {
		if uid != "" {
			q.Add("uids", uid)
		}
	}
	for _, k := range []string{"department_uuid", "page_size", "page_token", "fetch_child"} {
		if v := query.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	return c.CallAPI(http.MethodGet, "/openapi/v1/contact/users?"+q.Encode(), nil)
}

// GetDepartmentChildren 获取部门子部门列表
func (c *OpenAPIClient) GetDepartmentChildren(departmentUUID string, params map[string]string) (json.RawMessage, error) {
	path := "/openapi/v1/contact/departments/" + departmentUUID + "/children?"
	for k, v := range params {
		if v != "" {
			path += k + "=" + v + "&"
		}
	}
	return c.CallAPI(http.MethodGet, path, nil)
}

// GetGroupUsers 获取群组成员
func (c *OpenAPIClient) GetGroupUsers(groupUUID string, params map[string]string) (json.RawMessage, error) {
	path := "/openapi/v1/contact/groups/" + groupUUID + "/users?uid_type=union_uid"
	for k, v := range params {
		if v != "" {
			path += "&" + k + "=" + v
		}
	}
	return c.CallAPI(http.MethodGet, path, nil)
}

// GetGroups 获取群组列表
func (c *OpenAPIClient) GetGroups() (json.RawMessage, error) {
	return c.CallAPI(http.MethodGet, "/openapi/v1/contact/groups?uid_type=union_uid", nil)
}

// GetRoleMembers 获取角色成员
func (c *OpenAPIClient) GetRoleMembers(roleUUID string, params map[string]string) (json.RawMessage, error) {
	path := "/openapi/v1/contact/roles/" + roleUUID + "/members?uid_type=union_uid"
	for k, v := range params {
		if v != "" {
			path += "&" + k + "=" + v
		}
	}
	return c.CallAPI(http.MethodGet, path, nil)
}

// GetAllUsers 获取所有用户（大分页，用于全量同步优化）
// GET /openapi/v1/contact/all_users?page_size=500
func (c *OpenAPIClient) GetAllUsers(pageSize int, pageToken string) (json.RawMessage, error) {
	if pageSize <= 0 {
		pageSize = 500
	}
	if pageSize > 500 {
		pageSize = 500
	}
	path := fmt.Sprintf("/openapi/v1/contact/all_users?page_size=%d", pageSize)
	if pageToken != "" {
		path += "&page_token=" + pageToken
	}
	return c.CallAPI(http.MethodGet, path, nil)
}

// GetAllDepartments 获取所有部门（大分页）
// GET /openapi/v1/contact/all_departments?page_size=500
func (c *OpenAPIClient) GetAllDepartments(pageSize int, pageToken string) (json.RawMessage, error) {
	if pageSize <= 0 {
		pageSize = 500
	}
	if pageSize > 500 {
		pageSize = 500
	}
	path := fmt.Sprintf("/openapi/v1/contact/all_departments?page_size=%d", pageSize)
	if pageToken != "" {
		path += "&page_token=" + pageToken
	}
	return c.CallAPI(http.MethodGet, path, nil)
}

// GetTenantInfo 获取租户完整信息（应用、AI配置、License）
// GET /openapi/v1/app/tenant
func (c *OpenAPIClient) GetTenantInfo() (json.RawMessage, error) {
	return c.CallAPI(http.MethodGet, "/openapi/v1/app/tenant", nil)
}

// GetVisibilityUsers 获取应用可见性用户列表
// GET /openapi/v1/app/visibility/users
func (c *OpenAPIClient) GetVisibilityUsers() (json.RawMessage, error) {
	return c.CallAPI(http.MethodGet, "/openapi/v1/app/visibility/users", nil)
}

// ListAllUsers 分页获取所有用户（用于全量同步，使用大分页接口优化）
func (c *OpenAPIClient) ListAllUsers(pageSize int) ([]model.OpenAPIUser, error) {
	var allUsers []model.OpenAPIUser
	pageToken := ""

	for {
		// 使用新的 all_users 接口，默认 500 条/页
		body, err := c.GetAllUsers(pageSize, pageToken)
		if err != nil {
			return nil, err
		}

		var resp model.BatchGetUsersResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		allUsers = append(allUsers, resp.Data.Items...)
		log.Printf("[OpenAPI] 已拉取 %d 个用户", len(allUsers))

		if !resp.Data.HasMore || resp.Data.PageToken == "" {
			break
		}
		pageToken = resp.Data.PageToken
	}

	return allUsers, nil
}

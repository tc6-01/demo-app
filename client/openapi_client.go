package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"mws365-demo-app/model"
)

// OpenAPIClient MWS365 OpenAPI 客户端
type OpenAPIClient struct {
	baseURL    string
	appID      string
	appSecret  string
	tenantUUID string
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewOpenAPIClient 创建 OpenAPI 客户端
func NewOpenAPIClient(cfg *model.Config) *OpenAPIClient {
	return &OpenAPIClient{
		baseURL:    cfg.MWS.BaseURL,
		appID:      cfg.OpenAPI.AppID,
		appSecret:  cfg.OpenAPI.AppSecret,
		tenantUUID: cfg.OpenAPI.TenantUUID,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// GetTenantAccessToken 获取 tenant_access_token，带内存缓存
func (c *OpenAPIClient) GetTenantAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 缓存有效（提前 5 分钟刷新）
	if c.cachedToken != "" && time.Now().Add(5*time.Minute).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

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
		return "", fmt.Errorf("解析 tenant_access_token 响应失败: %w, body: %s", err, string(respBody))
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("获取 tenant_access_token 失败: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	c.cachedToken = tokenResp.TenantAccessToken
	// ExpiresIn 是毫秒
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Millisecond)

	log.Printf("[OpenAPI] tenant_access_token 获取成功，过期时间: %s", c.tokenExpiry.Format(time.RFC3339))
	return c.cachedToken, nil
}

// CallAPI 通用 API 调用（自动携带 tenant_access_token）
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
		// Token 可能已过期，清除缓存，重试一次
		log.Println("[OpenAPI] 收到 401，清除 token 缓存并重试")
		c.mu.Lock()
		c.cachedToken = ""
		c.tokenExpiry = time.Time{}
		c.mu.Unlock()

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
		return nil, fmt.Errorf("API 返回错误: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetUsers 批量获取用户（通过 union_uid）
func (c *OpenAPIClient) GetUsers(unionUIDs string) (*model.BatchGetUsersResp, error) {
	path := "/openapi/contact/v1/users?user_uuids=" + unionUIDs
	body, err := c.CallAPI(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp model.BatchGetUsersResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析用户列表响应失败: %w", err)
	}
	return &resp, nil
}

// ListAllUsers 分页获取所有用户
func (c *OpenAPIClient) ListAllUsers(pageSize int) ([]model.OpenAPIUser, error) {
	var allUsers []model.OpenAPIUser
	pageToken := ""

	for {
		path := fmt.Sprintf("/openapi/contact/v1/users?page_size=%d", pageSize)
		if pageToken != "" {
			path += "&page_token=" + pageToken
		}

		body, err := c.CallAPI(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var resp model.BatchGetUsersResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("解析用户列表响应失败: %w", err)
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

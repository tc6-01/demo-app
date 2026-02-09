package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mws365-demo-app/model"
)

// OAuth2Client MWS365 OAuth2 客户端
type OAuth2Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       string
	httpClient   *http.Client
}

// NewOAuth2Client 创建 OAuth2 客户端
func NewOAuth2Client(cfg *model.Config) *OAuth2Client {
	return &OAuth2Client{
		baseURL:      cfg.MWS.BaseURL,
		clientID:     cfg.OAuth2.ClientID,
		clientSecret: cfg.OAuth2.ClientSecret,
		redirectURI:  cfg.OAuth2.RedirectURI,
		scopes:       cfg.OAuth2.Scopes,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// BuildAuthorizeURL 构建授权跳转 URL
func (c *OAuth2Client) BuildAuthorizeURL(state, nonce string) string {
	params := url.Values{
		"client_id":     {c.clientID},
		"redirect_uri":  {c.redirectURI},
		"response_type": {"code"},
		"scope":         {c.scopes},
		"state":         {state},
		"nonce":         {nonce},
	}
	return c.baseURL + "/oauth2/authorize?" + params.Encode()
}

// ExchangeCode 用授权码换取 Token
func (c *OAuth2Client) ExchangeCode(code string) (*model.OAuth2TokenResp, error) {
	log.Printf("[OAuth2] 用授权码换取 Token: code=%s...", code[:min(len(code), 10)])

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {c.redirectURI},
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/oauth2/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("请求 token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取 token 失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenResp model.OAuth2TokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}

	log.Printf("[OAuth2] Token 获取成功，expires_in=%d", tokenResp.ExpiresIn)
	return &tokenResp, nil
}

// RefreshToken 刷新 access_token
func (c *OAuth2Client) RefreshToken(refreshToken string) (*model.OAuth2TokenResp, error) {
	log.Println("[OAuth2] 刷新 Token...")

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/oauth2/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("刷新 token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("刷新 token 失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenResp model.OAuth2TokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析刷新 token 响应失败: %w", err)
	}

	log.Println("[OAuth2] Token 刷新成功")
	return &tokenResp, nil
}

// GetUserInfo 获取用户信息
func (c *OAuth2Client) GetUserInfo(accessToken string) (*model.OAuth2UserInfo, error) {
	log.Println("[OAuth2] 获取用户信息...")

	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户信息失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var userInfo model.OAuth2UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	log.Printf("[OAuth2] 用户信息: sub=%s, name=%s, email=%s", userInfo.Sub, userInfo.Name, userInfo.Email)
	return &userInfo, nil
}

// BuildLogoutURL 构建登出跳转 URL
func (c *OAuth2Client) BuildLogoutURL(postLogoutRedirectURI, refreshToken string) string {
	params := url.Values{
		"post_logout_redirect_uri": {postLogoutRedirectURI},
	}
	if refreshToken != "" {
		params.Set("refresh_token", refreshToken)
	}
	return c.baseURL + "/oauth2/logout?" + params.Encode()
}

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

type OAuth2Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       string
	httpClient   *http.Client
}

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
	return &tokenResp, nil
}

func (c *OAuth2Client) RefreshToken(refreshToken string) (*model.OAuth2TokenResp, error) {
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
	return &tokenResp, nil
}

func (c *OAuth2Client) GetUserInfo(accessToken string) (*model.OAuth2UserInfo, error) {
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
	return &userInfo, nil
}

func (c *OAuth2Client) BuildLogoutURL(postLogoutRedirectURI, refreshToken string) string {
	params := url.Values{
		"post_logout_redirect_uri": {postLogoutRedirectURI},
	}
	if refreshToken != "" {
		params.Set("refresh_token", refreshToken)
	}
	return c.baseURL + "/oauth2/logout?" + params.Encode()
}

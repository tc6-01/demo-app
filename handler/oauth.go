package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"mws365-demo-app/client"
	"mws365-demo-app/model"
)

// SessionStore 内存 Session 存储
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*model.Session // sessionID -> Session
	states   map[string]time.Time      // OAuth2 state -> 创建时间（防 CSRF）
}

// NewSessionStore 创建 Session 存储
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*model.Session),
		states:   make(map[string]time.Time),
	}
}

// GetSession 获取 Session
func (s *SessionStore) GetSession(sessionID string) *model.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

// SetSession 保存 Session
func (s *SessionStore) SetSession(sess *model.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

// DeleteSession 删除 Session
func (s *SessionStore) DeleteSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// SaveState 保存 OAuth2 state
func (s *SessionStore) SaveState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = time.Now()
}

// ValidateState 验证并消费 OAuth2 state
func (s *SessionStore) ValidateState(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	created, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	// state 10 分钟内有效
	return time.Since(created) < 10*time.Minute
}

// OAuthHandler OAuth2 登录处理器
type OAuthHandler struct {
	oauth2Client *client.OAuth2Client
	sessions     *SessionStore
	baseURL      string // 应用的 base URL
}

// NewOAuthHandler 创建 OAuth2 处理器
func NewOAuthHandler(oauth2Client *client.OAuth2Client, sessions *SessionStore, baseURL string) *OAuthHandler {
	return &OAuthHandler{
		oauth2Client: oauth2Client,
		sessions:     sessions,
		baseURL:      baseURL,
	}
}

// HandleLogin 发起 OAuth2 登录
// GET /oauth2/login
func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomString(32)
	nonce := randomString(32)

	h.sessions.SaveState(state)

	authorizeURL := h.oauth2Client.BuildAuthorizeURL(state, nonce)
	log.Printf("[OAuth2] 跳转到授权页: %s", authorizeURL)
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

// HandleCallback 处理 OAuth2 回调
// GET /oauth2/callback
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	// 处理错误回调
	if errParam != "" {
		log.Printf("[OAuth2] 授权失败: error=%s", errParam)
		http.Redirect(w, r, "/?error="+errParam, http.StatusFound)
		return
	}

	// 验证 state
	if !h.sessions.ValidateState(state) {
		log.Printf("[OAuth2] state 验证失败")
		http.Error(w, "state 验证失败，可能存在 CSRF 攻击", http.StatusBadRequest)
		return
	}

	if code == "" {
		http.Error(w, "缺少授权码", http.StatusBadRequest)
		return
	}

	// 用授权码换 Token
	tokenResp, err := h.oauth2Client.ExchangeCode(code)
	if err != nil {
		log.Printf("[OAuth2] 换取 Token 失败: %v", err)
		http.Error(w, "换取 Token 失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取用户信息
	userInfo, err := h.oauth2Client.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		log.Printf("[OAuth2] 获取用户信息失败: %v", err)
		http.Error(w, "获取用户信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 创建 Session
	sessionID := randomString(32)
	sess := &model.Session{
		ID:           sessionID,
		UserInfo:     userInfo,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		CreatedAt:    time.Now(),
	}
	h.sessions.SetSession(sess)

	// 设置 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400, // 1 天
	})

	log.Printf("[OAuth2] 登录成功: user=%s (%s)", userInfo.Name, userInfo.Email)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// HandleLogout 登出
// GET /oauth2/logout
func (h *OAuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	var refreshToken string

	if sessionID != "" {
		if sess := h.sessions.GetSession(sessionID); sess != nil {
			refreshToken = sess.RefreshToken
		}
		h.sessions.DeleteSession(sessionID)
	}

	// 清除 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// 跳转到 MWS 登出
	logoutURL := h.oauth2Client.BuildLogoutURL(h.baseURL, refreshToken)
	log.Printf("[OAuth2] 登出，跳转: %s", logoutURL)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

// HandleRefresh 刷新 Token
// GET /oauth2/refresh
func (h *OAuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	if sessionID == "" {
		http.Error(w, "未登录", http.StatusUnauthorized)
		return
	}

	sess := h.sessions.GetSession(sessionID)
	if sess == nil {
		http.Error(w, "Session 不存在", http.StatusUnauthorized)
		return
	}

	tokenResp, err := h.oauth2Client.RefreshToken(sess.RefreshToken)
	if err != nil {
		log.Printf("[OAuth2] 刷新 Token 失败: %v", err)
		http.Error(w, "刷新 Token 失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新 Session
	sess.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		sess.RefreshToken = tokenResp.RefreshToken
	}
	sess.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	h.sessions.SetSession(sess)

	log.Println("[OAuth2] Token 已刷新")
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// GetCurrentSession 从请求中获取当前 Session（供其他 handler 使用）
func (h *OAuthHandler) GetCurrentSession(r *http.Request) *model.Session {
	sessionID := getSessionID(r)
	if sessionID == "" {
		return nil
	}
	return h.sessions.GetSession(sessionID)
}

// getSessionID 从 Cookie 中获取 session_id
func getSessionID(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// randomString 生成随机十六进制字符串
func randomString(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

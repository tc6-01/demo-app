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

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*model.Session
	states   map[string]time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*model.Session),
		states:   make(map[string]time.Time),
	}
}

func (s *SessionStore) GetSession(sessionID string) *model.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

func (s *SessionStore) SetSession(sess *model.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

func (s *SessionStore) DeleteSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *SessionStore) SaveState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = time.Now()
}

func (s *SessionStore) ValidateState(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	created, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Since(created) < 10*time.Minute
}

type OAuthHandler struct {
	oauth2Client *client.OAuth2Client
	sessions     *SessionStore
	baseURL      string
}

func NewOAuthHandler(oauth2Client *client.OAuth2Client, sessions *SessionStore, baseURL string) *OAuthHandler {
	return &OAuthHandler{
		oauth2Client: oauth2Client,
		sessions:     sessions,
		baseURL:      baseURL,
	}
}

// HandleLogin 跳转到 MWS OAuth2 授权页面
func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomString(32)
	nonce := randomString(16)
	h.sessions.SaveState(state)
	authURL := h.oauth2Client.BuildAuthorizeURL(state, nonce)
	log.Printf("[OAuth2] 跳转授权页面: %s", authURL)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback OAuth2 授权回调
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		http.Redirect(w, r, "/?error="+errParam, http.StatusFound)
		return
	}

	if !h.sessions.ValidateState(state) {
		http.Error(w, "state 验证失败", http.StatusBadRequest)
		return
	}

	if code == "" {
		http.Error(w, "缺少授权码", http.StatusBadRequest)
		return
	}

	tokenResp, err := h.oauth2Client.ExchangeCode(code)
	if err != nil {
		http.Error(w, "换取 Token 失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userInfo, err := h.oauth2Client.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		http.Error(w, "获取用户信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

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

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	log.Printf("[OAuth2] 登录成功: user=%s (%s)", userInfo.Name, userInfo.Email)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *OAuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	var refreshToken string
	if sessionID != "" {
		if sess := h.sessions.GetSession(sessionID); sess != nil {
			refreshToken = sess.RefreshToken
		}
		h.sessions.DeleteSession(sessionID)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	logoutURL := h.oauth2Client.BuildLogoutURL(h.baseURL+"/", refreshToken)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

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

	if sess.RefreshToken != "" {
		tokenResp, err := h.oauth2Client.RefreshToken(sess.RefreshToken)
		if err != nil {
			http.Error(w, "刷新 Token 失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		sess.AccessToken = tokenResp.AccessToken
		if tokenResp.RefreshToken != "" {
			sess.RefreshToken = tokenResp.RefreshToken
		}
		sess.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else {
		sess.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	h.sessions.SetSession(sess)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *OAuthHandler) GetCurrentSession(r *http.Request) *model.Session {
	sessionID := getSessionID(r)
	if sessionID == "" {
		return nil
	}
	return h.sessions.GetSession(sessionID)
}

func getSessionID(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func randomString(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

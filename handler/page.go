package handler

import (
	"html/template"
	"log"
	"net/http"

	"mws365-demo-app/store"
)

// PageHandler 页面渲染处理器
type PageHandler struct {
	oauthHandler *OAuthHandler
	userMode     string
	templates    *template.Template
}

// NewPageHandler 创建页面处理器
func NewPageHandler(oauthHandler *OAuthHandler, userMode string) *PageHandler {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("[Page] 加载模板失败: %v", err)
	}

	return &PageHandler{
		oauthHandler: oauthHandler,
		userMode:     userMode,
		templates:    tmpl,
	}
}

// HandleIndex 首页
// GET /
func (h *PageHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	sess := h.oauthHandler.GetCurrentSession(r)

	data := map[string]any{
		"LoggedIn": sess != nil,
		"Error":    r.URL.Query().Get("error"),
	}
	if sess != nil {
		data["User"] = sess.UserInfo
	}

	h.templates.ExecuteTemplate(w, "index.html", data)
}

// HandleDashboard 仪表盘
// GET /dashboard
func (h *PageHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	sess := h.oauthHandler.GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// 获取最近事件
	events, err := store.ListRecentEvents(20)
	if err != nil {
		log.Printf("[Page] 查询事件失败: %v", err)
	}

	// 模式 B：获取本地用户统计
	var userCount int64
	if h.userMode == "local" {
		userCount, _ = store.CountUsers()
	}

	data := map[string]any{
		"User":      sess.UserInfo,
		"UserMode":  h.userMode,
		"UserCount": userCount,
		"Events":    events,
		"Session":   sess,
	}

	h.templates.ExecuteTemplate(w, "dashboard.html", data)
}

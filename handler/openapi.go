package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"mws365-demo-app/client"
	"mws365-demo-app/model"
	"mws365-demo-app/store"
	contactSync "mws365-demo-app/sync"
)

// OpenAPIHandler OpenAPI 演示处理器
type OpenAPIHandler struct {
	apiClient *client.OpenAPIClient
	userMode  string
}

// NewOpenAPIHandler 创建 OpenAPI 处理器
func NewOpenAPIHandler(apiClient *client.OpenAPIClient, userMode string) *OpenAPIHandler {
	return &OpenAPIHandler{
		apiClient: apiClient,
		userMode:  userMode,
	}
}

// HandleGetToken 获取 tenant_access_token
// GET /api/tenant-token
func (h *OpenAPIHandler) HandleGetToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.apiClient.GetTenantAccessToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{
			Code: -1,
			Msg:  "获取 Token 失败: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.APIResponse{
		Code: 0,
		Msg:  "success",
		Data: map[string]string{
			"tenant_access_token": token,
		},
	})
}

// HandleGetUsers 查询用户
// GET /api/users?uids=uid1,uid2
func (h *OpenAPIHandler) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	if h.userMode == "local" {
		// 模式 B：查本地表
		users, err := store.ListUsers(50, 0)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.APIResponse{
				Code: -1,
				Msg:  "查询本地用户失败: " + err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, model.APIResponse{
			Code: 0,
			Msg:  "success (local)",
			Data: users,
		})
		return
	}

	// 模式 A：实时调 OpenAPI
	uids := r.URL.Query().Get("uids")
	if uids == "" {
		writeJSON(w, http.StatusBadRequest, model.APIResponse{
			Code: -1,
			Msg:  "参数 uids 不能为空（模式 A 需要指定用户 union_uid）",
		})
		return
	}

	resp, err := h.apiClient.GetUsers(uids)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{
			Code: -1,
			Msg:  "查询用户失败: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.APIResponse{
		Code: 0,
		Msg:  "success (openapi)",
		Data: resp.Data.Items,
	})
}

// HandleSync 触发全量同步（仅模式 B）
// POST /api/sync
func (h *OpenAPIHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	if h.userMode != "local" {
		writeJSON(w, http.StatusBadRequest, model.APIResponse{
			Code: -1,
			Msg:  "当前为 mws 模式，不支持同步",
		})
		return
	}

	go func() {
		if err := contactSync.SyncAllUsers(h.apiClient); err != nil {
			log.Printf("[API] 全量同步失败: %v", err)
		}
	}()

	writeJSON(w, http.StatusOK, model.APIResponse{
		Code: 0,
		Msg:  "全量同步已启动（后台执行）",
	})
}

// HandleListEvents 查看事件日志
// GET /api/events
func (h *OpenAPIHandler) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := store.ListRecentEvents(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{
			Code: -1,
			Msg:  "查询事件失败: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.APIResponse{
		Code: 0,
		Msg:  "success",
		Data: events,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

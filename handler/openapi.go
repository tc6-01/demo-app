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

type OpenAPIHandler struct {
	apiClient *client.OpenAPIClient
	userMode  string
}

func NewOpenAPIHandler(apiClient *client.OpenAPIClient, userMode string) *OpenAPIHandler {
	return &OpenAPIHandler{apiClient: apiClient, userMode: userMode}
}

// HandleGetToken 获取 tenant_access_token
func (h *OpenAPIHandler) HandleGetToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.apiClient.GetTenantAccessToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "获取 Token 失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Code: 0, Msg: "success", Data: map[string]string{"tenant_access_token": token}})
}

// HandleGetUsers 查询用户
func (h *OpenAPIHandler) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	if h.userMode == "local" {
		users, err := store.ListUsers(50, 0)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询失败: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, model.APIResponse{Code: 0, Msg: "success (local)", Data: users})
		return
	}

	body, err := h.apiClient.GetUsers(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询用户失败: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

// HandleGetDepartments 查询部门子部门
func (h *OpenAPIHandler) HandleGetDepartments(w http.ResponseWriter, r *http.Request) {
	deptUUID := r.URL.Query().Get("department_uuid")
	if deptUUID == "" {
		deptUUID = "0"
	}
	params := map[string]string{
		"fetch_child": r.URL.Query().Get("fetch_child"),
		"page_size":   r.URL.Query().Get("page_size"),
		"page_token":  r.URL.Query().Get("page_token"),
	}
	body, err := h.apiClient.GetDepartmentChildren(deptUUID, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询部门失败: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

// HandleGetGroups 查询群组列表
func (h *OpenAPIHandler) HandleGetGroups(w http.ResponseWriter, r *http.Request) {
	body, err := h.apiClient.GetGroups()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询群组失败: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

// HandleGetGroupUsers 查询群组成员
func (h *OpenAPIHandler) HandleGetGroupUsers(w http.ResponseWriter, r *http.Request) {
	groupUUID := r.URL.Query().Get("group_uuid")
	if groupUUID == "" {
		writeJSON(w, http.StatusBadRequest, model.APIResponse{Code: -1, Msg: "参数 group_uuid 不能为空"})
		return
	}
	params := map[string]string{
		"page_size":  r.URL.Query().Get("page_size"),
		"page_token": r.URL.Query().Get("page_token"),
	}
	body, err := h.apiClient.GetGroupUsers(groupUUID, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询群组成员失败: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

// HandleGetRoleMembers 查询角色成员
func (h *OpenAPIHandler) HandleGetRoleMembers(w http.ResponseWriter, r *http.Request) {
	roleUUID := r.URL.Query().Get("role_uuid")
	if roleUUID == "" {
		writeJSON(w, http.StatusBadRequest, model.APIResponse{Code: -1, Msg: "参数 role_uuid 不能为空"})
		return
	}
	params := map[string]string{
		"page_size":  r.URL.Query().Get("page_size"),
		"page_token": r.URL.Query().Get("page_token"),
	}
	body, err := h.apiClient.GetRoleMembers(roleUUID, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询角色成员失败: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

// HandleSync 触发全量同步（仅 local 模式）
func (h *OpenAPIHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	if h.userMode != "local" {
		writeJSON(w, http.StatusBadRequest, model.APIResponse{Code: -1, Msg: "当前为 mws 模式，不支持同步"})
		return
	}
	go func() {
		if err := contactSync.SyncAllUsers(h.apiClient); err != nil {
			log.Printf("[API] 全量同步失败: %v", err)
		}
	}()
	writeJSON(w, http.StatusOK, model.APIResponse{Code: 0, Msg: "全量同步已启动（后台执行）"})
}

// HandleListEvents 查看事件日志
func (h *OpenAPIHandler) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := store.ListRecentEvents(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.APIResponse{Code: -1, Msg: "查询事件失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Code: 0, Msg: "success", Data: events})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

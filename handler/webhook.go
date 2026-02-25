package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"mws365-demo-app/model"
	"mws365-demo-app/signature"
	"mws365-demo-app/store"
)

// WebhookHandler 事件/回调 Webhook 处理器
type WebhookHandler struct {
	encryptKey string
	userMode   string // "mws" or "local"
}

// NewWebhookHandler 创建 Webhook 处理器
func NewWebhookHandler(encryptKey, userMode string) *WebhookHandler {
	return &WebhookHandler{
		encryptKey: encryptKey,
		userMode:   userMode,
	}
}

// HandleEvents 处理事件推送
// POST /webhook/events
func (h *WebhookHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Webhook] 读取请求体失败: %v", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	// 签名验证（如果配置了 encrypt_key）
	if h.encryptKey != "" {
		timestamp := r.Header.Get("X-MWS-Request-Timestamp")
		nonce := r.Header.Get("X-MWS-Request-Nonce")
		sig := r.Header.Get("X-MWS-Signature")

		if !signature.Verify(h.encryptKey, timestamp, nonce, string(body), sig) {
			log.Printf("[Webhook] 签名验证失败: timestamp=%s, nonce=%s", timestamp, nonce)
			http.Error(w, "signature verification failed", http.StatusForbidden)
			return
		}
		log.Println("[Webhook] 签名验证通过")
	}

	// 解析事件
	var payload model.EventPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Webhook] 解析事件失败: %v", err)
		http.Error(w, "parse event failed", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] 收到事件: type=%s, uuid=%s, tenant=%s, app=%s",
		payload.Metadata.EventType, payload.Metadata.EventUUID,
		payload.Metadata.TenantUUID, payload.Metadata.AppID)

	// 事件落库 + 去重（INSERT IGNORE）
	isNew, err := store.InsertEventLog(
		payload.Metadata.EventUUID,
		payload.Metadata.EventType,
		payload.Metadata.TenantUUID,
		payload.Metadata.AppID,
		string(body),
	)
	if err != nil {
		log.Printf("[Webhook] 事件落库失败: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if !isNew {
		log.Printf("[Webhook] 重复事件，跳过: uuid=%s", payload.Metadata.EventUUID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 处理事件
	h.processEvent(payload)

	// 标记已处理
	if err := store.MarkEventProcessed(payload.Metadata.EventUUID); err != nil {
		log.Printf("[Webhook] 标记事件已处理失败: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

// processEvent 按事件类型分发处理
func (h *WebhookHandler) processEvent(payload model.EventPayload) {
	eventType := payload.Metadata.EventType

	switch eventType {
	// 用户事件
	case "contact.user.create", "contact.user.update":
		var user model.ExternalEventUser
		if err := json.Unmarshal(payload.Event, &user); err != nil {
			log.Printf("[Webhook] 解析用户事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 用户事件: type=%s, uid=%s, name=%s", eventType, user.UnionUID, user.Name)
		if h.userMode == "local" {
			if err := store.UpsertUserFromEvent(&user); err != nil {
				log.Printf("[Webhook] 更新本地用户失败: uid=%s, err=%v", user.UnionUID, err)
			} else {
				log.Printf("[Webhook] 本地用户已更新: uid=%s", user.UnionUID)
			}
		}

	// 群组事件
	case "contact.group.create", "contact.group.update", "contact.group.delete":
		var group model.EventContact
		if err := json.Unmarshal(payload.Event, &group); err != nil {
			log.Printf("[Webhook] 解析群组事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 群组事件: type=%s, uuid=%s, name=%s", eventType, group.Uuid, group.Name)

	// 群组成员变更
	case "contact.group.add_users", "contact.group.remove_users":
		var group model.ExternalEventGroup
		if err := json.Unmarshal(payload.Event, &group); err != nil {
			log.Printf("[Webhook] 解析群组成员事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 群组成员事件: type=%s, group=%s, users=%v", eventType, group.GroupUuid, group.UserUnionUIDs)

	// 部门事件
	case "contact.department.create", "contact.department.update", "contact.department.delete":
		var dept model.EventDepartment
		if err := json.Unmarshal(payload.Event, &dept); err != nil {
			log.Printf("[Webhook] 解析部门事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 部门事件: type=%s, uuid=%s, name=%s", eventType, dept.Uuid, dept.Name)

	// 部门成员变更
	case "contact.department.add_users", "contact.department.remove_users":
		var dept model.ExternalEventDepartmentUsers
		if err := json.Unmarshal(payload.Event, &dept); err != nil {
			log.Printf("[Webhook] 解析部门成员事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 部门成员事件: type=%s, dept=%s, users=%v", eventType, dept.DepartmentUuid, dept.UserUnionUIDs)

	// 角色成员变更
	case "roles.add_users", "roles.remove_users":
		var role model.ExternalEventRole
		if err := json.Unmarshal(payload.Event, &role); err != nil {
			log.Printf("[Webhook] 解析角色事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 角色事件: type=%s, role=%s, users=%v", eventType, role.RoleUuid, role.UserUnionUIDs)

	// 应用更新
	case "app.update":
		var app model.EventAppUpdate
		if err := json.Unmarshal(payload.Event, &app); err != nil {
			log.Printf("[Webhook] 解析应用事件失败: %v", err)
			return
		}
		log.Printf("[Webhook] 应用事件: type=%s, app_id=%s, name=%s", eventType, app.AppID, app.AppName)

	default:
		log.Printf("[Webhook] 未知事件类型 %s，已记录", eventType)
	}
}

// HandleCallbacks 处理回调推送
// POST /webhook/callbacks
func (h *WebhookHandler) HandleCallbacks(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Webhook] 读取回调请求体失败: %v", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	// 签名验证
	if h.encryptKey != "" {
		timestamp := r.Header.Get("X-MWS-Request-Timestamp")
		nonce := r.Header.Get("X-MWS-Request-Nonce")
		sig := r.Header.Get("X-MWS-Signature")

		if !signature.Verify(h.encryptKey, timestamp, nonce, string(body), sig) {
			log.Printf("[Webhook] 回调签名验证失败")
			http.Error(w, "signature verification failed", http.StatusForbidden)
			return
		}
	}

	var payload model.CallbackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Webhook] 解析回调失败: %v", err)
		http.Error(w, "parse callback failed", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] 收到回调: type=%s, uuid=%s, app=%s",
		payload.Metadata.CallbackType, payload.Metadata.CallbackUUID, payload.Metadata.AppID)

	// 按回调类型处理
	switch payload.Metadata.CallbackType {
	case "notify.button.click":
		var click model.CallbackNotificationButtonClick
		if err := json.Unmarshal(payload.Callback, &click); err != nil {
			log.Printf("[Webhook] 解析按钮回调失败: %v", err)
		} else {
			log.Printf("[Webhook] 按钮回调: msg=%s, button=%s, uid=%s",
				click.MsgUuid, click.ButtonId, click.UnionUid)
		}
	default:
		log.Printf("[Webhook] 未知回调类型: %s", payload.Metadata.CallbackType)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

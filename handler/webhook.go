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

	log.Printf("[Webhook] 收到事件: type=%s, uuid=%s, tenant=%s",
		payload.Metadata.EventType, payload.Metadata.EventUUID, payload.Metadata.TenantUUID)

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
		// 仍然返回 200，避免 MWS 重复推送
		w.WriteHeader(http.StatusOK)
		return
	}

	if !isNew {
		log.Printf("[Webhook] 重复事件，跳过: uuid=%s", payload.Metadata.EventUUID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 模式 B：增量更新本地用户表
	if h.userMode == "local" {
		h.processUserEvent(payload.Metadata.EventType, body)
	}

	// 标记已处理
	if err := store.MarkEventProcessed(payload.Metadata.EventUUID); err != nil {
		log.Printf("[Webhook] 标记事件已处理失败: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

// processUserEvent 处理用户相关事件，更新本地 users 表
func (h *WebhookHandler) processUserEvent(eventType string, rawBody []byte) {
	switch eventType {
	case "contact.user.create", "contact.user.update":
		// 重新解析拿到 event 字段
		var full struct {
			Event model.EventUser `json:"event"`
		}
		if err := json.Unmarshal(rawBody, &full); err != nil {
			log.Printf("[Webhook] 解析用户事件失败: %v", err)
			return
		}

		if err := store.UpsertUserFromEvent(&full.Event); err != nil {
			log.Printf("[Webhook] 更新本地用户失败: uid=%s, err=%v", full.Event.UnionUID, err)
		} else {
			log.Printf("[Webhook] 本地用户已更新: uid=%s, name=%s", full.Event.UnionUID, full.Event.Name)
		}
	default:
		// 其他事件类型仅记录日志
		log.Printf("[Webhook] 事件 %s 已记录（不触发用户同步）", eventType)
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

	log.Printf("[Webhook] 收到回调: type=%s, uuid=%s",
		payload.Metadata.CallbackType, payload.Metadata.CallbackUUID)

	// 回调需要返回响应内容
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

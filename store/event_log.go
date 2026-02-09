package store

import (
	"database/sql"
	"log"

	"mws365-demo-app/model"
)

// InsertEventLog 插入事件日志，利用唯一索引去重（INSERT IGNORE）
// 返回 true 表示是新事件，false 表示重复事件
func InsertEventLog(eventUUID, eventType, tenantUUID, appID, payload string) (bool, error) {
	result, err := DB.Exec(
		`INSERT IGNORE INTO event_logs (event_uuid, event_type, tenant_uuid, app_id, payload)
		 VALUES (?, ?, ?, ?, ?)`,
		eventUUID, eventType, tenantUUID, appID, payload,
	)
	if err != nil {
		return false, err
	}

	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// MarkEventProcessed 标记事件已处理
func MarkEventProcessed(eventUUID string) error {
	_, err := DB.Exec(
		`UPDATE event_logs SET processed = 1 WHERE event_uuid = ?`,
		eventUUID,
	)
	return err
}

// ListRecentEvents 查询最近的事件日志
func ListRecentEvents(limit int) ([]model.DBEventLog, error) {
	rows, err := DB.Query(
		`SELECT id, event_uuid, event_type, tenant_uuid, app_id, payload, processed, created_at
		 FROM event_logs ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.DBEventLog
	for rows.Next() {
		var e model.DBEventLog
		var payload sql.NullString
		if err := rows.Scan(&e.ID, &e.EventUUID, &e.EventType, &e.TenantUUID, &e.AppID, &payload, &e.Processed, &e.CreatedAt); err != nil {
			log.Printf("[DB] scan event_log error: %v", err)
			continue
		}
		e.Payload = payload.String
		events = append(events, e)
	}
	return events, nil
}

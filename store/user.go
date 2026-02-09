package store

import (
	"log"

	"mws365-demo-app/model"
)

// UpsertUser 插入或更新用户（UPSERT）
func UpsertUser(u *model.OpenAPIUser) error {
	_, err := DB.Exec(
		`INSERT INTO users (union_uid, name, nickname, email, mobile, avatar, status, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
		 ON DUPLICATE KEY UPDATE
		   name = VALUES(name),
		   nickname = VALUES(nickname),
		   email = VALUES(email),
		   mobile = VALUES(mobile),
		   avatar = VALUES(avatar),
		   status = VALUES(status),
		   synced_at = NOW()`,
		u.UnionUID, u.Name, u.Nickname, u.Email, u.Mobile, u.Avatar, u.Status,
	)
	return err
}

// UpsertUserFromEvent 从事件数据更新用户
func UpsertUserFromEvent(u *model.EventUser) error {
	_, err := DB.Exec(
		`INSERT INTO users (union_uid, name, nickname, email, mobile, avatar, status, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
		 ON DUPLICATE KEY UPDATE
		   name = VALUES(name),
		   nickname = VALUES(nickname),
		   email = VALUES(email),
		   mobile = VALUES(mobile),
		   avatar = VALUES(avatar),
		   status = VALUES(status),
		   synced_at = NOW()`,
		u.UnionUID, u.Name, u.Nickname, u.Email, u.Mobile, u.Avatar, u.Status,
	)
	return err
}

// ListUsers 查询本地用户列表
func ListUsers(limit, offset int) ([]model.DBUser, error) {
	rows, err := DB.Query(
		`SELECT id, union_uid, name, nickname, email, mobile, avatar, status, synced_at, created_at, updated_at
		 FROM users ORDER BY id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.DBUser
	for rows.Next() {
		var u model.DBUser
		if err := rows.Scan(&u.ID, &u.UnionUID, &u.Name, &u.Nickname, &u.Email, &u.Mobile, &u.Avatar, &u.Status, &u.SyncedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			log.Printf("[DB] scan user error: %v", err)
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// CountUsers 统计用户总数
func CountUsers() (int64, error) {
	var count int64
	err := DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

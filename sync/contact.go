package sync

import (
	"log"

	"mws365-demo-app/client"
	"mws365-demo-app/store"
)

// SyncAllUsers 全量同步所有用户到本地 users 表
func SyncAllUsers(apiClient *client.OpenAPIClient) error {
	log.Println("[Sync] 开始全量同步用户...")

	users, err := apiClient.ListAllUsers(100)
	if err != nil {
		return err
	}

	successCount := 0
	failCount := 0

	for i := range users {
		if err := store.UpsertUser(&users[i]); err != nil {
			log.Printf("[Sync] 同步用户 %s 失败: %v", users[i].UnionUID, err)
			failCount++
		} else {
			successCount++
		}
	}

	log.Printf("[Sync] 全量同步完成: 成功=%d, 失败=%d, 总计=%d", successCount, failCount, len(users))
	return nil
}

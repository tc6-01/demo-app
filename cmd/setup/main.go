package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// 配置模板
const configTemplate = `server:
  port: {{.ServerPort}}
  base_url: "{{.AppBaseURL}}"

app:
  user_mode: "mws"  # "mws" = 直接复用 MWS 用户, "local" = 本地 users 表

mysql:
  dsn: "{{.DemoDSN}}"

mws:
  base_url: "{{.MWSBaseURL}}"

openapi:
  app_id: "{{.AppID}}"
  app_secret: "{{.AppSecret}}"
  tenant_uuid: "{{.TenantUUID}}"
  encrypt_key: "{{.EncryptKey}}"

oauth2:
  client_id: "{{.AppID}}"
  client_secret: "{{.AppSecret}}"
  redirect_uri: "{{.AppBaseURL}}/oauth2/callback"
  scopes: "openid profile email"
`

type setupConfig struct {
	ServerPort int
	AppBaseURL string
	DemoDSN    string
	MWSBaseURL string
	AppID      string
	AppSecret  string
	TenantUUID string
	EncryptKey string
}

func main() {
	// 命令行参数
	mwsDSN := flag.String("mws-dsn", "", "MWS 平台数据库 DSN（必填），例: root:123456@tcp(127.0.0.1:3306)/mws365")
	mwsBaseURL := flag.String("mws-base-url", "https://uccp-dev.shimorelease.com", "MWS 服务地址")
	tenantID := flag.Int64("tenant-id", 0, "租户 ID（不传则自动创建新租户）")
	tenantName := flag.String("tenant-name", "Demo Tenant", "新建租户名称（仅在未指定 tenant-id 时生效）")
	tenantDomain := flag.String("tenant-domain", "", "新建租户域名（可选）")
	appName := flag.String("app-name", "MWS365 Demo App", "应用名称")
	appBaseURL := flag.String("app-base-url", "http://localhost:8080", "Demo App 外部访问地址")
	demoDSN := flag.String("demo-dsn", "root:123456@tcp(127.0.0.1:3306)/mws365_demo?charset=utf8mb4&parseTime=true&loc=Local", "Demo App 自身数据库 DSN")
	serverPort := flag.Int("port", 8080, "Demo App 监听端口")
	output := flag.String("output", "", "输出 config.yaml 的路径（为空则输出到 stdout）")
	dryRun := flag.Bool("dry-run", false, "仅打印将要执行的 SQL，不实际执行")
	flag.Parse()

	if *mwsDSN == "" && !*dryRun {
		log.Fatal("必须指定 --mws-dsn 参数（或使用 --dry-run 仅输出 SQL）")
	}

	// 生成凭证
	appID := "demo_" + randomHex(8)
	appSecret := randomHex(32)
	encryptKey := randomHex(16)

	log.Println("========== MWS365 Demo App Setup ==========")
	log.Printf("应用名称:    %s", *appName)
	log.Printf("应用 ID:     %s", appID)
	log.Printf("应用密钥:    %s", appSecret)
	log.Printf("加密密钥:    %s", encryptKey)
	log.Printf("MWS 地址:    %s", *mwsBaseURL)
	if *tenantID > 0 {
		log.Printf("租户 ID:     %d（使用已有租户）", *tenantID)
	} else {
		log.Printf("租户名称:    %s（将自动创建）", *tenantName)
	}
	log.Printf("App 地址:    %s", *appBaseURL)
	log.Println("============================================")

	// 生成 UUID bytes
	appUUID := randomUUID()
	oauthUUID := randomUUID()
	tenantAppUUID := randomUUID()
	tenantUUID := randomUUID()

	now := time.Now().UnixMilli()
	createTenant := *tenantID <= 0

	// 获取或创建租户，拿到 tenant UUID hex
	var tenantUUIDHex string

	if *dryRun {
		log.Println("\n[DRY-RUN] 将要执行的 SQL：")
		if createTenant {
			printDryRunTenantSQL(tenantUUID, *tenantName, *tenantDomain, now)
		}
		printDryRunSQL(appUUID, oauthUUID, tenantAppUUID, appID, appSecret, encryptKey, *appName, *appBaseURL, *tenantID, now)
		tenantUUIDHex = "<tenant-uuid>"
	} else {
		db, err := sql.Open("mysql", *mwsDSN)
		if err != nil {
			log.Fatalf("连接 MWS 数据库失败: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Fatalf("MWS 数据库连接测试失败: %v", err)
		}
		log.Println("[DB] MWS 数据库连接成功")

		// 创建或查询租户
		if createTenant {
			newID, err := createNewTenant(db, tenantUUID, *tenantName, *tenantDomain, now)
			if err != nil {
				log.Fatalf("创建租户失败: %v", err)
			}
			*tenantID = newID
			tenantUUIDHex = hex.EncodeToString(tenantUUID)
			log.Printf("[DB] 已创建租户: id=%d, name=%s, uuid=%s", *tenantID, *tenantName, tenantUUIDHex)
		} else {
			tenantUUIDHex, err = getTenantUUID(db, *tenantID)
			if err != nil {
				log.Fatalf("查询租户 UUID 失败: %v", err)
			}
			log.Printf("[DB] 使用已有租户: id=%d, uuid=%s", *tenantID, tenantUUIDHex)
		}

		// 检查 app_id 是否已存在
		exists, err := appExists(db, appID)
		if err != nil {
			log.Fatalf("检查应用是否存在失败: %v", err)
		}
		if exists {
			log.Fatalf("应用 app_id=%s 已存在，请重新运行", appID)
		}

		// 执行插入
		if err := insertRecords(db, appUUID, oauthUUID, tenantAppUUID, appID, appSecret, encryptKey, *appName, *appBaseURL, *tenantID, now); err != nil {
			log.Fatalf("插入数据失败: %v", err)
		}

		log.Println("[DB] 数据插入成功")
	}

	// 格式化 tenant UUID 为带横线格式
	tenantUUIDFormatted := formatUUID(tenantUUIDHex)

	// 生成 config.yaml
	cfg := setupConfig{
		ServerPort: *serverPort,
		AppBaseURL: *appBaseURL,
		DemoDSN:    *demoDSN,
		MWSBaseURL: *mwsBaseURL,
		AppID:      appID,
		AppSecret:  appSecret,
		TenantUUID: tenantUUIDFormatted,
		EncryptKey: encryptKey,
	}

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		log.Fatalf("解析配置模板失败: %v", err)
	}

	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			log.Fatalf("创建配置文件失败: %v", err)
		}
		defer f.Close()

		if err := tmpl.Execute(f, cfg); err != nil {
			log.Fatalf("写入配置文件失败: %v", err)
		}
		log.Printf("\n[OK] 配置文件已写入: %s", *output)
	} else {
		log.Println("\n========== 生成的 config.yaml ==========")
		if err := tmpl.Execute(os.Stdout, cfg); err != nil {
			log.Fatalf("输出配置失败: %v", err)
		}
		log.Println("=========================================")
	}

	log.Println("\n[DONE] Setup 完成！")
	log.Println("接下来：")
	log.Println("  1. 确保 Demo App 数据库已创建: mysql -e 'CREATE DATABASE IF NOT EXISTS mws365_demo'")
	log.Println("  2. 执行建表: mysql mws365_demo < schema.sql")
	log.Println("  3. 启动应用: go run main.go")
}

// ==================== 工具函数 ====================

// randomHex 生成指定字节数的随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("生成随机数失败: %v", err)
	}
	return hex.EncodeToString(b)
}

// randomUUID 生成随机 16 字节 UUID（v4 格式）
func randomUUID() []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("生成 UUID 失败: %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return b
}

// formatUUID 将 32 位十六进制字符串格式化为 8-4-4-4-12
func formatUUID(hexStr string) string {
	hexStr = strings.ToLower(strings.ReplaceAll(hexStr, "-", ""))
	if len(hexStr) != 32 {
		return hexStr
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

// columnExists 检查表中是否存在某列
func columnExists(db *sql.DB, table, column string) bool {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
		table, column,
	).Scan(&count)
	return err == nil && count > 0
}

// ==================== 租户操作 ====================

// createNewTenant 创建新租户，返回自增 ID
func createNewTenant(db *sql.DB, tenantUUID []byte, name, domain string, now int64) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO tenants (uuid, name, domain, scale, status, member_capacity, created_at, updated_at) VALUES (?, ?, ?, 1, 1, 1000, ?, ?)",
		tenantUUID, name, domain, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("插入 tenants 失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取新租户 ID 失败: %w", err)
	}
	return id, nil
}

// getTenantUUID 查询租户的 UUID 十六进制字符串
func getTenantUUID(db *sql.DB, tenantID int64) (string, error) {
	var uuidBytes []byte
	err := db.QueryRow("SELECT uuid FROM tenants WHERE id = ? AND deleted_at = 0", tenantID).Scan(&uuidBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("租户 ID=%d 不存在", tenantID)
		}
		return "", err
	}
	return hex.EncodeToString(uuidBytes), nil
}

// ==================== 应用操作 ====================

// appExists 检查 app_id 是否已存在
func appExists(db *sql.DB, appID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM applications WHERE app_id = ? AND deleted_at = 0", appID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// insertRecords 在事务中插入所有记录（动态检测可选列，兼容不同 schema）
func insertRecords(db *sql.DB, appUUID, oauthUUID, tenantAppUUID []byte, appID, appSecret, encryptKey, appName, appBaseURL string, tenantID, now int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 1. 插入 applications
	appCols := "uuid, app_id, union_id, name, app_secret, `desc`, type, scope, status, created_at, updated_at"
	appHolders := "?, ?, 0, ?, ?, 'MWS365 Demo App - 第三方应用对接参考实现', 2, 'openid profile email', 1, ?, ?"
	appArgs := []any{appUUID, appID, appName, appSecret, now, now}

	type optCol struct {
		name string
		val  any
	}
	appOptional := []optCol{
		{"event_url", appBaseURL + "/webhook/events"},
		{"callback_url", appBaseURL + "/webhook/callbacks"},
		{"home_url", appBaseURL},
		{"encrypt_key", encryptKey},
	}
	for _, oc := range appOptional {
		if columnExists(db, "applications", oc.name) {
			appCols += ", " + oc.name
			appHolders += ", ?"
			appArgs = append(appArgs, oc.val)
		} else {
			log.Printf("[DB] applications 表无 %s 列，跳过", oc.name)
		}
	}

	if _, err = tx.Exec(fmt.Sprintf("INSERT INTO applications (%s) VALUES (%s)", appCols, appHolders), appArgs...); err != nil {
		return fmt.Errorf("插入 applications 失败: %w", err)
	}
	log.Printf("[DB] 已插入 applications: app_id=%s", appID)

	// 2. 插入 oauth_clients
	oaCols := "uuid, client_id, tenant_id, name, secret, redirect_uri, is_public, status, created_at, updated_at"
	oaHolders := "?, ?, ?, ?, ?, ?, 0, 1, ?, ?"
	oaArgs := []any{oauthUUID, appID, tenantID, appName, appSecret, appBaseURL + "/oauth2/callback", now, now}

	oaOptional := []optCol{
		{"audience", appBaseURL},
		{"logout_url", appBaseURL + "/oauth2/logout"},
	}
	for _, oc := range oaOptional {
		if columnExists(db, "oauth_clients", oc.name) {
			oaCols += ", " + oc.name
			oaHolders += ", ?"
			oaArgs = append(oaArgs, oc.val)
		} else {
			log.Printf("[DB] oauth_clients 表无 %s 列，跳过", oc.name)
		}
	}

	if _, err = tx.Exec(fmt.Sprintf("INSERT INTO oauth_clients (%s) VALUES (%s)", oaCols, oaHolders), oaArgs...); err != nil {
		return fmt.Errorf("插入 oauth_clients 失败: %w", err)
	}
	log.Printf("[DB] 已插入 oauth_clients: client_id=%s, tenant_id=%d", appID, tenantID)

	// 3. 插入 tenant_applications
	taCols := "uuid, app_id, tenant_id, status, visible_to_all, created_at, updated_at"
	taHolders := "?, ?, ?, 1, 1, ?, ?"
	taArgs := []any{tenantAppUUID, appID, tenantID, now, now}

	if columnExists(db, "tenant_applications", "installed_at") {
		taCols += ", installed_at"
		taHolders += ", ?"
		taArgs = append(taArgs, now)
	}

	if _, err = tx.Exec(fmt.Sprintf("INSERT INTO tenant_applications (%s) VALUES (%s)", taCols, taHolders), taArgs...); err != nil {
		return fmt.Errorf("插入 tenant_applications 失败: %w", err)
	}
	log.Printf("[DB] 已插入 tenant_applications: app_id=%s, tenant_id=%d", appID, tenantID)

	// 4. 插入事件/回调订阅（app_subscriptions）
	eventTypes := []string{
		"contact.user.create", "contact.user.update",
		"contact.group.create", "contact.group.update", "contact.group.delete",
		"contact.group.add_users", "contact.group.remove_users",
		"contact.department.create", "contact.department.update", "contact.department.delete",
		"contact.department.add_users", "contact.department.remove_users",
		"roles.add_users", "roles.remove_users",
		"app.update",
	}
	callbackTypes := []string{
		"notify.button.click",
	}

	subInsert := "INSERT INTO app_subscriptions (uuid, app_id, subscription_type, type_code, status, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, 1, ?, ?, 0)"
	for _, et := range eventTypes {
		if _, err = tx.Exec(subInsert, randomUUID(), appID, "event", et, now, now); err != nil {
			return fmt.Errorf("插入事件订阅 %s 失败: %w", et, err)
		}
	}
	for _, ct := range callbackTypes {
		if _, err = tx.Exec(subInsert, randomUUID(), appID, "callback", ct, now, now); err != nil {
			return fmt.Errorf("插入回调订阅 %s 失败: %w", ct, err)
		}
	}
	log.Printf("[DB] 已插入 %d 条事件订阅 + %d 条回调订阅", len(eventTypes), len(callbackTypes))

	// 5. 为租户用户创建 app_users 映射（union_uid = tenant_uid，保持一致）
	rows, err := tx.Query(
		`SELECT tu.user_id, tu.tenant_uid FROM tenants_users tu
		 WHERE tu.tenant_id = ? AND tu.deleted_at = 0
		 AND tu.user_id NOT IN (SELECT user_id FROM app_users WHERE app_id = ? AND tenant_id = ? AND deleted_at = 0)`,
		tenantID, appID, tenantID)
	if err != nil {
		log.Printf("[DB] 查询租户用户失败（可忽略）: %v", err)
	} else {
		var appUserCount int
		for rows.Next() {
			var userID int64
			var tenantUIDBytes []byte
			if err := rows.Scan(&userID, &tenantUIDBytes); err != nil {
				continue
			}
			_, err := tx.Exec(
				"INSERT INTO app_users (uuid, app_id, tenant_id, user_id, app_uid, union_id, union_uid, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, 0)",
				randomUUID(), appID, tenantID, userID, randomUUID(), tenantUIDBytes, now, now)
			if err != nil {
				log.Printf("[DB] 插入 app_user user_id=%d 失败: %v", userID, err)
				continue
			}
			appUserCount++
		}
		rows.Close()
		log.Printf("[DB] 已插入 %d 条 app_users 映射", appUserCount)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// ==================== Dry-run 输出 ====================

func printDryRunTenantSQL(tenantUUID []byte, name, domain string, now int64) {
	fmt.Printf(`-- 0. 创建租户
INSERT INTO tenants (uuid, name, domain, scale, status, member_capacity, created_at, updated_at)
VALUES (0x%s, '%s', '%s', 1, 1, 1000, %d, %d);
-- 后续 SQL 中 tenant_id 使用 LAST_INSERT_ID()

`,
		hex.EncodeToString(tenantUUID), name, domain, now, now,
	)
}

func printDryRunSQL(appUUID, oauthUUID, tenantAppUUID []byte, appID, appSecret, encryptKey, appName, appBaseURL string, tenantID, now int64) {
	tidStr := fmt.Sprintf("%d", tenantID)
	if tenantID <= 0 {
		tidStr = "LAST_INSERT_ID()"
	}
	fmt.Printf(`-- 1. 创建应用
INSERT INTO applications (uuid, app_id, union_id, name, app_secret, `+"`desc`"+`, type, scope, status, created_at, updated_at)
VALUES (0x%s, '%s', 0, '%s', '%s', 'MWS365 Demo App - 第三方应用对接参考实现', 2, 'openid profile email', 1, %d, %d);
-- 注意：如果表有 event_url/callback_url/home_url/encrypt_key 列，需额外添加

-- 2. 创建 OAuth2 客户端
INSERT INTO oauth_clients (uuid, client_id, tenant_id, name, secret, redirect_uri, is_public, status, created_at, updated_at)
VALUES (0x%s, '%s', %s, '%s', '%s', '%s/oauth2/callback', 0, 1, %d, %d);

-- 3. 安装应用到租户
INSERT INTO tenant_applications (uuid, app_id, tenant_id, status, visible_to_all, created_at, updated_at)
VALUES (0x%s, '%s', %s, 1, 1, %d, %d);

-- 4. 订阅事件和回调
`,
		hex.EncodeToString(appUUID), appID, appName, appSecret, now, now,
		hex.EncodeToString(oauthUUID), appID, tidStr, appName, appSecret, appBaseURL, now, now,
		hex.EncodeToString(tenantAppUUID), appID, tidStr, now, now,
	)

	subTypes := []struct{ stype, code string }{
		{"event", "contact.user.create"}, {"event", "contact.user.update"},
		{"event", "contact.group.create"}, {"event", "contact.group.update"}, {"event", "contact.group.delete"},
		{"event", "contact.group.add_users"}, {"event", "contact.group.remove_users"},
		{"event", "contact.department.create"}, {"event", "contact.department.update"}, {"event", "contact.department.delete"},
		{"event", "contact.department.add_users"}, {"event", "contact.department.remove_users"},
		{"event", "roles.add_users"}, {"event", "roles.remove_users"},
		{"event", "app.update"},
		{"callback", "notify.button.click"},
	}
	for _, s := range subTypes {
		fmt.Printf("INSERT INTO app_subscriptions (uuid, app_id, subscription_type, type_code, status, created_at, updated_at, deleted_at) VALUES (UNHEX(REPLACE(UUID(),'-','')), '%s', '%s', '%s', 1, %d, %d, 0);\n",
			appID, s.stype, s.code, now, now)
	}
	fmt.Println()
}

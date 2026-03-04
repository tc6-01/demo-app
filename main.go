package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"

	"mws365-demo-app/client"
	"mws365-demo-app/handler"
	"mws365-demo-app/model"
	"mws365-demo-app/store"
	contactSync "mws365-demo-app/sync"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	doSync := flag.Bool("sync", false, "启动时执行用户全量同步（仅 local 模式）")
	flag.Parse()

	// 加载配置
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("用户模式: %s", cfg.App.UserMode)
	log.Printf("MWS 服务地址: %s", cfg.MWS.BaseURL)

	// 初始化数据库
	if err := store.InitDB(cfg.MySQL.DSN); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer store.CloseDB()

	// 初始化客户端
	apiClient := client.NewOpenAPIClient(cfg)
	oauth2Client := client.NewOAuth2Client(cfg)

	// 初始化 Session 存储
	sessions := handler.NewSessionStore()

	// 初始化处理器
	oauthHandler := handler.NewOAuthHandler(oauth2Client, sessions, cfg.Server.BaseURL)
	webhookHandler := handler.NewWebhookHandler(cfg.OpenAPI.EncryptKey, cfg.App.UserMode)
	openapiHandler := handler.NewOpenAPIHandler(apiClient, cfg.App.UserMode)
	pageHandler := handler.NewPageHandler(oauthHandler, cfg.App.UserMode)

	// 启动时全量同步（可选）
	if *doSync && cfg.App.UserMode == "local" {
		go func() {
			if err := contactSync.SyncAllUsers(apiClient); err != nil {
				log.Printf("启动同步失败: %v", err)
			}
		}()
	}

	// 注册路由
	mux := http.NewServeMux()

	// 页面
	mux.HandleFunc("/", pageHandler.HandleIndex)
	mux.HandleFunc("/dashboard", pageHandler.HandleDashboard)

	// OAuth2
	mux.HandleFunc("/oauth2/login", oauthHandler.HandleLogin)
	mux.HandleFunc("/oauth2/callback", oauthHandler.HandleCallback)
	mux.HandleFunc("/oauth2/logout", oauthHandler.HandleLogout)
	mux.HandleFunc("/oauth2/refresh", oauthHandler.HandleRefresh)

	// Webhook
	mux.HandleFunc("/webhook/events", webhookHandler.HandleEvents)
	mux.HandleFunc("/webhook/callbacks", webhookHandler.HandleCallbacks)

	// API - 认证
	mux.HandleFunc("/api/tenant-token", openapiHandler.HandleGetToken)
	
	// API - 通讯录
	mux.HandleFunc("/api/users", openapiHandler.HandleGetUsers)
	mux.HandleFunc("/api/departments", openapiHandler.HandleGetDepartments)
	mux.HandleFunc("/api/groups", openapiHandler.HandleGetGroups)
	mux.HandleFunc("/api/group-users", openapiHandler.HandleGetGroupUsers)
	mux.HandleFunc("/api/role-members", openapiHandler.HandleGetRoleMembers)
	
	// API - 全量接口（大分页）
	mux.HandleFunc("/api/all-users", openapiHandler.HandleGetAllUsers)
	mux.HandleFunc("/api/all-departments", openapiHandler.HandleGetAllDepartments)
	
	// API - 应用与租户
	mux.HandleFunc("/api/tenant-info", openapiHandler.HandleGetTenantInfo)
	mux.HandleFunc("/api/visibility-users", openapiHandler.HandleGetVisibilityUsers)
	
	// API - 同步与事件
	mux.HandleFunc("/api/sync", openapiHandler.HandleSync)
	mux.HandleFunc("/api/events", openapiHandler.HandleListEvents)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("MWS365 Demo App 启动: http://localhost%s", addr)
	log.Printf("  OAuth2 回调地址: %s/oauth2/callback", cfg.Server.BaseURL)
	log.Printf("  事件 Webhook:    %s/webhook/events", cfg.Server.BaseURL)
	log.Printf("  回调 Webhook:    %s/webhook/callbacks", cfg.Server.BaseURL)

	if err := http.ListenAndServe(addr, logMiddleware(mux)); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// loadConfig 加载 YAML 配置文件
func loadConfig(path string) (*model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.App.UserMode == "" {
		cfg.App.UserMode = "mws"
	}

	return &cfg, nil
}

// logMiddleware 请求日志中间件
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

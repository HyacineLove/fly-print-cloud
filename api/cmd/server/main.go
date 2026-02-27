package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/handlers"
	"fly-print-cloud/api/internal/middleware"
	"fly-print-cloud/api/internal/security"
	"fly-print-cloud/api/internal/websocket"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 设置Gin模式
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// 连接数据库
	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// 初始化数据库表
	if err := db.InitTables(); err != nil {
		log.Fatal("Failed to initialize database tables:", err)
	}



	// 初始化服务
	userRepo := database.NewUserRepository(db)
	edgeNodeRepo := database.NewEdgeNodeRepository(db)
	printerRepo := database.NewPrinterRepository(db)
	printJobRepo := database.NewPrintJobRepository(db)
	fileRepo := database.NewFileRepository(db)
	tokenUsageRepo := database.NewTokenUsageRepository(db)

	// 初始化凭证管理器（支持一次性凭证验证）
	tokenManager := security.NewTokenManager(
		cfg.Security.FileAccessSecret,
		cfg.Security.UploadTokenTTL,
		cfg.Security.DownloadTokenTTL,
		tokenUsageRepo,
	)

	// 启动Token使用记录清理任务（每小时清理过期记录）
	go startTokenCleanupTask(tokenUsageRepo)
	// 启动文件清理任务（定期删除1天前的文件）
	go startFileCleanupTask(fileRepo, cfg.Storage.UploadDir)

	// 初始化 WebSocket 管理器
	wsManager := websocket.NewConnectionManager(tokenManager)
	wsHandler := websocket.NewWebSocketHandler(wsManager, printerRepo, edgeNodeRepo, printJobRepo, fileRepo, tokenManager)

	// 初始化处理器
	userHandler := handlers.NewUserHandler(userRepo)
	edgeNodeHandler := handlers.NewEdgeNodeHandler(edgeNodeRepo, printerRepo, wsManager, tokenUsageRepo)
	printerHandler := handlers.NewPrinterHandler(printerRepo, edgeNodeRepo, wsManager, tokenUsageRepo)
	printJobHandler := handlers.NewPrintJobHandler(printJobRepo, printerRepo, edgeNodeRepo, wsManager)
	oauth2Handler := handlers.NewOAuth2Handler(&cfg.OAuth2, &cfg.Admin, userRepo)
	fileHandler := handlers.NewFileHandler(fileRepo, &cfg.Storage, wsManager, tokenManager)

	// 启动 WebSocket 管理器
	go wsManager.Run()

	// 创建Gin路由
	r := gin.New()

	// 添加中间件
	r.Use(middleware.LoggerMiddleware())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	// 设置路由
	setupRoutes(r, userHandler, edgeNodeHandler, printerHandler, printJobHandler, wsHandler, oauth2Handler, fileHandler, printJobRepo, edgeNodeRepo, printerRepo)

	// 启动服务器
	serverAddr := cfg.Server.GetServerAddr()
	log.Printf("Starting %s server on %s", cfg.App.Name, serverAddr)
	log.Printf("Environment: %s, Debug: %v", cfg.App.Environment, cfg.App.Debug)
	
	if err := r.Run(serverAddr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func setupRoutes(r *gin.Engine, userHandler *handlers.UserHandler, edgeNodeHandler *handlers.EdgeNodeHandler, printerHandler *handlers.PrinterHandler, printJobHandler *handlers.PrintJobHandler, wsHandler *websocket.WebSocketHandler, oauth2Handler *handlers.OAuth2Handler, fileHandler *handlers.FileHandler, printJobRepo *database.PrintJobRepository, edgeNodeRepo *database.EdgeNodeRepository, printerRepo *database.PrinterRepository) {
	// 公开路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "success",
			"data": gin.H{
				"status":  "ok",
				"service": "fly-print-cloud-api",
			},
		})
	})

	// OAuth2 认证路由
	authGroup := r.Group("/auth")
	{
		authGroup.GET("/login", oauth2Handler.Login)
		authGroup.GET("/callback", oauth2Handler.Callback)
		authGroup.GET("/me", oauth2Handler.Me)
		authGroup.GET("/verify", oauth2Handler.Verify)  // Nginx auth_request 使用
		authGroup.GET("/logout", oauth2Handler.Logout)   // 支持 GET 请求登出
		authGroup.POST("/logout", oauth2Handler.Logout)  // 保留 POST 支持
	}

	// 统一 API 路由组（/api/v1）- OAuth2 Resource Server
	apiV1Group := r.Group("/api/v1")
	{
		apiV1Group.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code":    http.StatusOK,
				"message": "success",
				"data": gin.H{
					"status":  "ok",
					"service": "fly-print-cloud-api",
					"version": "1.0.0",
				},
			})
		})

		// Admin Console API - 需要 admin:* scope
		adminGroup := apiV1Group.Group("/admin")
		{
			// Dashboard 路由 - 需要 admin 或 operator 权限
			dashboardGroup := adminGroup.Group("/dashboard", middleware.OAuth2ResourceServer("fly-print-admin", "fly-print-operator"))
			{
				dashboardHandler := handlers.NewDashboardHandler(printJobRepo)
				dashboardGroup.GET("/trends", dashboardHandler.GetTrends)
			}

			// 用户管理路由 - 需要 admin 权限
			userGroup := adminGroup.Group("/users", middleware.OAuth2ResourceServer("fly-print-admin"))
			{
				userGroup.GET("", userHandler.ListUsers)
				userGroup.POST("", userHandler.CreateUser)
				userGroup.GET("/:id", userHandler.GetUser)
				userGroup.PUT("/:id", userHandler.UpdateUser)
				userGroup.DELETE("/:id", userHandler.DeleteUser)
				userGroup.PUT("/:id/password", userHandler.ChangePassword)
			}
			
			// 当前用户业务信息 - 任何认证用户都可以访问自己的档案
			adminGroup.GET("/profile", middleware.OAuth2ResourceServer(), userHandler.GetCurrentUserProfile)

			// Edge Node 管理路由 - 需要 admin 或 operator 权限
			edgeNodeGroup := adminGroup.Group("/edge-nodes", middleware.OAuth2ResourceServer("fly-print-admin", "fly-print-operator"))
			{
				edgeNodeGroup.GET("", edgeNodeHandler.ListEdgeNodes)
				edgeNodeGroup.GET("/:id", edgeNodeHandler.GetEdgeNode)
				edgeNodeGroup.PUT("/:id", edgeNodeHandler.UpdateEdgeNode)
				edgeNodeGroup.DELETE("/:id", edgeNodeHandler.DeleteEdgeNode)
			}

			// 打印机管理路由 - 需要 admin 或 operator 权限
			printerGroup := adminGroup.Group("/printers", middleware.OAuth2ResourceServer("fly-print-admin", "fly-print-operator"))
			{
				printerGroup.GET("", printerHandler.ListPrinters)
				printerGroup.GET("/:id", printerHandler.GetPrinter)
				printerGroup.PUT("/:id", printerHandler.UpdatePrinter)
				printerGroup.DELETE("/:id", printerHandler.DeletePrinter)
			}

			// 打印任务管理路由 - 需要 admin 或 operator 权限
			// 注意：管理后台不再支持创建、删除、取消、重新打印任务
			// 这些操作仅通过第三方 API 或 Edge 端完成
			printJobGroup := adminGroup.Group("/print-jobs", middleware.OAuth2ResourceServer("fly-print-admin", "fly-print-operator"))
			{
				printJobGroup.GET("", printJobHandler.ListPrintJobs)
				printJobGroup.GET("/:id", printJobHandler.GetPrintJob)
				printJobGroup.PUT("/:id", printJobHandler.UpdatePrintJob)
			}
		}

		// 第三方打印API - 需要 print:submit 权限
		printGroup := apiV1Group.Group("/print-jobs", middleware.OAuth2ResourceServer("print:submit"))
		{
			printGroup.POST("", printJobHandler.CreatePrintJob)
			printGroup.GET("/:id", printJobHandler.GetPrintJob)
		}

		// 第三方打印机列表API - 需要 print:submit 权限
		apiV1Group.GET("/printers", middleware.OAuth2ResourceServer("print:submit"), printerHandler.ListPrinters)

		// Edge Node API - 需要 edge:* scope
		edgeGroup := apiV1Group.Group("/edge")
		{
			edgeGroup.POST("/register", middleware.OAuth2ResourceServer("edge:register"), edgeNodeHandler.RegisterEdgeNode)
			// HTTP 心跳 API 已删除，改为通过 WebSocket 进行心跳
			
			// Edge Node 的打印机管理 - 添加节点禁用检查
			edgeGroup.POST("/:node_id/printers", middleware.OAuth2ResourceServer("edge:printer"), middleware.EdgeNodeEnabledCheck(edgeNodeRepo), printerHandler.EdgeRegisterPrinter)
			// 删除打印机：启用的节点可以管理自己的所有打印机（包括禁用的）
			edgeGroup.DELETE("/:node_id/printers/:printer_id", middleware.OAuth2ResourceServer("edge:printer"), middleware.EdgeNodeEnabledCheck(edgeNodeRepo), printerHandler.EdgeDeletePrinter)
			
			// 功能 3.2.2: 批量状态上报 API - 从 WebSocket 迁移到 REST API
			// 功能 3.2.3: 放行禁用打印机的状态上报请求，仅检查节点启用状态
			// 放行禁用节点的批量状态上报，允许监控禁用节点的打印机状态
			edgeGroup.POST("/:node_id/printers/status", middleware.OAuth2ResourceServer("edge:printer"), printerHandler.EdgeBatchUpdatePrinterStatus)
			
			// WebSocket 连接
			edgeGroup.GET("/ws", wsHandler.HandleConnection)
		}

		// 文件上传/下载 - 支持凭证认证或 OAuth2 认证
		fileGroup := apiV1Group.Group("/files")
		{
			// 轻量验证上传Token（不消耗一次性Token）
			fileGroup.GET("/verify-upload-token", fileHandler.VerifyUploadToken)
			// 上传：支持上传凭证或 OAuth2 认证
			fileGroup.POST("", middleware.OptionalOAuth2ResourceServer(), fileHandler.Upload)
			// 下载：支持下载凭证或 OAuth2 认证
			fileGroup.GET("/:id", middleware.OptionalOAuth2ResourceServer(), fileHandler.Download)
		}
	}
}

// startFileCleanupTask 启动文件清理任务
// 每小时扫描一次，删除创建时间超过1天的文件记录和物理文件
func startFileCleanupTask(fileRepo *database.FileRepository, uploadDir string) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-24 * time.Hour)
		files, err := fileRepo.ListOldFiles(cutoff)
		if err != nil {
			log.Printf("File cleanup: failed to list old files: %v", err)
			continue
		}
		if len(files) == 0 {
			continue
		}

		deletedCount := 0
		for _, f := range files {
			// 尝试删除物理文件
			if err := os.Remove(f.FilePath); err != nil && !os.IsNotExist(err) {
				log.Printf("File cleanup: failed to remove file %s: %v", f.FilePath, err)
				continue
			}

			// 删除数据库记录
			if err := fileRepo.DeleteByID(f.ID); err != nil {
				log.Printf("File cleanup: failed to delete db record %s: %v", f.ID, err)
				continue
			}
			deletedCount++
		}

		if deletedCount > 0 {
			log.Printf("File cleanup: deleted %d files older than 24h", deletedCount)
		}
	}
}

// startTokenCleanupTask 启动Token使用记录清理任务
// 每小时清理一次过期的token记录，防止数据库表膨胀
func startTokenCleanupTask(tokenUsageRepo *database.TokenUsageRepository) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		deleted, err := tokenUsageRepo.CleanupExpiredTokens(time.Now())
		if err != nil {
			log.Printf("Token cleanup error: %v", err)
		} else if deleted > 0 {
			log.Printf("Cleaned up %d expired token records", deleted)
		}
	}
}

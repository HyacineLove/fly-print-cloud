package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fly-print-cloud/api/internal/auth"
	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/handlers"
	"fly-print-cloud/api/internal/logger"
	"fly-print-cloud/api/internal/middleware"
	"fly-print-cloud/api/internal/security"
	"fly-print-cloud/api/internal/websocket"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"go.uber.org/zap"

	_ "fly-print-cloud/api/docs" // Swagger 生成的文档
)

// @title           Fly Print Cloud API
// @version         1.0
// @description     云打印系统后端API服务，提供打印任务管理、边缘节点管理、OAuth2认证等功能
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@flyprint.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// 初始化日志系统
	if err := logger.Init(cfg.App.Debug); err != nil {
		logger.Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer logger.Sync()

	logger.Info("Starting application",
		zap.String("name", cfg.App.Name),
		zap.String("version", cfg.App.Version),
		zap.String("environment", cfg.App.Environment),
		zap.Bool("debug", cfg.App.Debug),
	)

	// 设置Gin模式
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// 连接数据库
	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	logger.Info("Database connected successfully")

	// 初始化数据库表
	if err := db.InitTables(); err != nil {
		logger.Fatal("Failed to initialize database tables", zap.Error(err))
	}

	// 创建默认管理员账户（如果配置了）
	if err := db.CreateDefaultAdmin(); err != nil {
		logger.Warn("Failed to create default admin", zap.Error(err))
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
	// 启动打印任务状态清理任务（每30分钟检查一次超时任务）
	go startStaleJobCleanupTask(printJobRepo)

	// 初始化 WebSocket 管理器
	wsManager := websocket.NewConnectionManager(tokenManager)
	wsHandler := websocket.NewWebSocketHandler(wsManager, printerRepo, edgeNodeRepo, printJobRepo, fileRepo, tokenManager, cfg.Server.AllowedOrigins)

	// 启动pending任务重试机制（每5分钟检查一次超过3分钟的pending任务）
	go startPendingJobRetryTask(printJobRepo, printerRepo, wsManager)

	// 初始化内置认证服务（builtin 模式）
	var builtinAuth *auth.BuiltinAuthService
	var oauth2ClientRepo *database.OAuth2ClientRepository
	if cfg.OAuth2.IsBuiltinMode() {
		oauth2ClientRepo = database.NewOAuth2ClientRepository(db)
		builtinAuth = auth.NewBuiltinAuthService(oauth2ClientRepo, userRepo, &cfg.OAuth2)
		logger.Info("OAuth2 mode: builtin (embedded auth service)")

		// 创建默认 Edge OAuth2 客户端（首次启动）
		if err := db.CreateDefaultOAuth2Client(); err != nil {
			logger.Warn("Failed to create default OAuth2 client", zap.Error(err))
		}
	} else {
		logger.Info("OAuth2 mode: keycloak (external identity provider)")
	}

	// 初始化处理器
	userHandler := handlers.NewUserHandler(userRepo)
	edgeNodeHandler := handlers.NewEdgeNodeHandler(db, edgeNodeRepo, printerRepo, wsManager, tokenUsageRepo)
	printerHandler := handlers.NewPrinterHandler(printerRepo, edgeNodeRepo, printJobRepo, wsManager, tokenUsageRepo)
	printJobHandler := handlers.NewPrintJobHandler(printJobRepo, printerRepo, edgeNodeRepo, wsManager)
	oauth2Handler := handlers.NewOAuth2Handler(&cfg.OAuth2, &cfg.Admin, userRepo, builtinAuth)
	fileHandler := handlers.NewFileHandler(fileRepo, &cfg.Storage, wsManager, tokenManager)
	healthHandler := handlers.NewHealthHandler(db, wsManager)

	// 启动 WebSocket 管理器
	go wsManager.Run()

	// 创建Gin路由
	r := gin.New()

	// Rate Limiting (10 req/s)
	rate := limiter.Rate{
		Period: 1 * time.Second,
		Limit:  10,
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	r.Use(mgin.NewMiddleware(instance))

	// 添加中间件
	r.Use(middleware.LoggerMiddleware())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(cfg.Server.AllowedOrigins))
	r.Use(middleware.SecurityHeadersMiddleware())

	// 设置路由
	setupRoutes(r, userHandler, edgeNodeHandler, printerHandler, printJobHandler, wsHandler, oauth2Handler, fileHandler, healthHandler, printJobRepo, edgeNodeRepo, printerRepo, oauth2ClientRepo)

	// 创建HTTP服务器
	serverAddr := cfg.Server.GetServerAddr()
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	// 启动服务器（在goroutine中）
	go func() {
		logger.Info("Server starting", zap.String("address", serverAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// 等待中断信号以优雅关机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 设置5秒超时的context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 优雅关闭服务器
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func setupRoutes(r *gin.Engine, userHandler *handlers.UserHandler, edgeNodeHandler *handlers.EdgeNodeHandler, printerHandler *handlers.PrinterHandler, printJobHandler *handlers.PrintJobHandler, wsHandler *websocket.WebSocketHandler, oauth2Handler *handlers.OAuth2Handler, fileHandler *handlers.FileHandler, healthHandler *handlers.HealthHandler, printJobRepo *database.PrintJobRepository, edgeNodeRepo *database.EdgeNodeRepository, printerRepo *database.PrinterRepository, oauth2ClientRepo *database.OAuth2ClientRepository) {
	// Swagger 文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 公开健康检查路由（快速响应）
	r.GET("/health", healthHandler.BasicHealth)
	r.HEAD("/health", healthHandler.BasicHealth)

	// OAuth2 认证路由
	authGroup := r.Group("/auth")
	{
		authGroup.GET("/mode", oauth2Handler.Mode)         // 返回当前认证模式（公开）
		authGroup.POST("/token", oauth2Handler.Token)      // Token 端点（builtin 模式）
		authGroup.GET("/userinfo", oauth2Handler.UserInfo) // UserInfo 端点
		authGroup.GET("/login", oauth2Handler.Login)
		authGroup.GET("/callback", oauth2Handler.Callback)
		authGroup.GET("/me", oauth2Handler.Me)
		authGroup.GET("/verify", oauth2Handler.Verify)  // Nginx auth_request 使用
		authGroup.GET("/logout", oauth2Handler.Logout)  // 支持 GET 请求登出
		authGroup.POST("/logout", oauth2Handler.Logout) // 保留 POST 支持
	}

	// 统一 API 路由组（/api/v1）- OAuth2 Resource Server
	apiV1Group := r.Group("/api/v1")
	{
		// 详细健康检查（包含各组件状态）
		apiV1Group.GET("/health", healthHandler.DetailedHealth)
		apiV1Group.HEAD("/health", healthHandler.DetailedHealth)

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
			// 注意：管理后台不再支持创建任务，仅支持查询、更新、取消和删除
			// 创建任务通过第三方 API 或 Edge 端完成
			printJobGroup := adminGroup.Group("/print-jobs", middleware.OAuth2ResourceServer("fly-print-admin", "fly-print-operator"))
			{
				printJobGroup.GET("", printJobHandler.ListPrintJobs)
				printJobGroup.GET("/:id", printJobHandler.GetPrintJob)
				printJobGroup.PUT("/:id", printJobHandler.UpdatePrintJob)
				printJobGroup.POST("/:id/cancel", printJobHandler.CancelPrintJob) // 取消任务
				printJobGroup.DELETE("/:id", printJobHandler.DeletePrintJob)      // 删除历史任务
			}

			// OAuth2 客户端管理路由（仅 builtin 模式可用）- 需要 admin 权限
			if oauth2ClientRepo != nil {
				oauth2ClientGroup := adminGroup.Group("/oauth2-clients", middleware.OAuth2ResourceServer("fly-print-admin"))
				{
					oauth2ClientHandler := handlers.NewOAuth2ClientHandler(oauth2ClientRepo)
					oauth2ClientGroup.GET("", oauth2ClientHandler.List)
					oauth2ClientGroup.POST("", oauth2ClientHandler.Create)
					oauth2ClientGroup.GET("/:id", oauth2ClientHandler.Get)
					oauth2ClientGroup.PUT("/:id", oauth2ClientHandler.Update)
					oauth2ClientGroup.PUT("/:id/secret", oauth2ClientHandler.ResetSecret)
					oauth2ClientGroup.DELETE("/:id", oauth2ClientHandler.Delete)
				}
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
			fileGroup.POST("/preflight", middleware.OptionalOAuth2ResourceServer(), fileHandler.PreflightUpload)
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
			logger.Error("File cleanup: failed to list old files", zap.Error(err))
			continue
		}
		if len(files) == 0 {
			continue
		}

		deletedCount := 0
		for _, f := range files {
			// 尝试删除物理文件
			if err := os.Remove(f.FilePath); err != nil && !os.IsNotExist(err) {
				logger.Warn("File cleanup: failed to remove file", zap.String("path", f.FilePath), zap.Error(err))
				continue
			}

			// 删除数据库记录
			if err := fileRepo.DeleteByID(f.ID); err != nil {
				logger.Warn("File cleanup: failed to delete db record", zap.String("id", f.ID), zap.Error(err))
				continue
			}
			deletedCount++
		}

		if deletedCount > 0 {
			logger.Info("File cleanup completed", zap.Int("deleted_count", deletedCount))
		}
	}
}

// startStaleJobCleanupTask 启动打印任务状态清理任务
// 每30分钟扫描一次，将超过30分钟未更新的“打印中”任务标记为失败
func startStaleJobCleanupTask(printJobRepo *database.PrintJobRepository) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// 30分钟超时
		timeout := 30 * time.Minute
		affected, err := printJobRepo.CleanupStaleJobs(timeout)
		if err != nil {
			logger.Error("Stale job cleanup error", zap.Error(err))
			continue
		}

		if affected > 0 {
			logger.Info("Stale job cleanup completed", zap.Int64("affected", affected))
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
			logger.Error("Token cleanup error", zap.Error(err))
		} else if deleted > 0 {
			logger.Info("Token cleanup completed", zap.Int64("deleted", deleted))
		}
	}
}

// startPendingJobRetryTask 启动pending任务重试机制
// 每5分钟检查一次创建时间超过3分钟的pending任务，并尝试重新分发
func startPendingJobRetryTask(printJobRepo *database.PrintJobRepository, printerRepo *database.PrinterRepository, wsManager *websocket.ConnectionManager) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger.Info("Pending job retry task started (checking every 5 minutes)")

	for range ticker.C {
		// 查询创建时间超过3分钟的pending任务
		jobs, err := printJobRepo.GetPendingJobsForRetry(3 * time.Minute)
		if err != nil {
			logger.Error("Failed to fetch pending jobs for retry", zap.Error(err))
			continue
		}

		if len(jobs) == 0 {
			continue
		}

		logger.Info("Found pending jobs for retry", zap.Int("count", len(jobs)))

		for _, job := range jobs {
			// 获取打印机信息
			printer, err := printerRepo.GetPrinterByID(job.PrinterID)
			if err != nil {
				logger.Error("Failed to get printer for retry", zap.String("job_id", job.ID), zap.Error(err))
				continue
			}

			if printer == nil {
				logger.Warn("Printer not found for pending job", zap.String("job_id", job.ID), zap.String("printer_id", job.PrinterID))
				// 打印机已删除，将任务标记为失败
				job.Status = "failed"
				job.ErrorMessage = "Printer not found"
				now := time.Now()
				job.EndTime = &now
				if updateErr := printJobRepo.UpdatePrintJob(job); updateErr != nil {
					logger.Error("Failed to mark job as failed", zap.String("job_id", job.ID), zap.Error(updateErr))
				}
				continue
			}

			// 检查节点和打印机是否可用
			if !printer.Enabled {
				logger.Debug("Printer disabled, skip retry", zap.String("job_id", job.ID), zap.String("printer_id", printer.ID))
				continue
			}

			// 检查节点是否在线
			if !wsManager.IsNodeConnected(printer.EdgeNodeID) {
				logger.Debug("Edge node not connected, skip retry", zap.String("job_id", job.ID), zap.String("node_id", printer.EdgeNodeID))
				continue
			}

			// 检查重试次数是否超过限制
			if job.RetryCount >= job.MaxRetries {
				logger.Warn("Job exceeded max retries, marking as failed",
					zap.String("job_id", job.ID),
					zap.Int("retry_count", job.RetryCount),
					zap.Int("max_retries", job.MaxRetries))

				// 标记任务为失败
				job.Status = "failed"
				job.ErrorMessage = fmt.Sprintf("Exceeded max retries (%d/%d)", job.RetryCount, job.MaxRetries)
				now := time.Now()
				job.EndTime = &now

				if updateErr := printJobRepo.UpdatePrintJob(job); updateErr != nil {
					logger.Error("Failed to mark job as failed", zap.String("job_id", job.ID), zap.Error(updateErr))
				}
				continue
			}

			// 尝试重新分发任务
			logger.Info("Retrying to dispatch pending job",
				zap.String("job_id", job.ID),
				zap.String("node_id", printer.EdgeNodeID),
				zap.Int("retry_count", job.RetryCount))

			// 增加重试计数
			job.RetryCount++

			err = wsManager.DispatchPrintJob(printer.EdgeNodeID, job, printer.Name)
			if err != nil {
				logger.Warn("Failed to retry dispatch job", zap.String("job_id", job.ID), zap.Error(err))
				// 分发失败，更新retry_count后保持pending状态，下次继续重试
				if updateErr := printJobRepo.UpdatePrintJob(job); updateErr != nil {
					logger.Error("Failed to update retry count", zap.String("job_id", job.ID), zap.Error(updateErr))
				}
			} else {
				// 分发成功，更新状态为dispatched
				job.Status = "dispatched"
				if updateErr := printJobRepo.UpdatePrintJob(job); updateErr != nil {
					logger.Error("Failed to update job status to dispatched", zap.String("job_id", job.ID), zap.Error(updateErr))
				} else {
					logger.Info("Successfully retried dispatch job", zap.String("job_id", job.ID))
				}
			}
		}
	}
}

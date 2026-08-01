package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/rocuae/importsshkey/go-server/internal/config"
	"github.com/rocuae/importsshkey/go-server/internal/handler"
	"github.com/rocuae/importsshkey/go-server/internal/middleware"
	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化仓储
	userRepo := repository.NewGormUserRepository(db)
	auditLogRepo := repository.NewGormAuditLogRepository(db)

	// 初始化处理器
	keyHandler := handler.NewKeyHandler(userRepo, auditLogRepo)
	healthHandler := handler.NewHealthHandler()
	pageHandler := handler.NewPageHandler(userRepo)

	// 创建 Gin 引擎
	r := gin.Default()

	// 中间件
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.Logger())

	// 主页面和统计接口（公开）
	r.GET("/", pageHandler.Page)
	r.GET("/stats", pageHandler.StatsAPI)

	// 健康检查
	r.GET("/health", healthHandler.Health)

	// 公开路由（需要认证）
	r.GET("/keys/:username", middleware.AdminAuth(cfg), keyHandler.GetKey)
	r.GET("/keys/:username/metadata", middleware.AdminAuth(cfg), keyHandler.GetKeyMetadata)

	// 管理路由（需要 Bearer Token 认证）
	admin := r.Group("/keys")
	admin.Use(middleware.AdminAuth(cfg))
	{
		admin.PUT("/:username", keyHandler.PutKey)
		admin.DELETE("/:username", keyHandler.DeleteKey)
		admin.GET("", keyHandler.ListKeys)
	}

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	log.Printf("Starting iskey-server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initDB 初始化数据库连接
// 参数：
//   - cfg: 应用配置
// 返回：
//   - *gorm.DB: 数据库实例
//   - error: 连接错误
func initDB(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.DBDriver {
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DSN)
	default:
		dialector = sqlite.Open(cfg.DSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

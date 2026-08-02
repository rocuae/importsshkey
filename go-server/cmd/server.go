package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/rocuae/importsshkey/go-server/internal/config"
	"github.com/rocuae/importsshkey/go-server/internal/handler"
	"github.com/rocuae/importsshkey/go-server/internal/middleware"
	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

var foreground bool
var configFile string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 SSH 密钥管理服务器",
	Long:  `启动 SSH 密钥管理服务器，默认以守护进程方式运行，使用 -f 参数在前台运行。`,
	Run: func(cmd *cobra.Command, args []string) {
		if foreground {
			runServer()
		} else {
			startDaemon()
		}
	},
}

func init() {
	serverCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "在前台运行服务器（默认以守护进程方式运行）")
	serverCmd.Flags().StringVarP(&configFile, "config", "c", "", "指定配置文件路径（默认为 config.yaml）")
}

func startDaemon() {
	// 获取可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("无法获取可执行文件路径: %v", err)
	}

	// 构建参数
	args := []string{"server", "-f"}
	if configFile != "" {
		args = append(args, "-c", configFile)
	}

	// 启动守护进程
	cmd := exec.Command(execPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// 分离进程
	if err := cmd.Start(); err != nil {
		log.Fatalf("无法启动守护进程: %v", err)
	}

	fmt.Printf("服务器已在后台启动，PID: %d\n", cmd.Process.Pid)
	os.Exit(0)
}

func runServer() {
	// 加载配置
	cfg := config.Load(configFile)

	// 初始化数据库
	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}); err != nil {
		log.Fatalf("无法迁移数据库: %v", err)
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

	// 公开路由（无需认证，SSH 需要读取公钥）
	r.GET("/keys/:username", keyHandler.GetKey)
	r.GET("/keys/:username/metadata", keyHandler.GetKeyMetadata)

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
	log.Printf("启动 iskey-server 于 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("无法启动服务器: %v", err)
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
		dialector = sqlite.Open(cfg.DSN())
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DSN())
	default:
		dialector = sqlite.Open(cfg.DSN())
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
// 本文件实现 server 子命令：初始化数据库、注册路由并以守护进程或前台方式运行。
package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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
	"github.com/rocuae/importsshkey/go-server/pkg/validator"
)

// foreground 为 true 时在前台运行，否则 fork 自身作为守护进程。
var foreground bool

// configFile 显式指定的配置文件路径；为空时使用默认 config.yaml。
var configFile string

// serverCmd 启动 HTTP 服务。
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

// init 注册 server 子命令的前台与配置文件参数。
func init() {
	serverCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "在前台运行服务器（默认以守护进程方式运行）")
	serverCmd.Flags().StringVarP(&configFile, "config", "c", "", "指定配置文件路径（默认为 config.yaml）")
}

// startDaemon 以自身二进制重新执行 server -f，并把日志写入 iskey-server.log。
func startDaemon() {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("无法获取可执行文件路径: %v", err)
	}

	args := []string{"server", "-f"}
	if configFile != "" {
		args = append(args, "-c", configFile)
	}

	logFile, err := os.OpenFile("iskey-server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(execPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		log.Fatalf("无法启动守护进程: %v", err)
	}

	fmt.Printf("服务器已在后台启动，PID: %d\n", cmd.Process.Pid)
	os.Exit(0)
}

// runServer 加载配置、迁移数据、注册路由并等待退出信号。
func runServer() {
	cfg, err := config.Load(configFile)
	if err != nil {
		log.Fatalf("无法加载配置: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}

	// 新表结构与旧 users 表的迁移都必须在开始服务前完成。
	if err := db.AutoMigrate(&model.UserKey{}, &model.AuditLog{}); err != nil {
		log.Fatalf("无法迁移数据库: %v", err)
	}
	if err := migrateLegacyUsers(db); err != nil {
		log.Printf("迁移旧数据失败: %v", err)
	}

	userRepo := repository.NewGormUserKeyRepository(db)
	auditLogRepo := repository.NewGormAuditLogRepository(db)

	keyHandler := handler.NewKeyHandler(userRepo, auditLogRepo)
	healthHandler := handler.NewHealthHandler(db)
	pageHandler := handler.NewPageHandler(userRepo, auditLogRepo)

	// 使用 gin.New 并显式挂载中间件，避免 gin.Default 重复注册日志。
	setGinMode(cfg.LogLevel)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("无法设置可信代理: %v", err)
	}
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.BodyLimit(1 << 20))

	r.GET("/", pageHandler.Page)
	r.GET("/stats", pageHandler.StatsAPI)
	r.GET("/health", healthHandler.Health)

	r.GET("/keys/:username", keyHandler.GetKey)
	r.GET("/keys/:username/metadata", keyHandler.GetKeyMetadata)

	admin := r.Group("/keys")
	admin.Use(middleware.AdminAuth(cfg))
	{
		admin.PUT("/:username", keyHandler.PutKey)
		admin.POST("/:username", keyHandler.PostKey)
		admin.DELETE("/:username", keyHandler.DeleteKey)
		admin.GET("", keyHandler.ListKeys)
	}

	// 给每个阶段设置超时，防止慢连接长期占用资源。
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("启动 iskey-server 于 %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("无法启动服务器: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	}
	log.Println("服务器已停止")
}

// setGinMode 根据日志级别切换 Gin 的调试/发布模式。
func setGinMode(level string) {
	switch level {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "warn", "warning", "error":
		gin.SetMode(gin.ReleaseMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}
}

// migrateLegacyUsers 将旧 users 表逐行迁移到 user_keys，重复指纹会被跳过。
func migrateLegacyUsers(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.User{}) {
		return nil
	}
	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if user.Username == "" || user.PublicKey == "" {
			continue
		}
		fingerprint, err := validator.Fingerprint(user.PublicKey)
		if err != nil {
			continue
		}
		key := &model.UserKey{
			Username:    user.Username,
			PublicKey:   user.PublicKey,
			Fingerprint: fingerprint,
			Source:      user.Source,
		}
		if err := db.Where("username = ? AND fingerprint = ?", key.Username, key.Fingerprint).
			FirstOrCreate(key).Error; err != nil {
			return err
		}
	}
	return nil
}

// initDB 按驱动建立数据库连接并设置连接池上限。
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

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

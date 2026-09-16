package main

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d . --parseInternal -o internal/kadmin/docs --outputTypes go,json,yaml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// ① 引入框架适配器（自注册）
	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	// ② 引入数据库驱动（PostgreSQL）
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/postgres"
	// ③ 引入主题（必须！否则 panic: wrong theme name）
	_ "github.com/GoAdminGroup/themes/adminlte"

	"github.com/GoAdminGroup/go-admin/engine"
	"github.com/GoAdminGroup/go-admin/internal/kadmin"
	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/language"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Printf("KAdmin 后端服务启动失败：%v", err)
		os.Exit(1)
	}
}

// @title KAdmin API
// @version 1.0
// @description KAdmin 前后端分离接口文档。除登录、刷新令牌和公开配置外，请使用 Bearer Token 调用接口。
// @license.name Apache 2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /api
// @schemes http https
// @accept json
// @produce json
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func run() error {
	if err := loadEnvironment(".env"); err != nil {
		return fmt.Errorf("加载根目录 .env 失败: %w", err)
	}

	debug := getenvBool("KADMIN_APP_DEBUG", true)
	swaggerEnabled := getenvBool("KADMIN_SWAGGER_ENABLED", debug)
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	gin.DefaultWriter = io.Discard
	log.SetOutput(ginDebugErrorWriter{destination: os.Stderr})

	r := gin.New()
	e := engine.Default()
	addr := ":" + getenv("KADMIN_APP_PORT", "9033")

	// ③ 配置
	cfg := config.Config{
		Env: config.EnvLocal,
		Databases: config.DatabaseList{
			"default": {
				Host:            getenv("KADMIN_DB_HOST", "127.0.0.1"),
				Port:            getenv("KADMIN_DB_PORT", "15432"),
				User:            getenv("KADMIN_DB_USER", "postgres"),
				Pwd:             getenv("KADMIN_DB_PASSWORD", "kadmin_dev_pwd"),
				Name:            getenv("KADMIN_DB_NAME", "kadmin"),
				Driver:          config.DriverPostgresql,
				MaxIdleConns:    50,
				MaxOpenConns:    150,
				ConnMaxLifetime: time.Hour,
			},
		},
		UrlPrefix: "admin", // 所有后台路由在 /admin/ 下
		Store:     config.Store{Path: "./uploads", Prefix: "uploads"},
		Extra: config.ExtraInfo{
			"minio": minioConfig(),
		},
		Language: language.CN, // 中文
		Debug:    debug,
	}

	// ④ 初始化引擎
	requestLogs, appRuntime, err := setupBackend(r, e, &cfg)
	if err != nil {
		return err
	}
	defer closeDatabase(e)
	defer requestLogs.Close()
	defer appRuntime.Close()
	kadmin.RegisterSwagger(r, swaggerEnabled)

	// 访问根路径自动跳转到后台
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin")
	})

	r.Static("/uploads", "./uploads")

	listener, err := listenHTTP(addr)
	if err != nil {
		return err
	}

	server := &http.Server{Handler: r}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	mode := gin.ReleaseMode
	if debug {
		mode = gin.DebugMode
	}
	log.Printf("KAdmin 后端服务启动成功（模式：%s）", mode)
	log.Printf("HTTP 监听地址：%s", listener.Addr())
	log.Print("KAdmin API 前缀：/api；前端项目：admin-web（独立运行）")
	if swaggerEnabled {
		log.Print("Swagger API 文档：/swagger/index.html")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
		return nil
	case sig := <-quit:
		log.Printf("收到退出信号 %s，正在停止 KAdmin 后端服务", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("HTTP 服务停止失败: %w", err)
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP 服务停止时发生错误: %w", err)
	}
	log.Print("KAdmin 后端服务已停止")
	return nil
}

func setupBackend(r *gin.Engine, e *engine.Engine, cfg *config.Config) (requestLogs *kadmin.RequestLogListener, appRuntime *kadmin.Runtime, err error) {
	stage := "初始化 GoAdmin 公共组件"
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s失败: %v", stage, recovered)
		}
		if err != nil && requestLogs != nil {
			requestLogs.Close()
			requestLogs = nil
		}
		if err != nil && appRuntime != nil {
			appRuntime.Close()
			appRuntime = nil
		}
	}()

	e.AddConfig(cfg)
	requestLogs = kadmin.NewRequestLogListener(e.DefaultConnection())
	r.Use(requestLogs.Middleware())
	if err := e.Use(r); err != nil {
		return nil, nil, fmt.Errorf("%s失败: %w", stage, err)
	}

	stage = "注册 KAdmin API"
	appRuntime, err = kadmin.Register(r, e.DefaultConnection())
	if err != nil {
		return nil, nil, fmt.Errorf("%s失败: %w", stage, err)
	}
	// Feed the request log listener into the interface load ranking sampler so
	// every logged request is aggregated into the bounded metric buckets.
	requestLogs.AttachMetricSink(appRuntime.LoadRankSampler())
	return requestLogs, appRuntime, nil
}

func listenHTTP(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("HTTP 服务监听 %q 失败: %w", addr, err)
	}
	return listener, nil
}

type ginDebugErrorWriter struct {
	destination io.Writer
}

func (w ginDebugErrorWriter) Write(p []byte) (int, error) {
	message := bytes.TrimLeft(p, "\r\n")
	if bytes.HasPrefix(message, []byte("[GIN-debug]")) &&
		!bytes.HasPrefix(message, []byte("[GIN-debug] [ERROR]")) {
		return len(p), nil
	}
	return w.destination.Write(p)
}

func closeDatabase(e *engine.Engine) {
	for _, err := range e.PostgresqlConnection().Close() {
		if err != nil {
			log.Printf("关闭数据库连接失败：%v", err)
		}
	}
}

func loadEnvironment(path string) error {
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func minioConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":           getenvBool("KADMIN_MINIO_ENABLED", false),
		"endpoint":          getenv("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000"),
		"internal_endpoint": getenv("KADMIN_MINIO_INTERNAL_ENDPOINT", "minio:9000"),
		"access_key":        getenv("KADMIN_MINIO_ACCESS_KEY", "kadmin_minio"),
		"secret_key":        getenv("KADMIN_MINIO_SECRET_KEY", "kadmin_minio_pwd"),
		"bucket":            getenv("KADMIN_MINIO_BUCKET", "kadmin"),
		"use_ssl":           getenvBool("KADMIN_MINIO_USE_SSL", false),
	}
}

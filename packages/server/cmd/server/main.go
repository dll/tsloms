package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/handler"
	"github.com/tsloms/server/internal/middleware"
	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/mqtt"
	"gorm.io/gorm"
)

// mqttClient 全局 MQTT 客户端实例（用于优雅停机时断开连接）
var mqttClient *mqtt.MQTTClient

func main() {
	// 启动即构造进程内配置单例（后续热路径经 config.Get() 复用同一实例）
	cfg := config.Get()

	// 生产环境拒绝弱默认凭据/密钥启动
	if cfg.IsProduction() {
		weak := []string{"tsloms-secret-key", "", "123456"}
		for _, w := range weak {
			if cfg.JWTSecret == w {
				log.Fatalf("生产环境禁止使用默认/弱 JWT_SECRET，请设置强密钥")
			}
			if cfg.DBPassword == w || cfg.DBPassword == "root" {
				log.Fatalf("生产环境禁止使用默认/弱数据库密码")
			}
		}
	}

	// 初始化数据库
	db, err := model.InitDB(cfg)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	if cfg.DBDriver == "sqlite" {
		log.Printf("SQLite 模式：使用本地文件库 %s", cfg.DBName)
	} else {
		// 初始化 Redis（SQLite 模式降级跳过）
		if err := model.InitRedis(cfg); err != nil {
			log.Printf("Redis 初始化失败（继续启动）: %v", err)
		}
	}

	// 初始化 MQTT 客户端并连接 Broker
	mqttClient = mqtt.NewMQTTClient()
	if err := mqttClient.Connect(cfg.MQTTBroker, cfg.MQTTUsername, cfg.MQTTPassword, cfg.MQTTClientID); err != nil {
		log.Printf("MQTT 连接失败（继续启动）: %v", err)
	} else {
		// 订阅设备上行 Topic：{topicPrefix}/+/+/+/U
		// + 为 MQTT 单层通配符，匹配任意网络号/站点号/硬件ID
		topic := cfg.MQTTTopicPrefix + "/+/+/+/U"
		mqttHandler := mqtt.NewHandler(mqttClient)
		if err := mqttClient.Subscribe(topic, mqttHandler.HandleMessage); err != nil {
			log.Printf("MQTT 订阅失败: %v", err)
		}
	}

	r := setupRouter(db, cfg)

	// 可信代理：仅信任本机 nginx/Caddy
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// 优雅停机：SIGTERM/SIGINT 时等待当前请求完成（最多 10s）再退出
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("TSLOMS 服务启动，监听端口 %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在优雅关闭服务...")

	// 断开 MQTT 连接（等待 2 秒处理完剩余消息）
	if mqttClient != nil {
		mqttClient.Disconnect(2000)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	}
	log.Println("服务已退出")
}

// setupRouter 配置路由
func setupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	api := r.Group("/api/v1")
	{
		// 公开接口（无需登录）
		api.POST("/auth/login", handler.Login)

		// 受保护接口（需登录）
		auth := api.Group("")
		auth.Use(middleware.Auth(cfg))
		{
			// 用户信息
			auth.GET("/user/info", handler.GetUserInfo)

			// 设备管理
			auth.GET("/devices", handler.ListDevices)
			auth.GET("/devices/stats", handler.DeviceStats)
			auth.GET("/devices/:id", handler.GetDevice)
			auth.PUT("/devices/:id", handler.UpdateDevice)

			// 故障查询
			auth.GET("/faults", handler.ListFaults)
			auth.GET("/faults/:id", handler.GetFault)

			// 工单管理
			auth.GET("/work-orders", handler.ListWorkOrders)
			auth.POST("/work-orders", handler.CreateWorkOrder)
			auth.PUT("/work-orders/:id/status", handler.UpdateWorkOrderStatus)

			// 数据看板
			auth.GET("/dashboard/overview", handler.DashboardOverview)
			auth.GET("/dashboard/fault-type-stats", handler.FaultTypeStats)
			auth.GET("/dashboard/work-order-stats", handler.WorkOrderStatusStats)
			auth.GET("/dashboard/fault-trend", handler.FaultTrendStats)
			auth.GET("/dashboard/device-fault-rank", handler.DeviceFaultRank)
		}
	}

	return r
}

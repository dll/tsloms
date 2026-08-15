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
	"github.com/tsloms/server/internal/service"
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

	// 初始化默认管理员账户
	if err := model.SeedAdmin(); err != nil {
		log.Printf("管理员种子失败: %v", err)
	} else {
		// 初始化 AI 配置（从环境变量填入 API Key）
		model.SeedAIConfig(cfg.AIAPIKey, cfg.AITextModel, cfg.AIVisionModel)
		log.Println("管理员账户已就绪（admin/admin123）")
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

	// 启动设备离线检测协程（签到超时自动置离线）
	offlineCtx, offlineCancel := context.WithCancel(context.Background())
	offlineSvc := service.NewOfflineCheck(cfg)
	offlineSvc.Start(offlineCtx)

	// 启动工单超时升级协程（pending 超 24h 自动转 processing，processing 超 48h 预警）
	woCtx, woCancel := context.WithCancel(context.Background())
	woSvc := service.NewWorkOrderEscalator()
	woSvc.Start(woCtx)

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

	// 停止离线检测协程
	offlineCancel()
	// 停止工单超时升级协程
	woCancel()

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

	// 媒体静态文件（手机上传的举证/监控片段，供前端播放/展示，同源访问）
	// 存储目录经 MEDIA_DIR 配置，默认 ./uploads/media
	r.Static("/media", handler.MediaDir())

	api := r.Group("/api/v1")
	{
		// 公开接口（无需登录）
		api.POST("/auth/login", handler.Login)
		api.GET("/health", handler.Health)
		// 地图瓦片代理（无鉴权，供 Cesium 图片加载使用）
		api.GET("/proxy/baidu", handler.BaiduTileProxy)
		api.GET("/proxy/gaode", handler.GaodeTileProxy)

		// 受保护接口（需登录）
		auth := api.Group("")
		auth.Use(middleware.Auth(cfg))
		{
			// 用户信息
			auth.GET("/user/info", handler.GetUserInfo)
			auth.PUT("/user/phone", handler.UpdateMyPhone)
			auth.PUT("/user/center", handler.UpdateMyCenter)

			// 设备管理（查看：所有角色，修改：管理员/运维）
			auth.GET("/devices", handler.ListDevices)
			auth.GET("/devices/stats", handler.DeviceStats)
			auth.GET("/devices/:id", handler.GetDevice)
			auth.POST("/devices", middleware.RequireOperator(), handler.CreateDevice)
			auth.PUT("/devices/:id", middleware.RequireOperator(), handler.UpdateDevice)
			auth.DELETE("/devices/:id", middleware.RequireAdmin(), handler.DeleteDevice)
			auth.GET("/intersections", handler.ListIntersections)
			auth.PUT("/intersections/rename", middleware.RequireOperator(), handler.RenameIntersection)
			auth.PUT("/intersections/location", middleware.RequireOperator(), handler.SetIntersectionLocation)
			auth.DELETE("/intersections/clear", middleware.RequireAdmin(), handler.ClearIntersection)

			// 故障查询（只读）
			auth.GET("/faults", handler.ListFaults)
			auth.GET("/faults/:id", handler.GetFault)

			// 工单管理（查看：所有角色，操作：管理员/运维）
			auth.GET("/work-orders", handler.ListWorkOrders)
	auth.GET("/work-orders/:id", handler.GetWorkOrder)
			auth.POST("/work-orders", middleware.RequireOperator(), handler.CreateWorkOrder)
			auth.PUT("/work-orders/:id/status", middleware.RequireOperator(), handler.UpdateWorkOrderStatus)
			auth.PUT("/work-orders/:id/assign", middleware.RequireOperator(), handler.AssignWorkOrder)
			auth.DELETE("/work-orders/:id", middleware.RequireAdmin(), handler.DeleteWorkOrder)

			// 数据看板（只读）
			auth.GET("/dashboard/overview", handler.DashboardOverview)
			auth.GET("/dashboard/fault-type-stats", handler.FaultTypeStats)
			auth.GET("/dashboard/work-order-stats", handler.WorkOrderStatusStats)
			auth.GET("/dashboard/fault-trend", handler.FaultTrendStats)
			auth.GET("/dashboard/device-fault-rank", handler.DeviceFaultRank)
			auth.GET("/dashboard/work-order-avg-closure", handler.WorkOrderAvgClosure)
	auth.GET("/dashboard/ai-overview", handler.AIDashboardOverview)

			// 日志查询
			auth.GET("/logs/packets", handler.ListPacketLogs)
			auth.GET("/logs/operations", handler.ListOperationLogs)

			// 用户管理（仅管理员）
			users := auth.Group("/users", middleware.RequireAdmin())
			{
				users.GET("", handler.ListUsers)
				users.POST("", handler.CreateUser)
				users.PUT("/:id", handler.UpdateUser)
				users.PUT("/:id/password", handler.ResetUserPassword)
				users.DELETE("/:id", handler.DeleteUser)
			}

			// 组织/部门管理（列表所有人可读，增删改仅管理员）
			auth.GET("/departments", handler.ListDepartments)
			departments := auth.Group("/departments", middleware.RequireAdmin())
			{
				departments.POST("", handler.CreateDepartment)
				departments.PUT("/:id", handler.UpdateDepartment)
				departments.DELETE("/:id", handler.DeleteDepartment)
			}

			// 设备媒体（视频举证/监控/时间视频）
			auth.GET("/media", handler.ListDeviceMedia)
			auth.POST("/media/upload", handler.UploadDeviceMedia)
			auth.POST("/media/streams", handler.CreateRTSPMedia)
			auth.DELETE("/media/:id", middleware.RequireOperator(), handler.DeleteDeviceMedia)

			// 维修耗材
			auth.GET("/materials", handler.ListMaterials)
			auth.POST("/materials", middleware.RequireOperator(), handler.UpsertMaterial)
			auth.DELETE("/materials/:id", middleware.RequireOperator(), handler.DeleteMaterial)

			// 问题反馈
			auth.GET("/feedbacks", handler.ListFeedbacks)
			auth.POST("/feedbacks", handler.CreateFeedback)
			auth.PUT("/feedbacks/:id", handler.UpdateFeedbackStatus)

			// 派单参考（设备聚合：故障/工单/耗材/媒体）
			auth.GET("/dispatch/reference", handler.DispatchReference)

			// 可派单人员（运维/管理员），供工单派单
			auth.GET("/users/assignable", handler.ListAssignableUsers)

			// ---- AI 分析 ----
			// 配置（管理员读写，普通用户读）+ 我的额度
			auth.GET("/ai/config", handler.GetAIConfig)
			auth.PUT("/ai/config", middleware.RequireAdmin(), handler.UpdateAIConfig)
			auth.GET("/ai/usage", handler.MyAIUsage)
			auth.GET("/ai/usage/logs", middleware.RequireAdmin(), handler.AIUsagePage)
			auth.POST("/ai/usage/reset", middleware.RequireAdmin(), handler.ResetAIUsage)
			// 故障预测
			auth.POST("/ai/predict/run", middleware.RequireOperator(), handler.RunPrediction)
	auth.GET("/ai/predict/by-intersection", handler.RunPredictionByIntersection)
			auth.GET("/ai/predict", handler.AIPredictions)
			auth.POST("/ai/predict/:id/enhance", handler.EnhancePredictionPlan)
			// 故障诊断（反馈，含图片）
			auth.POST("/ai/diagnose/:id", handler.DiagnoseFeedbackAPI)
			// 生命周期溯源
			auth.GET("/ai/lifecycle/:hwid", handler.BuildLifecycleAPI)
		}
	}

	return r
}

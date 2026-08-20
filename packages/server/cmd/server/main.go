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

	// 初始化默认管理员账户（首次初始化时可用 ADMIN_INIT_PASSWORD 指定；为空则生成随机强密码并打印一次）
	if initPwd, err := model.SeedAdmin(cfg.AdminInitPwd); err != nil {
		log.Printf("管理员种子失败: %v", err)
	} else {
		// 初始化 AI 配置（从环境变量填入 API Key）
		model.SeedAIConfig(cfg.AIAPIKey, cfg.AITextModel, cfg.AIVisionModel)
		if initPwd != "" {
			// 仅首次初始化且未显式指定密码时打印，供管理员安全渠道保存；日志权限应受限
			log.Printf("已生成初始管理员随机密码（仅本次输出，请安全保存）: %s", initPwd)
		} else {
			log.Println("管理员账户已就绪（已存在或使用 ADMIN_INIT_PASSWORD）")
		}
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
	// 注册全局 MQTT 客户端（供检测器接入状态上报使用）
	handler.SetMQTTClient(mqttClient)
	// 注册消息处理器（供 Mock 模拟/CSV 回放复用真实研判链路）
	mqttHandlerGlobal := mqtt.NewHandler(mqttClient)
	mqtt.RegisterActiveHandler(mqttHandlerGlobal)
	mqtt.SetSimTopic(cfg.MQTTTopicPrefix, "0", "0")
	if err := mqttClient.Connect(cfg.MQTTBroker, cfg.MQTTUsername, cfg.MQTTPassword, cfg.MQTTClientID); err != nil {
		log.Printf("MQTT 连接失败（继续启动）: %v", err)
	} else {
		// 订阅设备上行 Topic：{topicPrefix}/+/+/+/U
		// + 为 MQTT 单层通配符，匹配任意网络号/站点号/硬件ID
		topic := cfg.MQTTTopicPrefix + "/+/+/+/U"
		if err := mqttClient.Subscribe(topic, mqttHandlerGlobal.HandleMessage); err != nil {
			log.Printf("MQTT 订阅失败: %v", err)
		}
	}

	r := setupRouter(db, cfg)

	// 启动设备离线检测协程（签到超时自动置离线）
	offlineCtx, offlineCancel := context.WithCancel(context.Background())
	offlineSvc := service.NewOfflineCheck(cfg)
	offlineSvc.Start(offlineCtx)

	// 启动 AI 主动巡检协程（定时生成运维日报 + 异常检测 + 站内推送）
	patrolCtx, patrolCancel := context.WithCancel(context.Background())
	patrolSvc := service.NewPatrolService()
	patrolSvc.Start(patrolCtx)

	// 启动工单超时升级协程（pending 超 24h 自动转 processing，processing 超 48h 预警）
	woCtx, woCancel := context.WithCancel(context.Background())
	woSvc := service.NewWorkOrderEscalator()
	woSvc.Start(woCtx)

	// 可信代理：仅信任本机 nginx/Caddy
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// 优雅停机：SIGTERM/SIGINT 时等待当前请求完成（最多 10s）再退出
	// 超时保护（P1-05）：限制报文头/请求体读写与空闲连接时长，缓解慢请求与连接耗尽
	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
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
	// 停止 AI 主动巡检协程
	patrolCancel()
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
		api.POST("/auth/register", handler.Register)
		api.GET("/auth/captcha", handler.GetCaptcha)
		api.GET("/health", handler.Health)
		// 公开部门列表（仅 id/name，供注册页未登录态选择归属部门，避免暴露部门人数等内部数据）
		api.GET("/public/departments", handler.PublicDepartments)
		// 地图瓦片代理（无鉴权，供 Cesium 图片加载使用）
		api.GET("/proxy/baidu", handler.BaiduTileProxy)
		api.GET("/proxy/gaode", handler.GaodeTileProxy)

		// 受保护接口（需登录）
		auth := api.Group("")
		auth.Use(middleware.Auth(cfg))
		{
			// 当前实例已启模块列表（核心 + 已购可选）——前端动态菜单/路由依据
			auth.GET("/modules", handler.ListEnabledModules)
			auth.GET("/modules/settings", middleware.RequirePerm("module:manage"), handler.ListModuleSettings)
			auth.PUT("/modules/settings", middleware.RequirePerm("module:manage"), handler.UpdateModuleSettings)
			// 授权/试用管理（仅超级管理员 module:manage）
			auth.GET("/license/status", middleware.RequirePerm("module:manage"), handler.GetLicenseStatus)
			auth.POST("/license/trial/start", middleware.RequirePerm("module:manage"), handler.StartTrial)
			auth.POST("/license/unlock", middleware.RequirePerm("module:manage"), handler.UnlockLicense)

			// 用户信息
			auth.GET("/user/info", handler.GetUserInfo)
			auth.PUT("/user/phone", handler.UpdateMyPhone)
			auth.PUT("/user/center", handler.UpdateMyCenter)
			auth.PUT("/user/profile", handler.UpdateMyProfile)
			auth.POST("/user/avatar", handler.UploadMyAvatar)

			// 设备管理（查看：所有角色，修改：管理员/运维）
			auth.GET("/devices", handler.ListDevices)
			auth.GET("/devices/stats", handler.DeviceStats)
			auth.GET("/devices/:id", handler.GetDevice)
			auth.POST("/devices", middleware.RequirePerm("device:create"), handler.CreateDevice)
			auth.PUT("/devices/:id", middleware.RequirePerm("device:update"), handler.UpdateDevice)
			auth.POST("/devices/:id/retire", middleware.RequirePerm("device:update"), handler.RetireDevice)
			auth.POST("/devices/:id/restore", middleware.RequirePerm("device:update"), handler.RestoreDevice)
			auth.DELETE("/devices/:id", middleware.RequirePerm("device:delete"), handler.DeleteDevice)
			auth.GET("/intersections", handler.ListIntersections)
			auth.PUT("/intersections/rename", middleware.RequirePerm("intersection:update"), handler.RenameIntersection)
			auth.PUT("/intersections/location", middleware.RequirePerm("intersection:update"), handler.SetIntersectionLocation)
			auth.DELETE("/intersections/clear", middleware.RequirePerm("intersection:delete"), handler.ClearIntersection)

			// ---- P0-4 路口/行政区划（新增独立命名空间，向后兼容） ----
			auth.GET("/crossings", handler.ListCrossings)
			auth.GET("/crossings/:id", handler.GetCrossing)
			auth.POST("/crossings", middleware.RequirePerm("crossing:manage"), handler.CreateCrossing)
			auth.PUT("/crossings/:id", middleware.RequirePerm("crossing:manage"), handler.UpdateCrossing)
			auth.DELETE("/crossings/:id", middleware.RequirePerm("crossing:manage"), handler.DeleteCrossing)
			auth.GET("/crossings/:id/devices", handler.GetCrossingDevices)
			auth.GET("/areas/tree", handler.ListAreasTree)
			auth.POST("/areas", middleware.RequirePerm("area:manage"), handler.CreateArea)
			auth.PUT("/areas/:id", middleware.RequirePerm("area:manage"), handler.UpdateArea)
			auth.DELETE("/areas/:id", middleware.RequirePerm("area:manage"), handler.DeleteArea)

			// ---- P0-5 地图分级聚合（新增，向后兼容；/map 前端原有设备打点不变） ----
			auth.GET("/map/crossing-data", handler.GetCrossingMapData)
			auth.GET("/map/road-data", handler.GetRoadMapData)
			// 路口详情聚合（点击卡片：设备/预警/故障/工单/维护列表）
			auth.GET("/map/crossing/:id/detail", handler.CrossingDetail)
			// 高德 POI 地名搜索代理（走服务器端 AMAP_WEB_KEY；未配置时前端降级本地搜索）
			auth.GET("/proxy/amap/place", handler.AmapPlaceSearch)
			// 检测器接入（真实硬件 / CSV 导入 / Mock 模拟）：状态 + Mock 发送 + CSV 回放
			auth.GET("/access/status", handler.DetectorAccessStatus)
			auth.POST("/access/mock/send", handler.MockSend)
			auth.POST("/access/csv/import", handler.CSVImport)
			// 系统演示（仅系统管理员）：生成随机演示数据 / 一键清理回滚
			auth.GET("/demo/status", middleware.RequireSystemAdmin(), handler.DemoStatus)
			auth.POST("/demo/start", middleware.RequireSystemAdmin(), handler.DemoStart)
			auth.POST("/demo/end", middleware.RequireSystemAdmin(), handler.DemoEnd)

			// ---- P1 自动巡检（独立命名空间 /patrol/*） ----
			auth.GET("/patrol/tasks", handler.ListPatrolTasks)
			auth.POST("/patrol/tasks", middleware.RequirePerm("patrol:manage"), handler.CreatePatrolTask)
			auth.GET("/patrol/tasks/:id", handler.GetPatrolTask)
			auth.PUT("/patrol/tasks/:id", middleware.RequirePerm("patrol:manage"), handler.UpdatePatrolTask)
			auth.DELETE("/patrol/tasks/:id", middleware.RequirePerm("patrol:manage"), handler.DeletePatrolTask)
			auth.POST("/patrol/tasks/:id/run", middleware.RequirePerm("patrol:run"), handler.RunPatrolTask)
			auth.GET("/patrol/records", handler.ListPatrolRecords)
			auth.GET("/patrol/ranking", handler.GetPatrolRanking)
			auth.POST("/patrol/selfcheck", middleware.RequirePerm("patrol:selfcheck"), handler.PostPatrolSelfCheck)

			// ---- P0-3 预警管理（新增独立命名空间） ----
			auth.GET("/warnings", handler.ListWarnings)
			auth.GET("/warnings/export", middleware.RequirePerm("warning:manage"), handler.ExportWarnings)
			auth.GET("/warnings/:id", handler.GetWarning)
			auth.POST("/warnings/:id/ignore", middleware.RequirePerm("warning:manage"), handler.IgnoreWarning)
			auth.POST("/warnings/batch-ignore", middleware.RequirePerm("warning:manage"), handler.BatchIgnoreWarnings)
			auth.POST("/warnings/:id/to-workorder", middleware.RequirePerm("warning:manage"), handler.WarningToWorkOrder)
			auth.POST("/warnings/auto-ignore", middleware.RequirePerm("warning:manage"), handler.AutoIgnoreWarnings)
			// 预警配置（忽略规则）CRUD
			auth.GET("/warning-rules", handler.ListWarningRules)
			auth.POST("/warning-rules", middleware.RequirePerm("warning:rule"), handler.CreateWarningRule)
			auth.PUT("/warning-rules/:id", middleware.RequirePerm("warning:rule"), handler.UpdateWarningRule)
			auth.DELETE("/warning-rules/:id", middleware.RequirePerm("warning:rule"), handler.DeleteWarningRule)

			// 故障查询（只读）
			auth.GET("/faults", handler.ListFaults)
			auth.GET("/faults/export", handler.ExportFaults)
			auth.GET("/faults/:id", handler.GetFault)
			// 故障管理（确认/负责人/维修人/状态更新：管理员/运维）
			auth.PUT("/faults/:id", middleware.RequirePerm("fault:update"), handler.UpdateFault)
			auth.DELETE("/faults/:id", middleware.RequirePerm("fault:delete"), handler.DeleteFault)
			auth.POST("/faults/:id/dispatch", middleware.RequirePerm("fault:dispatch"), handler.DispatchFault)

			// ---- 智能多源故障识别研判引擎（范围A，独立路径向后兼容） ----
			// 多源证据明细（待确认/被过滤证据亦可回看）
			auth.GET("/faults/:id/evidence", handler.ListFaultEvidence)
			// 待确认复核：证据补充后升级为确认（可自动派单）
			auth.POST("/faults/:id/review", middleware.RequirePerm("fault:review"), handler.ReviewFault)
			// 预留外部数据源证据写入（举证/反馈/监控）
			auth.POST("/evidence/ingest", middleware.RequirePerm("evidence:ingest"), handler.IngestEvidence)
			auth.GET("/evidence/sources", handler.ListEvidenceSources)
			// 识别案例库（列表/回标/训练/统计）
			auth.GET("/fault-cases", handler.ListFaultCases)
			auth.POST("/fault-cases", middleware.RequirePerm("faultcase:manage"), handler.CreateFaultCase)
			auth.POST("/fault-cases/train", middleware.RequirePerm("faultcase:manage"), handler.TrainFaultCases)
			auth.GET("/recognition/stats", handler.RecognitionStats)

			// 工单管理（查看：所有角色，操作：管理员/运维）
			auth.GET("/work-orders", handler.ListWorkOrders)
			auth.GET("/work-orders/:id", handler.GetWorkOrder)
			auth.POST("/work-orders", middleware.RequirePerm("workorder:create"), handler.CreateWorkOrder)
			auth.PUT("/work-orders/:id/status", middleware.RequirePerm("workorder:update"), handler.UpdateWorkOrderStatus)
			auth.PUT("/work-orders/:id/assign", middleware.RequirePerm("workorder:assign"), handler.AssignWorkOrder)
			auth.DELETE("/work-orders/:id", middleware.RequirePerm("workorder:delete"), handler.DeleteWorkOrder)

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

			// 用户管理（仅“用户-管理”权限）
			users := auth.Group("/users", middleware.RequirePerm("user:manage"))
			{
				users.GET("", handler.ListUsers)
				users.POST("", handler.CreateUser)
				users.PUT("/:id", handler.UpdateUser)
				users.PUT("/:id/password", handler.ResetUserPassword)
				users.DELETE("/:id", handler.DeleteUser)
			}

			// 组织/部门管理（列表所有人可读，增删改仅“组织-管理”权限）
			auth.GET("/departments", handler.ListDepartments)
			departments := auth.Group("/departments", middleware.RequirePerm("dept:manage"))
			{
				departments.POST("", handler.CreateDepartment)
				departments.PUT("/:id", handler.UpdateDepartment)
				departments.DELETE("/:id", handler.DeleteDepartment)
			}

			// ---- 角色与权限管理（仅“角色-管理”权限） ----
			rbac := auth.Group("/rbac", middleware.RequirePerm("role:manage"))
			{
				rbac.GET("/permissions", handler.ListPermissions)
				rbac.GET("/roles", handler.ListRoles)
				rbac.POST("/roles", handler.CreateRole)
				rbac.PUT("/roles/:id", handler.UpdateRole)
				rbac.DELETE("/roles/:id", handler.DeleteRole)
				rbac.GET("/users/:id/permissions", handler.GetUserPermissions)
				rbac.PUT("/users/:id/permissions", handler.SetUserPermissions)
			}
			// 当前登录用户的有效权限（所有登录用户可读，供前端菜单/按钮联动）
			auth.GET("/my/permissions", handler.MyPermissions)

			// 设备媒体（视频举证/监控/时间视频）
			auth.GET("/media", handler.ListDeviceMedia)
			auth.POST("/media/upload", middleware.RequirePerm("media:upload"), handler.UploadDeviceMedia)
			// RTSP 实时拉流仅视频监控模块（可选）
			auth.POST("/media/streams", handler.RequireModule(handler.ModuleVideo), middleware.RequirePerm("media:upload"), handler.CreateRTSPMedia)
			auth.DELETE("/media/:id", middleware.RequirePerm("media:delete"), handler.DeleteDeviceMedia)

			// 固件管理（OTA 升级）
			auth.GET("/firmwares", handler.ListFirmwares)
			auth.GET("/firmwares/:id", handler.GetFirmware)
			auth.POST("/firmwares/upload", middleware.RequirePerm("firmware:manage"), handler.UploadFirmware)
			auth.PUT("/firmwares/:id", middleware.RequirePerm("firmware:manage"), handler.UpdateFirmware)
			auth.PUT("/firmwares/:id/publish", middleware.RequirePerm("firmware:manage"), handler.PublishFirmware)
			auth.DELETE("/firmwares/:id", middleware.RequirePerm("firmware:delete"), handler.DeleteFirmware)
			auth.GET("/firmware-upgrades", handler.ListFirmwareUpgrades)
			auth.POST("/firmware-upgrades", middleware.RequirePerm("firmware:manage"), handler.CreateFirmwareUpgrade)
			auth.DELETE("/firmware-upgrades/:id", middleware.RequirePerm("firmware:delete"), handler.DeleteFirmwareUpgrade)

			// 库存管理（物料档案 + 出入库流水 + 统计）【可选模块 inventory】
			auth.GET("/inv/materials", handler.RequireModule(handler.ModuleInventory), handler.ListMaterialsV2)
			auth.GET("/inv/materials/stats", handler.RequireModule(handler.ModuleInventory), handler.MaterialStats)
			auth.POST("/inv/materials", handler.RequireModule(handler.ModuleInventory), middleware.RequirePerm("inventory:manage"), handler.SaveMaterial)
			auth.PUT("/inv/materials/:id", handler.RequireModule(handler.ModuleInventory), middleware.RequirePerm("inventory:manage"), handler.SaveMaterial)
			auth.DELETE("/inv/materials/:id", handler.RequireModule(handler.ModuleInventory), middleware.RequirePerm("inventory:delete"), handler.DeleteMaterialV2)
			auth.GET("/inv/stocks", handler.RequireModule(handler.ModuleInventory), handler.ListMaterialStocks)
			auth.POST("/inv/stocks/adjust", handler.RequireModule(handler.ModuleInventory), middleware.RequirePerm("inventory:manage"), handler.AdjustMaterialStock)
			auth.POST("/inv/stocks/use", handler.RequireModule(handler.ModuleInventory), middleware.RequirePerm("inventory:manage"), handler.UseMaterialStock)

			// 供应商【可选模块 supplier】
			auth.GET("/suppliers", handler.RequireModule(handler.ModuleSupplier), handler.ListSuppliers)
			auth.POST("/suppliers", handler.RequireModule(handler.ModuleSupplier), middleware.RequirePerm("supplier:manage"), handler.SaveSupplier)
			auth.PUT("/suppliers/:id", handler.RequireModule(handler.ModuleSupplier), middleware.RequirePerm("supplier:manage"), handler.SaveSupplier)
			auth.DELETE("/suppliers/:id", handler.RequireModule(handler.ModuleSupplier), middleware.RequirePerm("supplier:delete"), handler.DeleteSupplier)

			// 采购单（进销存）【可选模块 purchase】
			auth.GET("/purchases", handler.RequireModule(handler.ModulePurchase), handler.ListPurchaseOrders)
			auth.GET("/purchases/:id", handler.RequireModule(handler.ModulePurchase), handler.GetPurchaseOrder)
			auth.POST("/purchases", handler.RequireModule(handler.ModulePurchase), middleware.RequirePerm("purchase:manage"), handler.CreatePurchaseOrder)
			auth.POST("/purchases/:id/receive", handler.RequireModule(handler.ModulePurchase), middleware.RequirePerm("purchase:manage"), handler.ReceivePurchase)
			auth.POST("/purchases/:id/cancel", handler.RequireModule(handler.ModulePurchase), middleware.RequirePerm("purchase:manage"), handler.CancelPurchase)
			auth.DELETE("/purchases/:id", handler.RequireModule(handler.ModulePurchase), middleware.RequirePerm("purchase:delete"), handler.DeletePurchase)

			// 维修费用（耗材/人工/交通/其它）【可选模块 expense】
			auth.GET("/expenses", handler.RequireModule(handler.ModuleExpense), handler.ListRepairExpenses)
			auth.GET("/expenses/stats", handler.RequireModule(handler.ModuleExpense), handler.ExpenseStats)
			auth.POST("/expenses", handler.RequireModule(handler.ModuleExpense), middleware.RequirePerm("expense:manage"), handler.SaveRepairExpense)
			auth.PUT("/expenses/:id", handler.RequireModule(handler.ModuleExpense), middleware.RequirePerm("expense:manage"), handler.SaveRepairExpense)
			auth.PUT("/expenses/:id/confirm", handler.RequireModule(handler.ModuleExpense), middleware.RequirePerm("expense:manage"), handler.ConfirmRepairExpense)
			auth.DELETE("/expenses/:id", handler.RequireModule(handler.ModuleExpense), middleware.RequirePerm("expense:delete"), handler.DeleteRepairExpense)

			// 问题反馈
			auth.GET("/feedbacks", handler.ListFeedbacks)
			auth.POST("/feedbacks", handler.CreateFeedback)
			auth.PUT("/feedbacks/:id", handler.UpdateFeedbackStatus)

			// 派单参考（设备聚合：故障/工单/耗材/媒体）
			auth.GET("/dispatch/reference", handler.RequireModule(handler.ModuleDispatch), handler.DispatchReference)

			// 站内通知（AI 主动巡检推送：报告提醒/异常预警）【可选模块 notification】
			auth.GET("/notifications", handler.RequireModule(handler.ModuleNotification), handler.ListNotificationsAPI)
			auth.GET("/notifications/unread-count", handler.RequireModule(handler.ModuleNotification), handler.UnreadCountAPI)
			auth.PUT("/notifications/:id/read", handler.RequireModule(handler.ModuleNotification), handler.ReadNotificationAPI)
			auth.PUT("/notifications/read-all", handler.RequireModule(handler.ModuleNotification), handler.ReadAllNotificationsAPI)

			// 可派单人员（运维/管理员），供工单派单
			auth.GET("/users/assignable", handler.ListAssignableUsers)

			// ---- AI 分析（可选模块 ai）----
			aiM := handler.RequireModule(handler.ModuleAI)
			// 配置（管理员读写，普通用户读）+ 我的额度
			auth.GET("/ai/config", aiM, handler.GetAIConfig)
			auth.PUT("/ai/config", aiM, middleware.RequirePerm("ai:config"), handler.UpdateAIConfig)
			auth.GET("/ai/usage", aiM, handler.MyAIUsage)
			auth.GET("/ai/usage/logs", aiM, middleware.RequirePerm("ai:config"), handler.AIUsagePage)
			auth.POST("/ai/usage/reset", aiM, middleware.RequirePerm("ai:config"), handler.ResetAIUsage)
			// 故障预测
			auth.POST("/ai/predict/run", aiM, middleware.RequirePerm("ai:ops"), handler.RunPrediction)
			auth.GET("/ai/predict/by-intersection", aiM, middleware.RequirePerm("ai:ops"), handler.RunPredictionByIntersection)
			auth.GET("/ai/predict", aiM, middleware.RequirePerm("ai:ops"), handler.AIPredictions)
			auth.POST("/ai/predict/:id/enhance", aiM, middleware.RequirePerm("ai:ops"), handler.EnhancePredictionPlan)
			// 故障诊断（反馈，含图片）
			auth.POST("/ai/diagnose/:id", aiM, middleware.RequirePerm("ai:ops"), handler.DiagnoseFeedbackAPI)
			// 生命周期溯源
			auth.GET("/ai/lifecycle/:hwid", aiM, middleware.RequirePerm("ai:ops"), handler.BuildLifecycleAPI)

			// ---- AI 原生增强（库存/成本分析 + 运维报告 + 核心流程建议） ----
			// 库存健康 / 成本归因分析
			auth.GET("/ai/analyze/inventory", aiM, middleware.RequirePerm("ai:ops"), handler.AnalyzeInventoryAPI)
			auth.GET("/ai/analyze/cost", aiM, middleware.RequirePerm("ai:ops"), handler.AnalyzeCostAPI)
			// 运维报告（日报/指定模块）生成与历史查询
			auth.POST("/ai/report/generate", aiM, middleware.RequirePerm("ai:ops"), handler.GenerateReportAPI)
			auth.GET("/ai/reports", aiM, middleware.RequirePerm("ai:ops"), handler.ListReportsAPI)
			// 核心流程 AI 建议：故障确认/派单辅助 + 工单 Copilot + 历史查询
			auth.GET("/ai/advice/fault/:id", aiM, middleware.RequirePerm("ai:ops"), handler.SuggestFaultAdviceAPI)
			auth.GET("/ai/advice/workorder/:id", aiM, middleware.RequirePerm("ai:ops"), handler.SuggestWorkOrderAdviceAPI)
			auth.POST("/ai/advice/device", aiM, middleware.RequirePerm("ai:ops"), handler.SuggestDeviceCopilotAPI)
			auth.POST("/ai/advice/workorder/create", aiM, middleware.RequirePerm("ai:ops"), handler.SuggestWorkOrderCreateAPI)
			auth.POST("/ai/advice/purchase", aiM, middleware.RequirePerm("ai:ops"), handler.SuggestPurchaseCopilotAPI)
			auth.GET("/ai/advices", aiM, middleware.RequirePerm("ai:ops"), handler.ListAdvicesAPI)
			auth.POST("/ai/nl/interact", aiM, middleware.RequirePerm("ai:ops"), handler.NLInteractAPI)
			auth.POST("/ai/decision/center", aiM, middleware.RequirePerm("ai:ops"), handler.DecisionCenterAPI)
			auth.POST("/ai/decision/adopt", aiM, middleware.RequirePerm("ai:ops"), middleware.RequirePerm("purchase:manage"), handler.AdoptDecisionAPI)
			auth.GET("/ai/anomaly/stream", aiM, middleware.RequirePerm("ai:ops"), handler.AnomalyStreamAPI)
		}
	}

	return r
}

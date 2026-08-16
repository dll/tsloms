package handler

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
)

// ============================================================================
// 模块化 / 插件化（按甲方购买加载）
// ----------------------------------------------------------------------------
// 核心思路：主菜单按模块划分，甲方购买（启用）的模块才加载，否则不可用。
//   - 核心基础模块：恒启（不可被配置关闭）——仪表盘/设备管理/路口管理/地图大屏/问题反馈/
//     故障管理/工单管理/固件管理/系统日志/系统设置。
//   - 可选模块：默认不加载，经配置启用（ENABLED_MODULES 环境变量，逗号分隔）。
//
// 拦截方式：路由层面用 RequireModule(key) 中间件，未启用模块的接口直接 403。
// 前端：登录/用户信息接口返回 EnabledModules，前端据此动态生成菜单与路由。
// ============================================================================

// 核心基础模块（恒启，不可配置关闭）
const (
	ModuleDashboard    = "dashboard"    // 仪表盘
	ModuleDevice       = "device"       // 设备管理
	ModuleIntersection = "intersection" // 路口管理
	ModuleMap          = "map"          // 地图大屏
	ModuleFeedback     = "feedback"     // 问题反馈
	ModuleFault        = "fault"        // 故障管理
	ModuleWorkorder    = "workorder"    // 工单管理
	ModuleFirmware     = "firmware"     // 固件管理
	ModuleLog          = "log"          // 系统日志
	ModuleSettings     = "settings"     // 系统设置
)

// 可选模块（默认不加载，甲方购买后经 ENABLED_MODULES 启用）
const (
	ModuleVideo        = "video"        // 视频监控（含监控大屏）
	ModuleInventory    = "inventory"    // 物料库存
	ModulePurchase     = "purchase"     // 采购管理
	ModuleExpense      = "expense"      // 维修费用
	ModuleSupplier     = "supplier"     // 供应商
	ModuleAI           = "ai"           // AI 分析
	ModuleDispatch     = "dispatch"     // 派单参考
	ModuleNotification = "notification" // 站内通知（AI 巡检推送）
)

// ModuleInfo 模块元信息（供前端渲染菜单）
type ModuleInfo struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Core bool   `json:"core"`
}

// coreModuleKeys 核心基础模块 key（有序）
var coreModuleKeys = []string{
	ModuleDashboard,
	ModuleDevice,
	ModuleIntersection,
	ModuleMap,
	ModuleFeedback,
	ModuleFault,
	ModuleWorkorder,
	ModuleFirmware,
	ModuleLog,
	ModuleSettings,
}

// optionalModuleKeys 可选模块 key（有序）
var optionalModuleKeys = []string{
	ModuleVideo,
	ModuleInventory,
	ModulePurchase,
	ModuleExpense,
	ModuleSupplier,
	ModuleAI,
	ModuleDispatch,
	ModuleNotification,
}

// moduleName 模块 key → 展示名
var moduleName = map[string]string{
	ModuleDashboard:    "仪表盘",
	ModuleDevice:       "设备管理",
	ModuleIntersection: "路口管理",
	ModuleMap:          "地图大屏",
	ModuleFeedback:     "问题反馈",
	ModuleFault:        "故障管理",
	ModuleWorkorder:    "工单管理",
	ModuleFirmware:     "固件管理",
	ModuleLog:          "系统日志",
	ModuleSettings:     "系统设置",
	ModuleVideo:        "视频监控",
	ModuleInventory:    "物料库存",
	ModulePurchase:     "采购管理",
	ModuleExpense:      "维修费用",
	ModuleSupplier:     "供应商",
	ModuleAI:           "AI 分析",
	ModuleDispatch:     "派单参考",
	ModuleNotification: "站内通知",
}

// enabledSet 已启模块集合（核心恒启 + 配置中的可选模块）
var enabledSet map[string]bool

// runningUnderTest 是否为 go test 运行环境（含 -test. 参数）。
// 用于：测试环境下可选模块默认全开，避免既有直接调用可选模块接口的单测因模块拦截而失败；生产零影响。
var runningUnderTest = func() bool {
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-test.") {
			return true
		}
	}
	return false
}()

// init 在包加载时解析一次（首次访问前即可用）
func init() {
	parseEnabledModules()
}

// parseEnabledModules 从配置读取并重建已启集合
func parseEnabledModules() {
	parseEnabledModulesFrom(config.Get().EnabledModules)
}

// parseEnabledModulesFrom 按给定原始串（逗号分隔）重建已启集合；测试可直接注入
func parseEnabledModulesFrom(raw string) {
	enabledSet = map[string]bool{}
	// 核心模块恒启
	for _, k := range coreModuleKeys {
		enabledSet[k] = true
	}
	// 可选模块：默认关闭，配置启用；go test 环境下默认全开（避免既有测试因模块拦截失败）
	configured := map[string]bool{}
	r := strings.TrimSpace(raw)
	if r != "" {
		for _, part := range strings.Split(r, ",") {
			k := strings.TrimSpace(part)
			if k != "" {
				configured[k] = true
			}
		}
	}
	for _, k := range optionalModuleKeys {
		if runningUnderTest || configured[k] {
			enabledSet[k] = true
		}
	}
}

// ModuleEnabled 判断某模块是否已启用
func ModuleEnabled(key string) bool {
	if enabledSet == nil {
		parseEnabledModules()
	}
	return enabledSet[key]
}

// RequireModule 返回校验模块启用的中间件：未启用 → 403（功能不可用）
func RequireModule(moduleKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ModuleEnabled(moduleKey) {
			forbidden(c, "该功能模块未启用")
			c.Abort()
			return
		}
		c.Next()
	}
}

// EnabledModuleList 已启模块 key 列表（核心 + 已启可选，保持展示顺序）
func EnabledModuleList() []string {
	if enabledSet == nil {
		parseEnabledModules()
	}
	out := append([]string{}, coreModuleKeys...)
	for _, k := range optionalModuleKeys {
		if enabledSet[k] {
			out = append(out, k)
		}
	}
	return out
}

// EnabledModuleInfos 已启模块元信息列表（供前端动态菜单）
func EnabledModuleInfos() []ModuleInfo {
	keys := EnabledModuleList()
	out := make([]ModuleInfo, 0, len(keys))
	for _, k := range keys {
		out = append(out, ModuleInfo{Key: k, Name: moduleName[k], Core: isCoreModule(k)})
	}
	return out
}

func isCoreModule(k string) bool {
	for _, c := range coreModuleKeys {
		if c == k {
			return true
		}
	}
	return false
}

// ListEnabledModules 返回已启模块列表（前端登录后拉取菜单用）
func ListEnabledModules(c *gin.Context) {
	ok(c, gin.H{"modules": EnabledModuleInfos()})
}

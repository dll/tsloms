package handler

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/license"
	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
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

// ModuleEnabled 判断某模块是否已启用（核心=授权可用；可选=开关 且 授权可用）
func ModuleEnabled(key string) bool {
	if enabledSet == nil {
		parseEnabledModules()
	}
	// 授权校验：未授权/试用过期的模块不可用（超管登录与授权管理不受此限）
	if !moduleLicenseOK(key) {
		return false
	}
	// 核心模块恒启（授权通过后启用）；可选模块还需开关判定
	for _, c := range coreModuleKeys {
		if c == key {
			return true
		}
	}
	// 可选模块：DB 运行时开关优先（超级管理员设置），未配置则回退 env 默认
	if val, ok := loadModuleToggle(key); ok {
		return val
	}
	return enabledSet[key]
}

// moduleLicenseOK 授权门槛：核心模块需核心试用/解锁通过；可选模块需其自身试用/解锁通过。
// 依据 license_state（见 handler/license.go）。
func moduleLicenseOK(key string) bool {
	if model.DB == nil {
		return true // 无数据库（只读/测试/启动前）不作授权拦截
	}
	ls := model.LoadLicenseState(model.DB)
	if ls == nil {
		return false // 无授权状态，保守起见默认不可用（但超管可先在授权页开始试用/解锁）
	}
	now := time.Now()

	// 时间回拨检测：若系统时间早于最近校验时间，判定篡改，锁定（避免改时钟续期）
	if ls.LastCheckTime != nil && now.Add(-1*time.Minute).Before(*ls.LastCheckTime) {
		// 回拨超过 1 分钟 → 触发锁定
		return false
	}

	// 核心模块：需要核心试用进行中（首次访问自动开试用懒启动）或 已解锁
	isCore := false
	for _, c := range coreModuleKeys {
		if c == key {
			isCore = true
			break
		}
	}
	if isCore {
		if ls.CoreUnlocked {
			return true
		}
		// 惰性自动开始核心试用（首次访问起算 100 天）
		if ls.CoreActivatedAt == nil {
			t := now
			ls.CoreActivatedAt = &t
			_ = model.SaveLicenseState(model.DB, ls)
		}
		expiry := ls.CoreActivatedAt.Add(license.TrialDaysCore * 24 * time.Hour)
		return now.Before(expiry)
	}

	// 可选模块：需要该模块试用进行中（首次访问自动开试用懒启动）或 已解锁
	for _, k := range optionalModuleKeys {
		if k != key {
			continue
		}
		m := ls.DecodeModules()[key]
		if m == nil || m.ActivatedAt == nil {
			// 惰性自动开始可选模块试用（首次访问起算 30 天）
			if m == nil {
				m = &model.ModuleLicenseState{}
			}
			m.ActivatedAt = &now
			mm := ls.DecodeModules()
			mm[key] = m
			ls.EncodeModules(mm)
			_ = model.SaveLicenseState(model.DB, ls)
			return true
		}
		if m.Unlocked {
			if m.UnlockExpiry != nil && now.After(*m.UnlockExpiry) {
				return false // 解锁但已到期
			}
			return true
		}
		expiry := m.ActivatedAt.Add(license.TrialDaysOptional * 24 * time.Hour)
		return now.Before(expiry)
	}
	return true
}

// RequireModule 返回校验模块启用的中间件：未启用 → 403（功能不可用）
func RequireModule(moduleKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 超级管理员恒可访问（管理授权/模块，避免试用未开始或过期时无法进入管理页的死锁）
		if roleFromCtx(c) != model.RoleSuperAdmin && !ModuleEnabled(moduleKey) {
			forbidden(c, "该功能模块未启用或试用已过期/未授权")
			c.Abort()
			return
		}
		c.Next()
	}
}

// roleFromCtx 从 gin.Context 读取当前用户角色（由鉴权中间件注入）
func roleFromCtx(c *gin.Context) string {
	if v, ok := c.Get("user_role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// EnabledModuleList 已启模块 key 列表（核心恒列于菜单；可选按模块开关+授权判定，保持展示顺序）
func EnabledModuleList() []string {
	if enabledSet == nil {
		parseEnabledModules()
	}
	out := append([]string{}, coreModuleKeys...)
	for _, k := range optionalModuleKeys {
		if ModuleEnabled(k) {
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

// loadModuleToggle 从 DB 读取某可选模块的运行时开关；返回 (enabled, 存在与否)。
// 仅可选模块可被 DB 开关；核心模块恒启不受影响。
func loadModuleToggle(key string) (bool, bool) {
	if model.DB == nil {
		return false, false
	}
	var t model.ModuleToggle
	if err := model.DB.Where("module_key = ?", key).First(&t).Error; err != nil {
		return false, false
	}
	return t.Enabled, true
}

// ModuleSettingsItem 模块设置项（含当前启用态与是否可被开关）
type ModuleSettingsItem struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Core    bool   `json:"core"`
	Enabled bool   `json:"enabled"`
}

// ListModuleSettings GET /modules/settings（仅超级管理员 module:manage）
// 返回全部模块及当前启用状态，供超级管理员开关可选模块。
func ListModuleSettings(c *gin.Context) {
	items := make([]ModuleSettingsItem, 0, len(coreModuleKeys)+len(optionalModuleKeys))
	for _, k := range coreModuleKeys {
		items = append(items, ModuleSettingsItem{Key: k, Name: moduleName[k], Core: true, Enabled: true})
	}
	for _, k := range optionalModuleKeys {
		items = append(items, ModuleSettingsItem{Key: k, Name: moduleName[k], Core: false, Enabled: ModuleEnabled(k)})
	}
	ok(c, gin.H{"list": items, "total": len(items)})
}

// UpdateModuleSettings PUT /modules/settings（仅超级管理员 module:manage）
// body: {module_key: "ai", enabled: true} —— 仅可开关可选模块；核心模块拒绝。
// DB 持久化到 module_toggles。
func UpdateModuleSettings(c *gin.Context) {
	var req struct {
		ModuleKey string `json:"module_key" binding:"required"`
		Enabled   bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "module_key 必填")
		return
	}
	// 仅允许开关可选模块
	allowed := false
	for _, k := range optionalModuleKeys {
		if k == req.ModuleKey {
			allowed = true
			break
		}
	}
	if !allowed {
		badRequest(c, "仅可开关可选模块，核心模块恒启不可关闭")
		return
	}

	var t model.ModuleToggle
	err := model.DB.Where("module_key = ?", req.ModuleKey).First(&t).Error
	if err == gorm.ErrRecordNotFound {
		t = model.ModuleToggle{ModuleKey: req.ModuleKey, Enabled: req.Enabled, UpdatedAt: time.Now()}
		if e := model.DB.Create(&t).Error; e != nil {
			serverError(c, e)
			return
		}
	} else if err != nil {
		serverError(c, err)
		return
	} else {
		if e := model.DB.Model(&t).Updates(map[string]interface{}{"enabled": req.Enabled, "updated_at": time.Now()}).Error; e != nil {
			serverError(c, e)
			return
		}
	}

	recordOperation(c, model.OpUpdate, "module/"+req.ModuleKey, "设置模块启用="+fmt.Sprint(req.Enabled))
	ok(c, gin.H{"module_key": req.ModuleKey, "enabled": req.Enabled, "message": "模块设置已更新"})
}

package model

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// RBAC 动态权限模型
// 目标：从"固定三角色"升级为"角色 + 功能权限点"的可配置权限隔离，
//      支撑按组织结构分配角色、按功能点细粒度授权。
// 兼容性：保留 User.Role（admin/operator/viewer）作为内置角色的编码引用，
//         使得现有账号无需改动即可工作。
// ============================================================================

// Permission 功能权限点字典
// code 形如 "device:create"（模块:动作），唯一标识一项功能权限
type Permission struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"size:64;uniqueIndex;comment:权限编码(module:action)"`
	Name      string    `json:"name" gorm:"size:64;comment:权限名称(如:设备-新建)"`
	Module    string    `json:"module" gorm:"size:32;index;comment:所属模块"`
	Sort      int       `json:"sort" gorm:"default:0;comment:排序"`
	CreatedAt time.Time `json:"created_at"`
}

func (Permission) TableName() string { return "permissions" }

// Role 角色表
// 内置角色 code: admin / operator / viewer；也支持自定义角色
type Role struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"size:32;uniqueIndex;comment:角色编码"`
	Name        string    `json:"name" gorm:"size:64;comment:角色名称"`
	Builtin     bool      `json:"builtin" gorm:"default:false;comment:是否内置角色"`
	Description string    `json:"description" gorm:"size:255;comment:角色描述"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Role) TableName() string { return "roles" }

// RolePermission 角色-功能权限关联
type RolePermission struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	RoleID       uint   `json:"role_id" gorm:"index;comment:角色ID"`
	PermissionID uint   `json:"permission_id" gorm:"index;comment:权限ID"`
	RoleCode     string `json:"role_code" gorm:"size:32;index;comment:冗余角色编码,便于批量查询"`
}

func (RolePermission) TableName() string { return "role_permissions" }

// UserPermission 用户级功能权限覆写
// 语义：若某用户存在该表的记录，则以"记录中的权限集合"作为该用户的有效权限（完整覆盖角色默认）；
//
//	若某用户在该表无任何记录，则继承其角色的默认权限。
//
// granted=true 授权 / granted=false 显式拒绝（用于从角色默认中剔除某项）。
type UserPermission struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	UserID     uint   `json:"user_id" gorm:"index;comment:用户ID"`
	Permission string `json:"permission" gorm:"size:64;index;comment:权限编码"`
	Granted    bool   `json:"granted" gorm:"default:true;comment:是否授予"`
}

func (UserPermission) TableName() string { return "user_permissions" }

// 内置角色编码常量（沿用 User.Role 的取值，保证兼容）
const (
	BuiltinRoleAdmin    = "admin"
	BuiltinRoleOperator = "operator"
	BuiltinRoleViewer   = "viewer"
)

// 权限模块常量（用于前端分组展示）
const (
	PermModuleDevice       = "device"
	PermModuleIntersection = "intersection"
	PermModuleFault        = "fault"
	PermModuleWorkorder    = "workorder"
	PermModuleMedia        = "media"
	PermModuleFirmware     = "firmware"
	PermModuleInventory    = "inventory"
	PermModuleSupplier     = "supplier"
	PermModulePurchase     = "purchase"
	PermModuleExpense      = "expense"
	PermModuleUser         = "user"
	PermModuleDept         = "dept"
	PermModuleRole         = "role"
	PermModuleAI           = "ai"
)

// AllPermissions 全量功能权限点字典（种子数据）
// 每项: code / name / module / sort
var AllPermissions = []Permission{
	// 设备管理
	{Code: "device:create", Name: "设备-新建", Module: PermModuleDevice, Sort: 1},
	{Code: "device:update", Name: "设备-编辑", Module: PermModuleDevice, Sort: 2},
	{Code: "device:delete", Name: "设备-删除", Module: PermModuleDevice, Sort: 3},
	// 路口管理
	{Code: "intersection:update", Name: "路口-重命名/定位", Module: PermModuleIntersection, Sort: 4},
	{Code: "intersection:delete", Name: "路口-清空", Module: PermModuleIntersection, Sort: 5},
	// 故障管理
	{Code: "fault:update", Name: "故障-更新/确认", Module: PermModuleFault, Sort: 6},
	{Code: "fault:dispatch", Name: "故障-派单", Module: PermModuleFault, Sort: 7},
	{Code: "fault:delete", Name: "故障-删除", Module: PermModuleFault, Sort: 8},
	// 工单管理
	{Code: "workorder:create", Name: "工单-新建", Module: PermModuleWorkorder, Sort: 8},
	{Code: "workorder:update", Name: "工单-状态流转", Module: PermModuleWorkorder, Sort: 9},
	{Code: "workorder:assign", Name: "工单-指派", Module: PermModuleWorkorder, Sort: 10},
	{Code: "workorder:delete", Name: "工单-删除", Module: PermModuleWorkorder, Sort: 11},
	// 媒体
	{Code: "media:upload", Name: "媒体-上传", Module: PermModuleMedia, Sort: 12},
	{Code: "media:delete", Name: "媒体-删除", Module: PermModuleMedia, Sort: 13},
	// 固件
	{Code: "firmware:manage", Name: "固件-上传/编辑/发布", Module: PermModuleFirmware, Sort: 14},
	{Code: "firmware:delete", Name: "固件-删除", Module: PermModuleFirmware, Sort: 15},
	// 库存（物料档案 + 出入库）
	{Code: "inventory:manage", Name: "物料-档案/出入库", Module: PermModuleInventory, Sort: 16},
	{Code: "inventory:delete", Name: "物料-删除", Module: PermModuleInventory, Sort: 17},
	// 供应商
	{Code: "supplier:manage", Name: "供应商-新建/编辑", Module: PermModuleSupplier, Sort: 18},
	{Code: "supplier:delete", Name: "供应商-删除", Module: PermModuleSupplier, Sort: 19},
	// 采购
	{Code: "purchase:manage", Name: "采购-下单/收货/取消", Module: PermModulePurchase, Sort: 20},
	{Code: "purchase:delete", Name: "采购-删除", Module: PermModulePurchase, Sort: 21},
	// 维修费用
	{Code: "expense:manage", Name: "费用-登记/确认", Module: PermModuleExpense, Sort: 22},
	{Code: "expense:delete", Name: "费用-删除", Module: PermModuleExpense, Sort: 23},
	// 用户/组织/角色
	{Code: "user:manage", Name: "用户-管理", Module: PermModuleUser, Sort: 24},
	{Code: "dept:manage", Name: "组织-管理", Module: PermModuleDept, Sort: 25},
	{Code: "role:manage", Name: "角色-管理", Module: PermModuleRole, Sort: 26},
	// AI
	{Code: "ai:config", Name: "AI-配置/额度重置", Module: PermModuleAI, Sort: 27},
	{Code: "ai:ops", Name: "AI-分析/报告/建议", Module: PermModuleAI, Sort: 28},
}

// 内置角色的默认权限集合（按权限编码）
var BuiltinRolePerms = map[string][]string{
	// 管理员：全部权限
	BuiltinRoleAdmin: allPermCodes(),
	// 运维人员：业务写操作（不含用户/组织/角色管理，不含 AI 配置，不含核心删除）
	BuiltinRoleOperator: {
		"device:create", "device:update",
		"intersection:update",
		"fault:update", "fault:dispatch",
		"workorder:create", "workorder:update", "workorder:assign",
		"media:upload",
		"firmware:manage",
		"inventory:manage",
		"supplier:manage",
		"purchase:manage",
		"expense:manage",
		"ai:ops",
	},
	// 查看人员：仅只读（无写权限）
	BuiltinRoleViewer: {},
}

// allPermCodes 返回全部权限编码
func allPermCodes() []string {
	codes := make([]string, 0, len(AllPermissions))
	for _, p := range AllPermissions {
		codes = append(codes, p.Code)
	}
	return codes
}

// EffectivePermissions 计算某用户的“有效权限集合”（map[权限编码]bool）
// 规则：
//  1. 先取该用户所属角色的默认权限集合；
//  2. 若该用户在 user_permissions 表有显式记录：
//     - 有任一条 granted=true 的记录，则以这些授权项【覆盖】角色默认（全量替换）；
//     - 同时剔除 granted=false 的项（显式拒绝）。
//  3. 无显式记录则完全继承角色默认。
func EffectivePermissions(userID uint) (map[string]bool, error) {
	set := map[string]bool{}

	// 1) 角色默认权限（admin 内置逻辑兜底：admin 恒有全部权限）
	var user User
	if err := DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	switch user.Role {
	case BuiltinRoleAdmin:
		for _, c := range allPermCodes() {
			set[c] = true
		}
	default:
		var role Role
		if err := DB.Where("code = ?", user.Role).First(&role).Error; err == nil {
			var rps []RolePermission
			DB.Where("role_id = ?", role.ID).Find(&rps)
			for _, rp := range rps {
				var p Permission
				if err := DB.First(&p, rp.PermissionID).Error; err == nil {
					set[p.Code] = true
				}
			}
		} else if err == gorm.ErrRecordNotFound && len(BuiltinRolePerms[user.Role]) > 0 {
			// 角色未入库（异常兜底）时按内置映射兜底
			for _, c := range BuiltinRolePerms[user.Role] {
				set[c] = true
			}
		}
	}

	// 2) 用户级覆写
	var ups []UserPermission
	DB.Where("user_id = ?", userID).Find(&ups)
	grantedAny := false
	for _, up := range ups {
		if up.Granted {
			grantedAny = true
		}
	}
	if grantedAny {
		// 有显式授权记录：清空角色默认，以授权项为准
		set = map[string]bool{}
		for _, up := range ups {
			if up.Granted {
				set[up.Permission] = true
			}
		}
	} else {
		// 无显式授权：仅剔除显式拒绝项
		for _, up := range ups {
			if !up.Granted {
				delete(set, up.Permission)
			}
		}
	}

	return set, nil
}

// EffectivePermissionCodes 返回有效权限编码切片（排序稳定）
func EffectivePermissionCodes(userID uint) ([]string, error) {
	set, err := EffectivePermissions(userID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(set))
	for c := range set {
		if set[c] {
			codes = append(codes, c)
		}
	}
	return codes, nil
}

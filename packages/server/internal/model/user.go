package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户表
// role: admin-管理员 operator-运维人员 viewer-查看人员
// status: enabled-启用 disabled-停用（停用后不可登录）
// 人事核心字段（信号灯维护人员必要）：工号/工作照头像/性别/身份证号/住址/文化程度/工程等级
type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Username string `json:"username" gorm:"uniqueIndex;size:64;comment:用户名"`
	// P0-2 认证改造：手机号登录（可空，未绑定手机号的旧账号不受影响）
	// 注意：不在这两个新列上加 uniqueIndex——旧账号可能为空（NULL），MySQL 唯一索引允许多个 NULL
	//      但为避免空串冲突，仅对非空手机号用普通索引并在应用层做唯一性检查（只增不删）。
	PhoneLogin    string     `json:"phone_login" gorm:"size:20;index;comment:手机号登录账号(可空,绑定后唯一应用层校验)"`
	PhoneVerified bool       `json:"phone_verified" gorm:"default:false;comment:是否已验证手机号"`
	PasswordHash  string     `json:"-" gorm:"size:255;comment:密码哈希(bcrypt)"`
	Role          string     `json:"role" gorm:"size:16;default:viewer;comment:角色(admin/operator/viewer)"`
	RealName      string     `json:"real_name" gorm:"size:64;comment:姓名"`
	Phone         string     `json:"phone" gorm:"size:20;comment:手机号(注册时校验11位格式)"`
	Email         string     `json:"email" gorm:"size:64;comment:邮箱"`
	DepartmentID  *uint      `json:"department_id" gorm:"comment:所属部门ID"`
	Status        string     `json:"status" gorm:"size:16;default:enabled;comment:状态(enabled/disabled)"`
	LastLoginAt   *time.Time `json:"last_login_at" gorm:"comment:最后登录时间"`
	CenterLat     *float64   `json:"center_lat" gorm:"comment:地图中心纬度(该用户管辖区域)"`
	CenterLng     *float64   `json:"center_lng" gorm:"comment:地图中心经度(该用户管辖区域)"`
	Remark        string     `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt     time.Time  `json:"created_at"`
	// ---- 人事核心字段（第二轮补充）----
	WorkNo        string `json:"work_no" gorm:"size:64;index;comment:工号(组织单位编号)"`
	Avatar        string `json:"avatar" gorm:"size:255;comment:工作照/头像(上传图片URL)"`
	Gender        string `json:"gender" gorm:"size:8;comment:性别(male/female)"`
	IDCard        string `json:"id_card" gorm:"size:32;index;comment:身份证号"`
	Address       string `json:"address" gorm:"size:255;comment:住址"`
	Education     string `json:"education" gorm:"size:32;comment:文化程度"`
	EngineerLevel string `json:"engineer_level" gorm:"size:32;comment:工程等级(岗位/技能等级)"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// 用户角色常量
const (
	RoleSuperAdmin = "super_admin" // 超级管理员（模块设置/不对外开放）
	RoleAdmin      = "admin"       // 管理员（维护系统运行，不设模块）
	RoleOperator   = "operator"    // 运维人员（信号灯维护者）
	RoleViewer     = "viewer"      // 查看人员
)

// 用户状态常量
const (
	UserStatusEnabled  = "enabled"
	UserStatusDisabled = "disabled"
)

// Department 部门/组织表
type Department struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64;uniqueIndex;comment:部门名称"`
	ParentID    *uint     `json:"parent_id" gorm:"comment:上级部门ID(0/空为顶级)"`
	Leader      string    `json:"leader" gorm:"size:64;comment:负责人"`
	Description string    `json:"description" gorm:"size:255;comment:部门描述"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 指定表名
func (Department) TableName() string {
	return "departments"
}

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes)
}

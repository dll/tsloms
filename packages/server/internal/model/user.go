package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户表
// role: admin-管理员 operator-运维人员 viewer-查看人员
// status: enabled-启用 disabled-停用（停用后不可登录）
type User struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Username     string     `json:"username" gorm:"uniqueIndex;size:64;comment:用户名"`
	PasswordHash string     `json:"-" gorm:"size:255;comment:密码哈希(bcrypt)"`
	Role         string     `json:"role" gorm:"size:16;default:viewer;comment:角色(admin/operator/viewer)"`
	RealName     string     `json:"real_name" gorm:"size:64;comment:姓名"`
	Phone        string     `json:"phone" gorm:"size:20;comment:手机号"`
	Email        string     `json:"email" gorm:"size:64;comment:邮箱"`
	DepartmentID *uint      `json:"department_id" gorm:"comment:所属部门ID"`
	Status       string     `json:"status" gorm:"size:16;default:enabled;comment:状态(enabled/disabled)"`
	LastLoginAt  *time.Time `json:"last_login_at" gorm:"comment:最后登录时间"`
	CenterLat    *float64   `json:"center_lat" gorm:"comment:地图中心纬度(该用户管辖区域)"`
	CenterLng    *float64   `json:"center_lng" gorm:"comment:地图中心经度(该用户管辖区域)"`
	Remark       string     `json:"remark" gorm:"size:255;comment:备注"`
	CreatedAt    time.Time  `json:"created_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// 用户角色常量
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
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

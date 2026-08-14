package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户表
// role: admin-管理员 operator-运维人员 viewer-查看人员
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64;comment:用户名"`
	PasswordHash string    `json:"-" gorm:"size:255;comment:密码哈希(bcrypt)"`
	Role         string    `json:"role" gorm:"size:16;default:viewer;comment:角色(admin/operator/viewer)"`
	Phone        string    `json:"phone" gorm:"size:20;comment:手机号"`
	CenterLat    *float64  `json:"center_lat" gorm:"comment:地图中心纬度(该用户管辖区域)"`
	CenterLng    *float64  `json:"center_lng" gorm:"comment:地图中心经度(该用户管辖区域)"`
	CreatedAt    time.Time `json:"created_at"`
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

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes)
}

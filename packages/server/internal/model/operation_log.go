package model

import "time"

// OperationLog 系统操作日志表
// 记录用户的登录、增删改等关键操作，用于审计溯源
type OperationLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;comment:操作用户ID"`
	Username  string    `json:"username" gorm:"size:64;comment:操作用户名"`
	Action    string    `json:"action" gorm:"size:64;comment:操作类型(login/logout/create/update/delete)"`
	Target    string    `json:"target" gorm:"size:128;comment:操作对象(如 device/123)"`
	IP        string    `json:"ip" gorm:"size:64;comment:客户端IP"`
	Detail    string    `json:"detail" gorm:"type:text;comment:操作详情"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}

// 操作类型常量
const (
	OpLogin    = "login"
	OpLogout   = "logout"
	OpCreate   = "create"
	OpUpdate   = "update"
	OpDelete   = "delete"
	OpDispatch = "dispatch"
	OpRead     = "read"
)

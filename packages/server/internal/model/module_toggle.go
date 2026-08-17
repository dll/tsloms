package model

import "time"

// ModuleToggle 模块运行时启用/停用开关（仅超级管理员经 /modules/settings 维护；只影响可选模块）
// 与 config.EnabledModules（env 默认）合并：DB 开关优先。
// 基础模块恒启，不可通过此表关闭。
type ModuleToggle struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ModuleKey string    `json:"module_key" gorm:"size:32;uniqueIndex;comment:模块key(video/inventory/ai...)"`
	Enabled   bool      `json:"enabled" gorm:"default:false;comment:是否启用该可选模块"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (ModuleToggle) TableName() string { return "module_toggles" }

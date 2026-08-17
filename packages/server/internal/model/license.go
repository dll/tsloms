package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// LicenseState 授权/试用状态持久化（单行：id=1）
// 字段含义：
//   - CoreActivatedAt   核心功能首次激活时间（试用 100 天起点；NULL=未开始试用）
//   - CoreUnlocked      核心是否已被解锁（超管/授权码）
//   - ModuleJSON        各可选模块激活/解锁状态（见 ModuleLicenseState）
//   - LastCheckTime     最近一次授权校验时间（用于时间回拨检测）
type LicenseState struct {
	ID              uint       `gorm:"primaryKey"`
	CoreActivatedAt *time.Time `json:"core_activated_at"`
	CoreUnlocked    bool       `json:"core_unlocked" gorm:"default:false"`
	ModuleJSON      string     `json:"module_json" gorm:"type:text"`
	LastCheckTime   *time.Time `json:"last_check_time"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ModuleLicenseState 单个可选模块的授权状态
type ModuleLicenseState struct {
	ActivatedAt   *time.Time `json:"activated_at"`    // 试用开始时间（30天起点）
	Unlocked      bool       `json:"unlocked"`        // 是否已解锁
	UnlockExpiry  *time.Time `json:"unlock_expiry"`   // 解锁有效期(NULL=长期)
	UnlockByCode  string     `json:"unlock_by_code"`  // 解锁方式（self=超管/author=授权码）
	LastCheckTime *time.Time `json:"last_check_time"` // 该模块最近校验时间
}

// TableName 指定表名
func (LicenseState) TableName() string { return "license_state" }

// LicenseStateByKey 单例 key=1
const licenseStateID = 1

// LoadLicenseState 读取授权状态（不存在则返回空态；DB 未初始化时也返回空态，避免 nil panic）
func LoadLicenseState(db *gorm.DB) *LicenseState {
	if db == nil {
		return &LicenseState{ID: licenseStateID}
	}
	var ls LicenseState
	if err := db.First(&ls, licenseStateID).Error; err != nil {
		return &LicenseState{ID: licenseStateID}
	}
	return &ls
}

// SaveLicenseState 持久化授权状态
func SaveLicenseState(db *gorm.DB, ls *LicenseState) error {
	if db == nil {
		return nil
	}
	ls.ID = licenseStateID
	ls.UpdatedAt = time.Now()
	if ls.ModuleJSON == "" {
		ls.ModuleJSON = "{}"
	}
	return db.Save(ls).Error
}

// DecodeModules 把 ModuleJSON 解析为 map[string]*ModuleLicenseState
func (ls *LicenseState) DecodeModules() map[string]*ModuleLicenseState {
	out := map[string]*ModuleLicenseState{}
	if ls.ModuleJSON != "" {
		_ = json.Unmarshal([]byte(ls.ModuleJSON), &out)
	}
	if out == nil {
		out = map[string]*ModuleLicenseState{}
	}
	return out
}

// EncodeModules 把 map 序列化为 ModuleJSON
func (ls *LicenseState) EncodeModules(m map[string]*ModuleLicenseState) {
	b, _ := json.Marshal(m)
	ls.ModuleJSON = string(b)
}

// GetModule 获取某模块状态（无则初始化）
func (ls *LicenseState) GetModule(key string) *ModuleLicenseState {
	mm := ls.DecodeModules()
	if _, ok := mm[key]; !ok {
		mm[key] = &ModuleLicenseState{}
	}
	ls.EncodeModules(mm)
	// 读取后把最新序列化写回原 map 变量以便调用方可 Save
	return mm[key]
}

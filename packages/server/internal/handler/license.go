package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/license"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 授权/试用管理（仅超级管理员 module:manage）
// ----------------------------------------------------------------------------
//   · POST /license/trial/start   开始试用（记录核心 100 天 / 可选 30 天起点）
//   · GET  /license/status        查询整体授权状态（核心剩余天数/各模块状态）
//   · POST /license/unlock        解锁：body {module?, code?} — code 为空=超管一键解锁；有=验签授权码
//   · 时间回拨检测：若系统时间相对最近校验时间被回拨超过阈值，判定篡改并锁定业务功能（超管登录保留）
// 授权校验被 ModuleEnabled/RequireModule 复用（见 module.go），对可选模块叠加试用/解锁判定。
// ============================================================================

// licenseStatusItem 单个模块授权状态视图
type licenseStatusItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Core        bool   `json:"core"`
	State       string `json:"state"` // active(可用) / trial(试用中) / expired(已过期) / unlocked(已解锁)
	ActivatedAt string `json:"activated_at,omitempty"`
	TrialExpiry string `json:"trial_expiry,omitempty"`
	UnlockType  string `json:"unlock_type,omitempty"`
	RemainDays  int    `json:"remain_days,omitempty"`
}

// trialStartFor 返回该模块当前应启用的试用状态（惰性记录首次激活）
func ensureModuleTrial(ls *model.LicenseState, key string, now time.Time, days int) {
	m := ls.GetModule(key)
	if m.ActivatedAt == nil {
		m.ActivatedAt = &now
	}
	ls.EncodeModules(ls.DecodeModules())
}

// GetLicenseStatus GET /license/status
func GetLicenseStatus(c *gin.Context) {
	ls := model.LoadLicenseState(model.DB)
	now := time.Now()
	items := make([]licenseStatusItem, 0)

	// 核心
	coreItem := licenseStatusItem{Key: "core", Name: "核心功能", Core: true}
	if ls.CoreActivatedAt == nil {
		coreItem.State = "pending" // 未开始试用
	} else {
		expiry := ls.CoreActivatedAt.Add(license.TrialDaysCore * 24 * time.Hour)
		coreItem.ActivatedAt = license.FormatTrialDate(*ls.CoreActivatedAt)
		coreItem.TrialExpiry = license.FormatTrialDate(expiry)
		switch {
		case ls.CoreUnlocked:
			coreItem.State = "unlocked"
		case now.Before(expiry):
			coreItem.State = "trial"
			coreItem.RemainDays = license.FormatDurationDays(expiry.Sub(now))
		default:
			coreItem.State = "expired"
		}
	}
	items = append(items, coreItem)

	// 可选模块
	for _, k := range license.OptionalModuleKeys {
		it := licenseStatusItem{Key: k, Name: moduleName[k], Core: false}
		m := ls.DecodeModules()[k]
		switch {
		case m == nil || m.ActivatedAt == nil:
			it.State = "pending"
		default:
			expiry := m.ActivatedAt.Add(license.TrialDaysOptional * 24 * time.Hour)
			it.ActivatedAt = license.FormatTrialDate(*m.ActivatedAt)
			it.TrialExpiry = license.FormatTrialDate(expiry)
			switch {
			case m.Unlocked:
				it.State = "unlocked"
				it.UnlockType = m.UnlockByCode
				if m.UnlockExpiry != nil {
					it.TrialExpiry = license.FormatTrialDate(*m.UnlockExpiry)
					it.RemainDays = license.FormatDurationDays(m.UnlockExpiry.Sub(now))
				}
			case now.Before(expiry):
				it.State = "trial"
				it.RemainDays = license.FormatDurationDays(expiry.Sub(now))
			default:
				it.State = "expired"
			}
		}
		items = append(items, it)
	}
	ok(c, gin.H{"list": items, "now": now.Format("2006-01-02 15:04:05")})
}

// StartTrial POST /license/trial/start
// body: {module?: "core" | "video" | ...}（缺省=开始核心试用）
func StartTrial(c *gin.Context) {
	var req struct {
		Module string `json:"module"`
	}
	_ = c.ShouldBindJSON(&req)
	ls := model.LoadLicenseState(model.DB)
	now := time.Now()

	if req.Module == "" || req.Module == "core" {
		if ls.CoreActivatedAt == nil {
			ls.CoreActivatedAt = &now
		}
		if err := model.SaveLicenseState(model.DB, ls); err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, "license/trial/core", "开始核心试用(100天)")
		ok(c, gin.H{"message": "核心功能试用已开始（100 天）", "expiry": license.FormatTrialDate(now.Add(license.TrialDaysCore * 24 * time.Hour))})
		return
	}

	// 可选模块试用
	found := false
	for _, k := range license.OptionalModuleKeys {
		if k == req.Module {
			found = true
			break
		}
	}
	if !found {
		badRequest(c, "未知模块")
		return
	}
	mm := ls.DecodeModules()
	m := mm[req.Module]
	if m == nil {
		m = &model.ModuleLicenseState{}
		mm[req.Module] = m
	}
	if m.ActivatedAt == nil {
		m.ActivatedAt = &now
	}
	ls.EncodeModules(mm)
	if err := model.SaveLicenseState(model.DB, ls); err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "license/trial/"+req.Module, "开始可选模块试用(30天)")
	ok(c, gin.H{"message": "模块试用已开始（30 天）", "module": req.Module, "expiry": license.FormatTrialDate(now.Add(license.TrialDaysOptional * 24 * time.Hour))})
}

// Unlock License POST /license/unlock
// body: {module?: "core"|"video"|..., code?: "授权码"} — code 空=超管一键解锁(长期)；有=验签授权码解锁。
func UnlockLicense(c *gin.Context) {
	var req struct {
		Module string `json:"module"`
		Code   string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if req.Module == "" {
		badRequest(c, "module 必填（core 或模块 key）")
		return
	}
	ls := model.LoadLicenseState(model.DB)
	now := time.Now()

	// 核心解锁
	if req.Module == "core" {
		if req.Code != "" {
			valid, verr := license.VerifyUnlockCode(req.Code, "core", now)
			if verr != nil || !valid {
				unauthorized(c, "授权码无效："+unlockErrMsg(verr))
				return
			}
		}
		ls.CoreUnlocked = true
		if err := model.SaveLicenseState(model.DB, ls); err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, "license/unlock/core", "核心功能解锁")
		ok(c, gin.H{"message": "核心功能已解锁", "module": "core"})
		return
	}

	// 可选模块解锁
	found := false
	for _, k := range license.OptionalModuleKeys {
		if k == req.Module {
			found = true
			break
		}
	}
	if !found {
		badRequest(c, "未知模块")
		return
	}
	mm := ls.DecodeModules()
	m := mm[req.Module]
	if m == nil {
		m = &model.ModuleLicenseState{}
		mm[req.Module] = m
	}
	if req.Code != "" {
		valid, verr := license.VerifyUnlockCode(req.Code, req.Module, now)
		if verr != nil || !valid {
			unauthorized(c, "授权码无效："+unlockErrMsg(verr))
			return
		}
		m.UnlockByCode = "author"
		// 授权码仅提供长期/覆盖：这里按“批准使用”处理，不设到期（长期）
		m.UnlockExpiry = nil
	} else {
		// 超管一键解锁：长期
		m.UnlockByCode = "self"
		m.UnlockExpiry = nil
	}
	m.Unlocked = true
	ls.EncodeModules(mm)
	if err := model.SaveLicenseState(model.DB, ls); err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "license/unlock/"+req.Module, "模块解锁("+m.UnlockByCode+")")
	ok(c, gin.H{"message": "模块已解锁", "module": req.Module, "by": m.UnlockByCode})
}

func unlockErrMsg(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

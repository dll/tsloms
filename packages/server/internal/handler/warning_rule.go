package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-3 预警配置（忽略规则）CRUD
// ----------------------------------------------------------------------------
// GET    /warning-rules          列表（分页+过滤）
// POST   /warning-rules          新增
// PUT    /warning-rules/:id      编辑
// DELETE /warning-rules/:id      删除
// POST   /warnings/auto-ignore   依据规则自动忽略（可选，维护/预览用）
// ============================================================================

// ListWarningRules GET /warning-rules
func ListWarningRules(c *gin.Context) {
	page, pageSize := paginate(c)
	q := model.DB.Model(&model.WarningRule{})

	if name := c.Query("name"); name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	if enabled := c.Query("enabled"); enabled != "" {
		q = q.Where("enabled = ?", enabled == "true")
	}

	var total int64
	q.Count(&total)

	var list []model.WarningRule
	q.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// warningRuleRequest 规则请求体（共用）
type warningRuleRequest struct {
	Name          string  `json:"name"`
	CrossingID    *uint   `json:"crossing_id"`
	DeviceHwID    *string `json:"device_hw_id"`
	WarningCode   *int    `json:"warning_code"`
	Level         string  `json:"level"`
	EffectiveType string  `json:"effective_type"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	Action        string  `json:"action"`
	Enabled       *bool   `json:"enabled"`
	Remark        string  `json:"remark"`
}

func applyWarningRule(rule *model.WarningRule, req *warningRuleRequest) {
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.CrossingID != nil {
		rule.CrossingID = req.CrossingID
	}
	if req.DeviceHwID != nil {
		rule.DeviceHwID = req.DeviceHwID
	}
	if req.WarningCode != nil {
		rule.WarningCode = req.WarningCode
	}
	if req.Level != "" {
		rule.Level = req.Level
	}
	if req.EffectiveType != "" {
		rule.EffectiveType = req.EffectiveType
	}
	if req.StartTime != "" {
		rule.StartTime = req.StartTime
	}
	if req.EndTime != "" {
		rule.EndTime = req.EndTime
	}
	if req.Action != "" {
		rule.Action = req.Action
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Remark != "" {
		rule.Remark = req.Remark
	}
}

// CreateWarningRule POST /warning-rules
func CreateWarningRule(c *gin.Context) {
	var req warningRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	rule := model.WarningRule{
		EffectiveType: model.RuleEffectivePermanent,
		Action:        model.RuleActionIgnore,
		Enabled:       true,
	}
	applyWarningRule(&rule, &req)
	if rule.Name == "" {
		badRequest(c, "规则名称必填")
		return
	}
	if err := model.DB.Create(&rule).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, "warning-rule/"+utoa(rule.ID), "新增预警忽略规则")
	ok(c, gin.H{"rule": rule, "message": "规则已新增"})
}

// UpdateWarningRule PUT /warning-rules/:id
func UpdateWarningRule(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "规则ID无效")
		return
	}
	var req warningRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var rule model.WarningRule
	if err := model.DB.First(&rule, id).Error; err != nil {
		notFound(c, "规则不存在")
		return
	}
	applyWarningRule(&rule, &req)
	if err := model.DB.Model(&rule).Updates(map[string]interface{}{
		"name": rule.Name, "crossing_id": rule.CrossingID, "device_hw_id": rule.DeviceHwID,
		"warning_code": rule.WarningCode, "level": rule.Level, "effective_type": rule.EffectiveType,
		"start_time": rule.StartTime, "end_time": rule.EndTime, "action": rule.Action,
		"enabled": rule.Enabled, "remark": rule.Remark,
	}).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "warning-rule/"+utoa(rule.ID), "更新预警忽略规则")
	ok(c, gin.H{"rule": rule, "message": "规则已更新"})
}

// DeleteWarningRule DELETE /warning-rules/:id
func DeleteWarningRule(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "规则ID无效")
		return
	}
	rule := model.WarningRule{ID: id}
	if err := model.DB.First(&rule, id).Error; err != nil {
		notFound(c, "规则不存在")
		return
	}
	if err := model.DB.Delete(&rule).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, "warning-rule/"+utoa(id), "删除预警忽略规则")
	ok(c, gin.H{"message": "规则已删除"})
}

// AutoIgnoreWarnings POST /warnings/auto-ignore
// 依据全部启用规则，把「未处理」且命中规则的预警自动忽略。
// 请求可选过滤：仅处理指定预警ID（ids）或全部未处理。
func AutoIgnoreWarnings(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)

	var rules []model.WarningRule
	if err := model.DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		serverError(c, err)
		return
	}
	q := model.DB.Model(&model.Warning{}).Where("deal_state = ?", model.WarningDealUnhandled)
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	var warnings []model.Warning
	q.Find(&warnings)

	ignored := 0
	now := time.Now()
	for i := range warnings {
		for r := range rules {
			// 注意 index 捕获：rules 是值切片，用 rules[r] 的指针需要注意——先拷贝
			rule := rules[r]
			if rule.Matches(&warnings[i]) {
				model.DB.Model(&warnings[i]).Updates(map[string]interface{}{
					"deal_state":    model.WarningDealIgnored,
					"ignore_reason": "自动忽略规则 [" + rule.Name + "]",
					"resolved_at":   &now,
				})
				ignored++
				break
			}
		}
	}
	ok(c, gin.H{"message": "自动忽略完成", "affected": ignored})
}

// utoa 便捷转换（uint → 十进制字符串）
func utoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ExpenseStats 费用统计
func ExpenseStats(c *gin.Context) {
	var total float64
	var count int64
	var materialSum, laborSum, trafficSum, otherSum float64

	model.DB.Model(&model.RepairExpense{}).
		Select("COALESCE(SUM(amount),0)").Scan(&total)
	model.DB.Model(&model.RepairExpense{}).Count(&count)

	for _, t := range []struct {
		typ string
		dst *float64
	}{
		{model.ExpenseTypeMaterial, &materialSum},
		{model.ExpenseTypeLabor, &laborSum},
		{model.ExpenseTypeTraffic, &trafficSum},
		{model.ExpenseTypeOther, &otherSum},
	} {
		model.DB.Model(&model.RepairExpense{}).Where("type = ?", t.typ).
			Select("COALESCE(SUM(amount),0)").Scan(t.dst)
	}

	// 按费用的工单查找对应故障状态/设备：设备累计维修成本 TOP
	type row struct {
		DeviceHwID uint32
		Sum        float64
	}
	var topDev []row
	model.DB.Model(&model.RepairExpense{}).Where("device_hw_id > 0").
		Select("device_hw_id, SUM(amount) AS sum").
		Group("device_hw_id").Order("sum DESC").Limit(10).Scan(&topDev)

	devTop := make([]gin.H, 0, len(topDev))
	for _, d := range topDev {
		devTop = append(devTop, gin.H{"device_hw_id": d.DeviceHwID, "total": d.Sum})
	}

	ok(c, gin.H{
		"total_amount": total, "total_count": count,
		"material": materialSum, "labor": laborSum, "traffic": trafficSum, "other": otherSum,
		"top_devices": devTop,
	})
}

// ListRepairExpenses 维修费用列表（分页，可按设备/工单/类型/日期筛选）
func ListRepairExpenses(c *gin.Context) {
	page, pageSize := paginate(c)
	query := model.DB.Model(&model.RepairExpense{})

	if hw := c.Query("device_hw_id"); hw != "" {
		query = query.Where("device_hw_id = ?", hw)
	}
	if wo := c.Query("work_order_id"); wo != "" {
		query = query.Where("work_order_id = ?", wo)
	}
	if typ := c.Query("type"); typ != "" {
		query = query.Where("type = ?", typ)
	}
	if from := c.Query("from"); from != "" {
		query = query.Where("work_date >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		query = query.Where("work_date <= ?", to+" 23:59:59")
	}
	if conf := c.Query("confirmed"); conf == "true" || conf == "1" {
		query = query.Where("confirmed = ?", true)
	}

	var total int64
	query.Count(&total)
	var list []model.RepairExpense
	query.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// SaveRepairExpense 新增/更新维修费用
// body: type, amount, device_hw_id?, work_order_id?, description, work_date, note
func SaveRepairExpense(c *gin.Context) {
	var req struct {
		ID          *uint     `json:"id"`
		Type        string    `json:"type" binding:"required"`
		Amount      float64   `json:"amount" binding:"required"`
		DeviceHwID  uint32    `json:"device_hw_id"`
		WorkOrderID *uint     `json:"work_order_id"`
		Description string    `json:"description"`
		WorkDate    *string   `json:"work_date"` // yyyy-MM-dd
		Confirmed   bool      `json:"confirmed"`
		Note        string    `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（type、amount 必填）")
		return
	}
	switch req.Type {
	case model.ExpenseTypeMaterial, model.ExpenseTypeLabor, model.ExpenseTypeTraffic, model.ExpenseTypeOther:
	default:
		badRequest(c, "费用类型不合法")
		return
	}
	if req.Amount < 0 {
		badRequest(c, "费用金额不能为负")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	// 解析发生日期
	var workDate *time.Time
	if req.WorkDate != nil && *req.WorkDate != "" {
		t, err := time.Parse("2006-01-02", *req.WorkDate)
		if err != nil {
			badRequest(c, "日期格式应为 yyyy-MM-dd")
			return
		}
		workDate = &t
	}

	// 关联工单时，校验工单存在并自动带出设备ID
	if req.WorkOrderID != nil {
		var wo model.WorkOrder
		if err := model.DB.First(&wo, *req.WorkOrderID).Error; err != nil {
			badRequest(c, "关联工单不存在")
			return
		}
		if req.DeviceHwID == 0 {
			req.DeviceHwID = wo.DeviceHwID
		}
	}

	if req.ID != nil && *req.ID > 0 {
		var e model.RepairExpense
		if err := model.DB.First(&e, *req.ID).Error; err != nil {
			notFound(c, "费用记录不存在")
			return
		}
		updates := map[string]interface{}{
			"type": req.Type, "amount": req.Amount, "device_hw_id": req.DeviceHwID,
			"work_order_id": req.WorkOrderID, "description": req.Description,
			"work_date": workDate, "confirmed": req.Confirmed, "note": req.Note,
		}
		if err := model.DB.Model(&e).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, fmt.Sprintf("expense/%d", e.ID), "更新维修费用")
		ok(c, gin.H{"message": "费用已更新"})
		return
	}

	expenseNo := model.NextBizNoCol(model.DB, "repair_expenses", "expense_no", "FE")
	e := model.RepairExpense{
		ExpenseNo: expenseNo, Type: req.Type, Amount: req.Amount, DeviceHwID: req.DeviceHwID,
		WorkOrderID: req.WorkOrderID, Description: req.Description,
		WorkDate: workDate, Operator: operator, Confirmed: req.Confirmed, Note: req.Note,
	}
	if err := model.DB.Create(&e).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("expense/%d", e.ID), "新增维修费用 "+e.ExpenseNo)
	ok(c, gin.H{"expense": e, "message": "费用已新增"})
}

// ConfirmRepairExpense 确认/取消确认费用入账
func ConfirmRepairExpense(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "费用ID无效")
		return
	}
	var req struct {
		Confirmed bool `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var e model.RepairExpense
	if err := model.DB.First(&e, id).Error; err != nil {
		notFound(c, "费用记录不存在")
		return
	}
	if err := model.DB.Model(&e).Update("confirmed", req.Confirmed).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("expense/%d", id), "确认维修费用入账")
	ok(c, gin.H{"message": "操作成功"})
}

// DeleteRepairExpense 删除维修费用
func DeleteRepairExpense(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "费用ID无效")
		return
	}
	var e model.RepairExpense
	if err := model.DB.First(&e, id).Error; err != nil {
		notFound(c, "费用记录不存在")
		return
	}
	if err := model.DB.Delete(&e).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("expense/%d", id), "删除维修费用 "+e.ExpenseNo)
	ok(c, gin.H{"message": "删除成功"})
}

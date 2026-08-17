package handler

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
)

// ============================================================================
// P0-3 预警管理
// ----------------------------------------------------------------------------
// - GET  /warnings                      预警记录列表（分页+过滤）
// - GET  /warnings/:id                  预警详情
// - POST /warnings/:id/ignore           单条忽略
// - POST /warnings/batch-ignore         批量忽略
// - POST /warnings/:id/to-workorder     转工单（复用既有 workorder 创建逻辑，不破坏工单/去重红线）
// - GET  /warnings/export               导出 CSV
// - GET/POST/PUT/DELETE /warning-rules  预警配置（忽略规则）CRUD
//
// 边界：预警独立于 FaultRecord（故障识别-派单链路）；转工单走 model.EnsureActiveWorkOrder，
//      不与其 30min 去重/NextOrderNo/SLA 冲突（预警可无关联 fault，用独立占位处理）。
// ============================================================================

// warningView 预警视图：附带 路口名/设备路口/来源 冗余展示
func warningView(w *model.Warning) gin.H {
	v := gin.H{
		"id": w.ID, "device_hw_id": w.DeviceHwID, "crossing_id": w.CrossingID,
		"equipment_uuid": w.EquipmentUUID, "warning_code": w.WarningCode,
		"warning_label": w.WarningLabel, "level": w.Level, "func": w.Func,
		"source": w.Source, "deal_state": w.DealState, "status": w.Status,
		"fault_id": w.FaultID, "work_order_id": w.WorkOrderID,
		"ignore_reason": w.IgnoreReason, "occurred_at": w.OccurredAt,
		"resolved_at": w.ResolvedAt, "remark": w.Remark,
		"created_at": w.CreatedAt, "updated_at": w.UpdatedAt,
	}
	// 路口名 + 设备路口（冗余展示）
	var crossing model.Crossing
	if w.CrossingID != nil && model.DB.First(&crossing, *w.CrossingID).Error == nil {
		v["crossing_name"] = crossing.Name
		v["road_name"] = crossing.RoadName
	}
	var device model.Device
	if model.DB.Where("hw_id = ?", w.DeviceHwID).First(&device).Error == nil {
		v["intersection"] = device.Intersection
	}
	return v
}

// buildWarningQuery 从 gin.Context 解析预警过滤参数，返回基础 query（不分页）。
func buildWarningQuery(c *gin.Context) *gorm.DB {
	q := model.DB.Model(&model.Warning{})
	if crossingID := c.Query("crossing_id"); crossingID != "" {
		if v, err := strconv.ParseUint(crossingID, 10, 64); err == nil {
			q = q.Where("crossing_id = ?", v)
		}
	}
	if hw := c.Query("device_hw_id"); hw != "" {
		if v, err := strconv.ParseUint(hw, 10, 64); err == nil {
			q = q.Where("device_hw_id = ?", v)
		}
	}
	if level := c.Query("level"); level != "" {
		q = q.Where("level = ?", level)
	}
	if code := c.Query("warning_code"); code != "" {
		if v, err := strconv.Atoi(code); err == nil {
			q = q.Where("warning_code = ?", v)
		}
	}
	if dealState := c.Query("deal_state"); dealState != "" {
		q = q.Where("deal_state = ?", dealState)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if source := c.Query("source"); source != "" {
		q = q.Where("source = ?", source)
	}
	if start := c.Query("start_time"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			q = q.Where("occurred_at >= ?", t)
		}
	}
	if end := c.Query("end_time"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			q = q.Where("occurred_at <= ?", t.Add(24*time.Hour))
		}
	}
	return q
}

// ListWarnings GET /warnings
// 过滤：crossing_id / device_hw_id / level / warning_code / deal_state / status / source / 时间范围
func ListWarnings(c *gin.Context) {
	page, pageSize := paginate(c)
	q := buildWarningQuery(c)

	var total int64
	q.Count(&total)

	var list []model.Warning
	q.Order("occurred_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	view := make([]gin.H, 0, len(list))
	for i := range list {
		view = append(view, warningView(&list[i]))
	}
	ok(c, gin.H{"list": view, "total": total, "page": page, "page_size": pageSize})
}

// GetWarning GET /warnings/:id
func GetWarning(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "预警ID无效")
		return
	}
	var w model.Warning
	if err := model.DB.First(&w, id).Error; err != nil {
		notFound(c, "预警记录不存在")
		return
	}
	ok(c, gin.H{"warning": warningView(&w)})
}

// IgnoreWarning POST /warnings/:id/ignore
// 请求: {reason?: "忽略原因"}
func IgnoreWarning(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "预警ID无效")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	var w model.Warning
	if err := model.DB.First(&w, id).Error; err != nil {
		notFound(c, "预警记录不存在")
		return
	}
	now := time.Now()
	if err := model.DB.Model(&w).Updates(map[string]interface{}{
		"deal_state":    model.WarningDealIgnored,
		"ignore_reason": req.Reason,
		"resolved_at":   &now,
	}).Error; err != nil {
		serverError(c, err)
		return
	}
	model.DB.First(&w, id)
	recordOperation(c, model.OpUpdate, fmt.Sprintf("warning/%d", w.ID), "忽略预警")
	ok(c, gin.H{"warning": warningView(&w), "message": "预警已忽略"})
}

// BatchIgnoreWarnings POST /warnings/batch-ignore
// 请求: {ids: [1,2,3], reason?: "批量忽略原因"}
func BatchIgnoreWarnings(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "ids 必填")
		return
	}
	if len(req.IDs) == 0 {
		badRequest(c, "未选择预警")
		return
	}
	now := time.Now()
	res := model.DB.Model(&model.Warning{}).
		Where("id IN ? AND deal_state = ?", req.IDs, model.WarningDealUnhandled).
		Updates(map[string]interface{}{
			"deal_state":    model.WarningDealIgnored,
			"ignore_reason": req.Reason,
			"resolved_at":   &now,
		})
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	recordOperation(c, model.OpUpdate, "warning/batch-ignore", fmt.Sprintf("批量忽略预警 %d 条", res.RowsAffected))
	ok(c, gin.H{"message": "批量忽略成功", "affected": res.RowsAffected})
}

// WarningToWorkOrder POST /warnings/:id/to-workorder
// 请求: {remark?}  转工单（复用既有 WorkOrder 创建，走 model.EnsureActiveWorkOrder 防重）
//
// 预警可能无关联 fault_id（独立预警源）。为保证与既有工单红线（NextOrderNo + 活跃唯一 + SLA）
// 兼容，这里为无 fault 的预警建立一个「预占位」工作单语义：
//   - 若预警已转工单（status=transferred）则拒绝重复转单；
//   - 工单 device_hw_id 取预警设备；FaultID 取预警.fault_id（可能为 0/无）。
func WarningToWorkOrder(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "预警ID无效")
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&req)

	var w model.Warning
	if err := model.DB.First(&w, id).Error; err != nil {
		notFound(c, "预警记录不存在")
		return
	}
	if w.Status == model.WarningTransferred && w.WorkOrderID != nil {
		badRequest(c, "该预警已转工单，请勿重复转单")
		return
	}

	// 复用工单创建：确保活跃唯一（M1 语义）
	var wo *model.WorkOrder
	if w.FaultID != nil && *w.FaultID > 0 {
		wo = model.EnsureActiveWorkOrder(model.DB, *w.FaultID, w.DeviceHwID)
	} else {
		// 无来源故障：为其建独立活跃工单，并为该预警建立唯一占位语义。
		// 复用 EnsureActiveWorkOrder 需 fault_id>0 才有效；这里手建一张独立工单。
		wo = createStandaloneWarningWorkOrder(w.DeviceHwID, req.Remark)
	}
	if wo == nil {
		serverError(c, fmt.Errorf("工单创建失败"))
		return
	}

	// 回写预警：已转 + 关联工单 + 处理状态
	now := time.Now()
	model.DB.Model(&w).Updates(map[string]interface{}{
		"status":        model.WarningTransferred,
		"work_order_id": wo.ID,
		"deal_state":    model.WarningDealResolved,
		"resolved_at":   &now,
		"remark":        req.Remark,
	})
	model.DB.First(&w, id)
	recordOperation(c, model.OpCreate, fmt.Sprintf("warning/%d/to-workorder", w.ID), "预警转工单 "+wo.OrderNo)
	ok(c, gin.H{"warning": warningView(&w), "work_order": workOrderView(*wo), "message": "已转工单 " + wo.OrderNo})
}

// createStandaloneWarningWorkOrder 为「无来源故障」的预警创建独立活跃工单。
// 使用与既有工单相同的 NextOrderNo 与活跃唯一语义：FaultID=0（占位，不关联故障），
// 保证不与既有故障工单去重逻辑冲突。
func createStandaloneWarningWorkOrder(deviceHwID uint32, remark string) *model.WorkOrder {
	// 占位 fault_id=0；活跃唯一约束依赖 fault_active_scope。由于 fault_id=0 无唯一位，
	// 多条此类工单允许存在（符合历史占位语义，不影响既有 fault 派单唯一）。
	wo := &model.WorkOrder{
		OrderNo:          model.NextOrderNo(model.DB),
		FaultID:          0,
		DeviceHwID:       deviceHwID,
		Status:           model.WorkOrderStatusPending,
		Result:           remark,
		FaultActiveScope: nil, // 无来源故障：不占 fault 活跃位(NULL 不参与唯一索引，允许多条并存)
	}
	if err := model.DB.Create(wo).Error; err != nil {
		return nil
	}
	return wo
}

// ExportWarnings GET /warnings/export
// 导出当前过滤条件内的预警为 CSV（不区分页，最多导 5000 条）
func ExportWarnings(c *gin.Context) {
	// 复用列表过滤逻辑（非分页）
	exportQuery := buildWarningQuery(c)

	var list []model.Warning
	exportQuery.Order("occurred_at DESC").Limit(5000).Find(&list)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=warnings_"+time.Now().Format("20060102")+".csv")
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	// UTF-8 BOM 便于 Excel 识别中文
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	_ = writer.Write([]string{"ID", "设备硬件ID", "路口ID", "告警码", "告警内容", "级别", "来源", "处理状态", "工单状态", "发生时间"})
	for i := range list {
		w := &list[i]
		_ = writer.Write([]string{
			strconv.FormatUint(uint64(w.ID), 10),
			strconv.FormatUint(uint64(w.DeviceHwID), 10),
			fmt.Sprintf("%v", nilUint(w.CrossingID)),
			strconv.Itoa(w.WarningCode),
			w.WarningLabel,
			w.Level,
			w.Source,
			w.DealState,
			w.Status,
			w.OccurredAt.Format("2006-01-02 15:04:05"),
		})
	}
}

func nilUint(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

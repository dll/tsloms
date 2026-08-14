package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListWorkOrders 工单列表查询
// 支持按设备、状态、处理人、时间范围筛选，分页查询
func ListWorkOrders(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	query := model.DB.Model(&model.WorkOrder{})

	// 按设备硬件 ID 筛选
	if hwID := c.Query("hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}

	// 按状态筛选
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// 按处理人筛选
	if assigneeID := c.Query("assignee_id"); assigneeID != "" {
		query = query.Where("assignee_id = ?", assigneeID)
	}

	// 按工单编号筛选
	if orderNo := c.Query("order_no"); orderNo != "" {
		query = query.Where("order_no LIKE ?", "%"+orderNo+"%")
	}

	// 按时间范围筛选
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse("2006-01-02", startTime); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse("2006-01-02", endTime); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	// 总数
	var total int64
	query.Count(&total)

	// 分页查询
	var orders []model.WorkOrder
	query.Order("created_at DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&orders)

	// 关联处理人姓名
	list := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		item := workOrderView(o)
		list = append(list, item)
	}

	ok(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// workOrderView 工单视图：附带处理人姓名
func workOrderView(o model.WorkOrder) gin.H {
	assigneeName := ""
	if o.AssigneeID != nil {
		var u model.User
		if err := model.DB.Select("id, username").First(&u, *o.AssigneeID).Error; err == nil {
			assigneeName = u.Username
		}
	}
	return gin.H{
		"id": o.ID, "order_no": o.OrderNo, "fault_id": o.FaultID,
		"device_hw_id": o.DeviceHwID, "status": o.Status,
		"assignee_id": o.AssigneeID, "assignee_name": assigneeName,
		"result": o.Result, "created_at": o.CreatedAt, "closed_at": o.ClosedAt,
	}
}

// CreateWorkOrder 手动创建工单
// 运维人员/管理员可手动创建工单，关联故障记录
func CreateWorkOrder(c *gin.Context) {
	var req struct {
		FaultID    uint   `json:"fault_id" binding:"required"`
		DeviceHwID uint32 `json:"device_hw_id" binding:"required"`
		AssigneeID *uint  `json:"assignee_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	// 验证故障记录是否存在
	var fault model.FaultRecord
	if err := model.DB.First(&fault, req.FaultID).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	// 校验处理人存在且为运维/管理员
	if req.AssigneeID != nil {
		var u model.User
		if err := model.DB.First(&u, *req.AssigneeID).Error; err != nil {
			notFound(c, "处理人不存在")
			return
		}
		if u.Role != model.RoleAdmin && u.Role != model.RoleOperator {
			badRequest(c, "只能指派给运维人员或管理员")
			return
		}
	}

	// 生成工单编号：WO{yyyyMMdd}{4位自增序号}
	orderNo := model.NextOrderNo(model.DB)

	wo := model.WorkOrder{
		OrderNo:    orderNo,
		FaultID:    req.FaultID,
		DeviceHwID: req.DeviceHwID,
		Status:     model.WorkOrderStatusPending,
		AssigneeID: req.AssigneeID,
	}

	if err := model.DB.Create(&wo).Error; err != nil {
		serverError(c, err)
		return
	}

	// 关联故障记录的工单 ID
	model.DB.Model(&fault).Update("work_order_id", wo.ID)

	recordOperation(c, model.OpCreate, fmt.Sprintf("work-order/%d", wo.ID), "创建维修工单")

	ok(c, gin.H{"work_order": wo, "message": "工单创建成功"})
}

// UpdateWorkOrderStatus 更新工单状态
// 状态流转：pending → processing → completed/rejected
func UpdateWorkOrderStatus(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "工单ID无效")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
		Result string `json:"result"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	// 验证状态值
	validStatuses := map[string]bool{
		model.WorkOrderStatusPending:    true,
		model.WorkOrderStatusProcessing: true,
		model.WorkOrderStatusCompleted:  true,
		model.WorkOrderStatusRejected:   true,
	}
	if !validStatuses[req.Status] {
		badRequest(c, "无效的工单状态")
		return
	}

	var wo model.WorkOrder
	if err := model.DB.First(&wo, id).Error; err != nil {
		notFound(c, "工单不存在")
		return
	}

	updates := map[string]interface{}{
		"status": req.Status,
	}
	if req.Result != "" {
		updates["result"] = req.Result
	}

	// 工单完成时记录闭环时间，并将关联故障标记为已解决
	if req.Status == model.WorkOrderStatusCompleted {
		now := time.Now()
		updates["closed_at"] = &now
		model.DB.Model(&model.FaultRecord{}).
			Where("id = ?", wo.FaultID).
			Updates(map[string]interface{}{
				"status":    "resolved",
				"last_seen": now,
			})
	}

	// 工单驳回后重新派发（rejected → pending）：清空关闭时间
	if req.Status == model.WorkOrderStatusPending && wo.Status == model.WorkOrderStatusRejected {
		updates["closed_at"] = nil
	}

	if err := model.DB.Model(&wo).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpUpdate, fmt.Sprintf("work-order/%d", wo.ID), "更新工单状态为 "+req.Status)

	ok(c, gin.H{"work_order": wo, "message": "工单状态更新成功"})
}

// AssignWorkOrder 派单：指派/更换工单处理人（管理员/运维）
// 派单后工单进入处理中（processing）
func AssignWorkOrder(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "工单ID无效")
		return
	}

	var req struct {
		AssigneeID uint `json:"assignee_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请选择维修人员")
		return
	}

	// 校验处理人存在且为运维/管理员
	var u model.User
	if err := model.DB.First(&u, req.AssigneeID).Error; err != nil {
		notFound(c, "处理人不存在")
		return
	}
	if u.Role != model.RoleAdmin && u.Role != model.RoleOperator {
		badRequest(c, "只能指派给运维人员或管理员")
		return
	}

	var wo model.WorkOrder
	if err := model.DB.First(&wo, id).Error; err != nil {
		notFound(c, "工单不存在")
		return
	}

	updates := map[string]interface{}{
		"assignee_id": req.AssigneeID,
	}
	// 待处理→派单后进入处理中
	if wo.Status == model.WorkOrderStatusPending {
		updates["status"] = model.WorkOrderStatusProcessing
	}
	if err := model.DB.Model(&wo).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpDispatch, fmt.Sprintf("work-order/%d", wo.ID), "派单给用户"+u.Username)
	ok(c, gin.H{"work_order": wo, "message": "派单成功（已指派给 " + u.Username + "）"})
}

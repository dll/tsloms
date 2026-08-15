package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListFaults 故障记录列表查询
// 支持按设备、状态、故障类型、时间范围筛选，分页查询
func ListFaults(c *gin.Context) {
	page, pageSize := paginate(c)

	query := model.DB.Model(&model.FaultRecord{})

	// 按设备硬件 ID 筛选
	if hwID := c.Query("hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}

	// 按状态筛选（active 兼容旧语义 = 未解决：occurred/confirmed/dispatched）
	if status := c.Query("status"); status != "" {
		if status == "active" {
			query = query.Where("status IN ?", []string{
				model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
			})
		} else {
			query = query.Where("status = ?", status)
		}
	}

	// 按故障类型筛选
	if faultType := c.Query("fault_type"); faultType != "" {
		query = query.Where("fault_type = ?", faultType)
	}

	// 按故障等级筛选
	if faultLevel := c.Query("fault_level"); faultLevel != "" {
		query = query.Where("fault_level = ?", faultLevel)
	}

	// 按时间范围筛选（兼容 start_time/end_time 与 start_date/end_date 两套参数名）
	startTime := c.Query("start_time")
	if startTime == "" {
		startTime = c.Query("start_date")
	}
	if startTime != "" {
		if t, err := time.Parse("2006-01-02", startTime); err == nil {
			query = query.Where("first_seen >= ?", t)
		}
	}
	endTime := c.Query("end_time")
	if endTime == "" {
		endTime = c.Query("end_date")
	}
	if endTime != "" {
		if t, err := time.Parse("2006-01-02", endTime); err == nil {
			query = query.Where("last_seen <= ?", t.Add(24*time.Hour))
		}
	}

	// 总数
	var total int64
	query.Count(&total)

	// 分页查询
	var faults []model.FaultRecord
	query.Order("last_seen DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&faults)

	// 转为带负责人/维修人姓名的视图
	list := make([]gin.H, 0, len(faults))
	for i := range faults {
		list = append(list, faultView(c, faults[i]))
	}

	ok(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetFault 获取单个故障记录详情
func GetFault(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}

	var fault model.FaultRecord
	if err := model.DB.First(&fault, id).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	// 关联设备信息（路口/经纬度/在线状态）
	dev := gin.H{}
	var device model.Device
	if err := model.DB.Where("hw_id = ?", fault.DeviceHwID).First(&device).Error; err == nil {
		dev = gin.H{
			"id": device.ID, "hw_id": device.HwID, "intersection": device.Intersection,
			"lat": device.Lat, "lng": device.Lng, "online_status": device.OnlineStatus,
			"sw_version": device.SwVersion, "conf_version": device.ConfVersion,
			"last_checkin_at": device.LastCheckinAt, "created_at": device.CreatedAt,
		}
	}

	// 关联工单摘要（按 fault_id 反查，工单可能未回写 fault.WorkOrderID）
	wo := gin.H{}
	var order model.WorkOrder
	err = model.DB.Where("fault_id = ?", fault.ID).Order("created_at DESC").First(&order).Error
	if err == nil {
		wo = workOrderView(order)
	}

	ok(c, gin.H{"fault": faultView(c, fault), "device": dev, "work_order": wo})
}

// faultView 故障视图：附带负责人/维修人姓名
func faultView(c *gin.Context, f model.FaultRecord) gin.H {
	v := gin.H{
		"id": f.ID, "device_hw_id": f.DeviceHwID, "err_code": f.ErrCode,
		"fault_type": f.FaultType, "fault_level": f.FaultLevel, "led_state": f.LedState,
		"current_r": f.CurrentR, "current_y": f.CurrentY, "current_g": f.CurrentG,
		"first_seen": f.FirstSeen, "last_seen": f.LastSeen, "status": f.Status,
		"owner_id": f.OwnerID, "repairer_id": f.RepairerID,
		"confirmed_at": f.ConfirmedAt, "dispatched_at": f.DispatchedAt, "resolved_at": f.ResolvedAt,
		"work_order_id": f.WorkOrderID, "created_at": f.CreatedAt, "updated_at": f.UpdatedAt,
	}
	if f.OwnerID != nil {
		var u model.User
		if err := model.DB.Select("id, username").First(&u, *f.OwnerID).Error; err == nil {
			v["owner_name"] = u.Username
		}
	}
	if f.RepairerID != nil {
		var u model.User
		if err := model.DB.Select("id, username").First(&u, *f.RepairerID).Error; err == nil {
			v["repairer_name"] = u.Username
		}
	}
	return v
}

// UpdateFault 更新故障：确认故障（设置负责人/状态/维修人）
// 支持状态流转 occurred/confirmed/dispatched/resolved，及负责人、维修人变更
func UpdateFault(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}

	var req struct {
		Status     string `json:"status"`
		OwnerID    *uint  `json:"owner_id"`
		RepairerID *uint  `json:"repairer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	var fault model.FaultRecord
	if err := model.DB.First(&fault, id).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	// 校验状态值（允许增删状态，此处校验基础四态）
	valid := map[string]bool{
		model.FaultStatusOccurred: true, model.FaultStatusConfirmed: true,
		model.FaultStatusDispatched: true, model.FaultStatusResolved: true,
	}
	if req.Status != "" && !valid[req.Status] {
		badRequest(c, "无效的故障状态")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{}
	// 默认确认场景：未指定状态但设置了负责人 → 视为确认
	if req.Status != "" {
		updates["status"] = req.Status
		// 自动记录各阶段时间
		switch req.Status {
		case model.FaultStatusConfirmed:
			updates["confirmed_at"] = &now
		case model.FaultStatusDispatched:
			updates["dispatched_at"] = &now
		case model.FaultStatusResolved:
			updates["resolved_at"] = &now
			updates["last_seen"] = now
		}
	}
	if req.OwnerID != nil {
		if *req.OwnerID == 0 {
			updates["owner_id"] = nil
		} else {
			var u model.User
			if err := model.DB.First(&u, *req.OwnerID).Error; err != nil {
				notFound(c, "负责人不存在")
				return
			}
			updates["owner_id"] = *req.OwnerID
			updates["confirmed_at"] = &now
			// 设置了负责人 + 未指定状态 且当前为“发生”时，自动推进到“已确认”
			if req.Status == "" && fault.Status == model.FaultStatusOccurred {
				updates["status"] = model.FaultStatusConfirmed
			}
		}
	}
	if req.RepairerID != nil {
		if *req.RepairerID == 0 {
			updates["repairer_id"] = nil
		} else {
			var u model.User
			if err := model.DB.First(&u, *req.RepairerID).Error; err != nil {
				notFound(c, "维修人不存在")
				return
			}
			updates["repairer_id"] = *req.RepairerID
		}
	}

	if len(updates) == 0 {
		badRequest(c, "无可更新字段")
		return
	}
	if err := model.DB.Model(&fault).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}

	// 解决故障时，若有关联未完成工单，同步置为已完成
	if req.Status == model.FaultStatusResolved {
		model.DB.Model(&model.WorkOrder{}).
			Where("fault_id = ? AND status IN ?", fault.ID,
				[]string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).
			Updates(map[string]interface{}{"status": model.WorkOrderStatusCompleted, "closed_at": &now})
	}

	// 重新读取返回最新状态
	model.DB.First(&fault, id)
	recordOperation(c, model.OpUpdate, fmt.Sprintf("fault/%d", fault.ID), "更新故障为 "+fault.Status)
	ok(c, gin.H{"fault": faultView(c, fault), "message": "故障更新成功"})
}

// DispatchFault 从故障派发工单：创建或复用工单并指派维修人，故障推进到“已派单”
func DispatchFault(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}
	var req struct {
		AssigneeID uint `json:"assignee_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请选择维修人员")
		return
	}

	var fault model.FaultRecord
	if err := model.DB.First(&fault, id).Error; err != nil {
		notFound(c, "故障记录不存在")
		return
	}

	// 校验维修人
	var u model.User
	if err := model.DB.First(&u, req.AssigneeID).Error; err != nil {
		notFound(c, "维修人员不存在")
		return
	}
	if u.Role != model.RoleAdmin && u.Role != model.RoleOperator {
		badRequest(c, "只能指派给运维人员或管理员")
		return
	}

	now := time.Now()
	// 已有未完成工单则复用，否则新建
	var wo model.WorkOrder
	err = model.DB.Where("fault_id = ? AND status IN ?", fault.ID,
		[]string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).First(&wo).Error
	if err != nil {
		wo = model.WorkOrder{
			OrderNo:    model.NextOrderNo(model.DB),
			FaultID:    fault.ID,
			DeviceHwID: fault.DeviceHwID,
			Status:     model.WorkOrderStatusProcessing,
			AssigneeID: &req.AssigneeID,
		}
		if err := model.DB.Create(&wo).Error; err != nil {
			serverError(c, err)
			return
		}
	} else {
		model.DB.Model(&wo).Updates(map[string]interface{}{
			"assignee_id": req.AssigneeID,
			"status":      model.WorkOrderStatusProcessing,
		})
	}

	// 故障推进到已派单，记录维修人与派单时间
	model.DB.Model(&fault).Updates(map[string]interface{}{
		"status":        model.FaultStatusDispatched,
		"work_order_id": wo.ID,
		"repairer_id":   req.AssigneeID,
		"dispatched_at": &now,
	})

	model.DB.First(&fault, id)
	model.DB.First(&wo, wo.ID)
	recordOperation(c, model.OpDispatch, fmt.Sprintf("work-order/%d", wo.ID), "从故障派发工单给 "+u.Username)
	ok(c, gin.H{"fault": faultView(c, fault), "work_order": workOrderView(wo), "message": "派单成功"})
}

package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// activeStatuses 旧 “active” 兼容语义 = 未解决：occurred/confirmed/dispatched
var activeStatuses = []string{
	model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
}

// ParseStatusFilter 解析故障状态筛选参数
// “active” 为兼容旧语义（= 未解决三态），其余按字面状态匹配；空返回（不筛选）
func ParseStatusFilter(status string) (op string, arg interface{}, ok bool) {
	if status == "" {
		return "", nil, false
	}
	if status == "active" {
		return "IN", activeStatuses, true
	}
	return "=", status, true
}

// ParseFaultTimeRange 解析故障时间范围参数
// 兼容 start_time/start_date 与 end_time/end_date 两套参数名，格式 2006-01-02
func ParseFaultTimeRange(c *gin.Context) (start, end *time.Time) {
	startStr := c.Query("start_time")
	if startStr == "" {
		startStr = c.Query("start_date")
	}
	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			start = &t
		}
	}
	endStr := c.Query("end_time")
	if endStr == "" {
		endStr = c.Query("end_date")
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			e := t.Add(24 * time.Hour)
			end = &e
		}
	}
	return
}

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
		if op, arg, ok := ParseStatusFilter(status); ok {
			if op == "IN" {
				query = query.Where("status IN ?", arg)
			} else {
				query = query.Where("status = ?", arg)
			}
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

	// 按研判分流状态筛选（范围B：智能识别引擎 confirmed/pending_review/filtered）
	// 兼容旧语义：recognition_status=active 视为未解决三态（occurred/confirmed/dispatched）
	if recogStatus := c.Query("recognition_status"); recogStatus != "" {
		if recogStatus == "active" {
			query = query.Where("status IN ?", activeStatuses)
		} else {
			query = query.Where("recognition_status = ?", recogStatus)
		}
	}

	// 按时间范围筛选（兼容 start_time/end_time 与 start_date/end_date 两套参数名）
	start, end := ParseFaultTimeRange(c)
	if start != nil {
		query = query.Where("first_seen >= ?", *start)
	}
	if end != nil {
		query = query.Where("last_seen <= ?", *end)
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

	// 转为带负责人/维修人姓名的视图（批量预取避免逐行 N+1）
	list := make([]gin.H, 0, len(faults))
	userNames := faultUserNames(faults)
	for i := range faults {
		list = append(list, faultViewWithNames(c, faults[i], userNames))
	}

	ok(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// faultUserNames 批量预取故障负责人/维修人姓名（避免 faultView 逐行单查 N+1）
func faultUserNames(faults []model.FaultRecord) map[uint]string {
	ids := make([]uint, 0, len(faults)*2)
	seen := make(map[uint]bool, len(faults)*2)
	for _, f := range faults {
		for _, uid := range []*uint{f.OwnerID, f.RepairerID} {
			if uid != nil && !seen[*uid] {
				seen[*uid] = true
				ids = append(ids, *uid)
			}
		}
	}
	if len(ids) == 0 {
		return map[uint]string{}
	}
	var users []model.User
	if err := model.DB.Select("id, username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return map[uint]string{}
	}
	out := make(map[uint]string, len(users))
	for _, u := range users {
		out[u.ID] = u.Username
	}
	return out
}

// faultViewWithNames 使用预取的姓名 map 构建故障视图
func faultViewWithNames(c *gin.Context, f model.FaultRecord, names map[uint]string) gin.H {
	v := gin.H{
		"id": f.ID, "device_hw_id": f.DeviceHwID, "err_code": f.ErrCode,
		"fault_type": f.FaultType, "fault_level": f.FaultLevel, "led_state": f.LedState,
		"current_r": f.CurrentR, "current_y": f.CurrentY, "current_g": f.CurrentG,
		"first_seen": f.FirstSeen, "last_seen": f.LastSeen, "status": f.Status,
		"owner_id": f.OwnerID, "repairer_id": f.RepairerID,
		"confirmed_at": f.ConfirmedAt, "dispatched_at": f.DispatchedAt, "resolved_at": f.ResolvedAt,
		"work_order_id": f.WorkOrderID, "created_at": f.CreatedAt, "updated_at": f.UpdatedAt,
		// 识别研判可选字段（带缺省，前端无需改动）
		"confidence": f.Confidence, "recognition_source": f.RecognitionSource,
		"recognition_status": f.RecognitionStatus, "is_false_positive": f.IsFalsePositive,
		"evidence_count": f.EvidenceCount, "last_evaluation_id": f.LastEvaluationID,
	}
	if f.OwnerID != nil {
		if name, ok := names[*f.OwnerID]; ok && name != "" {
			v["owner_name"] = name
		}
	}
	if f.RepairerID != nil {
		if name, ok := names[*f.RepairerID]; ok && name != "" {
			v["repairer_name"] = name
		}
	}
	return v
}

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
		// 识别研判可选字段（带缺省，前端无需改动）
		"confidence": f.Confidence, "recognition_source": f.RecognitionSource,
		"recognition_status": f.RecognitionStatus, "is_false_positive": f.IsFalsePositive,
		"evidence_count": f.EvidenceCount, "last_evaluation_id": f.LastEvaluationID,
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

// DeleteFault 删除故障记录（硬删除，需 fault:delete 权限）
// 若有关联未完成工单，则拒绝删除以避免悬空引用；无关联或工单已完结时允许删除
func DeleteFault(c *gin.Context) {
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

	// 若存在未完成工单（待处理/处理中），阻止删除
	var open int64
	model.DB.Model(&model.WorkOrder{}).
		Where("fault_id = ? AND status IN ?", id,
			[]string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).
		Count(&open)
	if open > 0 {
		badRequest(c, "该故障存在未完成工单，请先完结或删除关联工单后再删除故障")
		return
	}

	if err := model.DB.Unscoped().Delete(&fault).Error; err != nil {
		serverError(c, err)
		return
	}
	// 同步解除关联工单的 fault_id 引用（若有历史已完结工单）
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", id).
		Updates(map[string]interface{}{"fault_id": nil})

	recordOperation(c, model.OpDelete, fmt.Sprintf("fault/%d", id), "删除故障记录")
	ok(c, gin.H{"message": "故障已删除", "id": id})
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
			Updates(map[string]interface{}{
				"status":             model.WorkOrderStatusCompleted,
				"closed_at":          &now,
				"fault_active_scope": nil, // 已完结：释放 fault 的活跃工单位
			})
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
		scope := fault.ID
		wo = model.WorkOrder{
			OrderNo:          model.NextOrderNo(model.DB),
			FaultID:          fault.ID,
			DeviceHwID:       fault.DeviceHwID,
			Status:           model.WorkOrderStatusProcessing,
			AssigneeID:       &req.AssigneeID,
			FaultActiveScope: &scope, // 活跃工单占据 fault 唯一索引位（M1，与自动派单一致）
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

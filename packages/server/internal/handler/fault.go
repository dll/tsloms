package handler

import (
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

	// 按状态筛选
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
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

	ok(c, gin.H{
		"list":      faults,
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

	ok(c, gin.H{"fault": fault, "device": dev, "work_order": wo})
}

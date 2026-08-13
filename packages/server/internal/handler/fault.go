package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListFaults 故障记录列表查询
// 支持按设备、状态、故障类型、时间范围筛选，分页查询
func ListFaults(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

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

	// 按时间范围筛选
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse("2006-01-02", startTime); err == nil {
			query = query.Where("first_seen >= ?", t)
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
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
		"faults":    faults,
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

	ok(c, gin.H{"fault": fault})
}

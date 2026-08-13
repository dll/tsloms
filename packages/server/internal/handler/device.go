package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListDevices 设备列表查询
// 支持按路口位置、在线状态筛选，分页查询
func ListDevices(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	query := model.DB.Model(&model.Device{})

	// 按路口位置筛选
	if intersection := c.Query("intersection"); intersection != "" {
		query = query.Where("intersection LIKE ?", "%"+intersection+"%")
	}

	// 按在线状态筛选
	if online := c.Query("online_status"); online != "" {
		query = query.Where("online_status = ?", online == "true")
	}

	// 按硬件 ID 筛选
	if hwID := c.Query("hw_id"); hwID != "" {
		query = query.Where("hw_id = ?", hwID)
	}

	// 总数
	var total int64
	query.Count(&total)

	// 分页查询
	var devices []model.Device
	query.Order("updated_at DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&devices)

	ok(c, gin.H{
		"list":      devices,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetDevice 获取单个设备详情
func GetDevice(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "设备ID无效")
		return
	}

	var device model.Device
	if err := model.DB.First(&device, id).Error; err != nil {
		notFound(c, "设备不存在")
		return
	}

	ok(c, gin.H{"device": device})
}

// UpdateDevice 更新设备信息（路口位置、安装日期等台账信息）
func UpdateDevice(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "设备ID无效")
		return
	}

	var device model.Device
	if err := model.DB.First(&device, id).Error; err != nil {
		notFound(c, "设备不存在")
		return
	}

	var req struct {
		Intersection string `json:"intersection"`
		NetworkCode  *int   `json:"network_code"`
		StationCode  *int   `json:"station_code"`
		InstalledAt  string `json:"installed_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.Intersection != "" {
		updates["intersection"] = req.Intersection
	}
	if req.NetworkCode != nil {
		updates["network_code"] = *req.NetworkCode
	}
	if req.StationCode != nil {
		updates["station_code"] = *req.StationCode
	}
	if req.InstalledAt != "" {
		if t, err := time.Parse("2006-01-02", req.InstalledAt); err == nil {
			updates["installed_at"] = t
		}
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&device).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
	}

	ok(c, gin.H{"device": device, "message": "更新成功"})
}

// DeviceStats 设备统计（在线/离线数量）
func DeviceStats(c *gin.Context) {
	var onlineCount, offlineCount int64
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&onlineCount)
	model.DB.Model(&model.Device{}).Where("online_status = ?", false).Count(&offlineCount)

	ok(c, gin.H{
		"online":  onlineCount,
		"offline": offlineCount,
		"total":   onlineCount + offlineCount,
	})
}

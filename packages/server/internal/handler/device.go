package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListDevices 设备列表查询
// 支持按路口位置、在线状态筛选，分页查询
func ListDevices(c *gin.Context) {
	page, pageSize := paginate(c)

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

	ok(c, gin.H{
		"device":        device,
		"sw_ver_info":   model.DecodeSwVer(device.SwVersion),
		"conf_ver_info": model.DecodeConfVer(device.ConfVersion),
	})
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
		Intersection string   `json:"intersection"`
		NetworkCode  *int     `json:"network_code"`
		StationCode  *int     `json:"station_code"`
		InstalledAt  string   `json:"installed_at"`
		Lat          *float64 `json:"lat"`
		Lng          *float64 `json:"lng"`
		IsWatched    *bool    `json:"is_watched"`
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
	// 经纬度更新（用于地图打点）
	if req.Lat != nil {
		updates["lat"] = *req.Lat
	}
	if req.Lng != nil {
		updates["lng"] = *req.Lng
	}
	// 关注状态（锁定/可能故障）
	if req.IsWatched != nil {
		updates["is_watched"] = *req.IsWatched
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&device).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, fmt.Sprintf("device/%d", device.ID), "更新设备台账信息")
	}

	ok(c, gin.H{"device": device, "message": "更新成功"})
}

// CreateDevice 新增设备台账（运维/管理员）
func CreateDevice(c *gin.Context) {
	var req struct {
		HwID         uint32   `json:"hw_id" binding:"required"`
		Intersection string   `json:"intersection"`
		NetworkCode  int      `json:"network_code"`
		StationCode  int      `json:"station_code"`
		Lat          *float64 `json:"lat"`
		Lng          *float64 `json:"lng"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "硬件ID必填")
		return
	}
	// 校验硬件ID唯一
	var count int64
	model.DB.Model(&model.Device{}).Where("hw_id = ?", req.HwID).Count(&count)
	if count > 0 {
		badRequest(c, "该硬件ID设备已存在")
		return
	}
	device := model.Device{
		HwID:         req.HwID,
		Intersection: req.Intersection,
		NetworkCode:  req.NetworkCode,
		StationCode:  req.StationCode,
		Lat:          req.Lat,
		Lng:          req.Lng,
	}
	if err := model.DB.Create(&device).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("device/%d", device.ID), "新增设备台账")
	ok(c, gin.H{"device": device, "message": "设备已新增"})
}

// DeleteDevice 删除设备台账（仅管理员）
func DeleteDevice(c *gin.Context) {
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
	if err := model.DB.Delete(&device).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("device/%d", id), "删除设备台账")
	ok(c, gin.H{"message": "设备已删除"})
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

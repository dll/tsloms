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
		query = query.Where("online_status = ?", online == "true" || online == "online")
	}
	if access := c.Query("access_status"); access != "" {
		query = query.Where("access_status = ?", access)
	}
	if lifecycle := c.Query("lifecycle_status"); lifecycle != "" {
		query = query.Where("lifecycle_status = ?", lifecycle)
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
		"fault_summary": deviceFaultSummary(device.HwID),
	})
}

func deviceFaultSummary(hwID string) gin.H {
	var active int64
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = ? AND status <> ?", hwID, model.FaultStatusResolved).Count(&active)
	return gin.H{"active_count": active}
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
		// P0-4 路口/区划挂接（可空）
		CrossingID  *uint  `json:"crossing_id"`
		StreetID    *uint  `json:"street_id"`
		CommunityID *uint  `json:"community_id"`
		RoadID      *uint  `json:"road_id"`
		RoadName    string `json:"road_name"`
		// 设备资料（照片/说明书/维修手册）
		Photo            *string `json:"photo"`
		ManualUrl        *string `json:"manual_url"`
		ManualName       *string `json:"manual_name"`
		RepairManualUrl  *string `json:"repair_manual_url"`
		RepairManualName *string `json:"repair_manual_name"`
		// 对齐项目 a：func信号灯功能 / orientation朝向(cx) / direction方向(fx) / batch批次 / remark备注
		Func        *string `json:"func"`
		Orientation *string `json:"orientation"`
		Direction   *string `json:"direction"`
		Batch       *string `json:"batch"`
		Remark      *string `json:"remark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	// 区划/路口挂接（只增不删；空指针表示不修改，传 0 指针可置空见下方）
	applyDeviceArea(updates, req.CrossingID, req.StreetID, req.CommunityID, req.RoadID, req.RoadName)
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
	// 设备资料（照片/说明书/维修手册；指针值可为空字符串以清除）
	if req.Photo != nil {
		updates["photo"] = *req.Photo
	}
	if req.ManualUrl != nil {
		updates["manual_url"] = *req.ManualUrl
	}
	if req.ManualName != nil {
		updates["manual_name"] = *req.ManualName
	}
	if req.RepairManualUrl != nil {
		updates["repair_manual_url"] = *req.RepairManualUrl
	}
	if req.RepairManualName != nil {
		updates["repair_manual_name"] = *req.RepairManualName
	}
	// 对齐项目 a：func/朝向/方向/批次/备注
	if req.Func != nil {
		updates["func"] = *req.Func
	}
	if req.Orientation != nil {
		updates["orientation"] = *req.Orientation
	}
	if req.Direction != nil {
		updates["direction"] = *req.Direction
	}
	if req.Batch != nil {
		updates["batch"] = *req.Batch
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
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
		HwID         string   `json:"hw_id" binding:"required"`
		Intersection string   `json:"intersection"`
		NetworkCode  int      `json:"network_code"`
		StationCode  int      `json:"station_code"`
		Lat          *float64 `json:"lat"`
		Lng          *float64 `json:"lng"`
		// P0-4 路口/区划挂接
		CrossingID  *uint  `json:"crossing_id"`
		StreetID    *uint  `json:"street_id"`
		CommunityID *uint  `json:"community_id"`
		RoadID      *uint  `json:"road_id"`
		RoadName    string `json:"road_name"`
		// 设备资料（照片/说明书/维修手册）
		Photo            string `json:"photo"`
		ManualUrl        string `json:"manual_url"`
		ManualName       string `json:"manual_name"`
		RepairManualUrl  string `json:"repair_manual_url"`
		RepairManualName string `json:"repair_manual_name"`
		// 对齐项目 a：func/朝向/方向/批次/备注
		Func        string `json:"func"`
		Orientation string `json:"orientation"`
		Direction   string `json:"direction"`
		Batch       string `json:"batch"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "硬件ID必填")
		return
	}
	req.HwID = model.NormalizeHardwareID(req.HwID)
	if !model.IsValidHardwareID(req.HwID) {
		badRequest(c, "硬件ID格式无效，应为8位十六进制或LA加8位编码（如 LA82533848）")
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
		HwID:             req.HwID,
		Intersection:     req.Intersection,
		NetworkCode:      req.NetworkCode,
		StationCode:      req.StationCode,
		Lat:              req.Lat,
		Lng:              req.Lng,
		CrossingID:       req.CrossingID,
		StreetID:         req.StreetID,
		CommunityID:      req.CommunityID,
		RoadID:           req.RoadID,
		RoadName:         req.RoadName,
		Photo:            req.Photo,
		ManualUrl:        req.ManualUrl,
		ManualName:       req.ManualName,
		RepairManualUrl:  req.RepairManualUrl,
		RepairManualName: req.RepairManualName,
		Func:             req.Func,
		Orientation:      req.Orientation,
		Direction:        req.Direction,
		Batch:            req.Batch,
		Remark:           req.Remark,
		LifecycleStatus:  "active", AccessStatus: "never", RegistrationSource: "manual",
	}
	if err := model.DB.Create(&device).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("device/%d", device.ID), "新增设备台账")
	ok(c, gin.H{"device": device, "message": "设备已新增"})
}

// RetireDevice 报废设备，保留故障、工单与审计历史。
func RetireDevice(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "设备ID无效")
		return
	}
	var device model.Device
	if model.DB.First(&device, id).Error != nil {
		notFound(c, "设备不存在")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	now := time.Now()
	if err := model.DB.Model(&device).Updates(map[string]interface{}{"lifecycle_status": "retired", "online_status": false, "access_status": "offline", "retired_at": now, "retired_reason": req.Reason}).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("device/%d", id), "设备报废")
	ok(c, gin.H{"message": "设备已报废"})
}

// RestoreDevice 恢复设备台账，等待再次 MQTT 接入。
func RestoreDevice(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "设备ID无效")
		return
	}
	var device model.Device
	if model.DB.First(&device, id).Error != nil {
		notFound(c, "设备不存在")
		return
	}
	if err := model.DB.Model(&device).Updates(map[string]interface{}{"lifecycle_status": "active", "online_status": false, "access_status": "never", "retired_at": nil, "retired_reason": ""}).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("device/%d", id), "恢复设备台账")
	ok(c, gin.H{"message": "设备已恢复，等待再次接入"})
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
	// 仅允许删除从未接入且无故障历史的误登记记录；正式设备必须报废以保留历史。
	if device.AccessStatus != "never" && device.FirstAccessAt != nil {
		badRequest(c, "正式设备不可直接删除，请使用报废操作保留历史")
		return
	}
	var faultCount int64
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = ?", device.HwID).Count(&faultCount)
	if faultCount > 0 {
		badRequest(c, "存在故障历史，请使用报废操作")
		return
	}
	if err := model.DB.Delete(&device).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("device/%d", id), "删除未接入误登记设备台账")
	ok(c, gin.H{"message": "未接入设备已删除"})
}

// applyDeviceArea 将路口/区划挂接字段写入 updates map。
// 约定：传入的指针若为 nil 表示不修改；若指向 0 表示置空（解除挂接）；否则写对应 ID。
func applyDeviceArea(updates map[string]interface{}, crossing, street, community, road *uint, roadName string) {
	setOrNull := func(key string, v *uint) {
		if v == nil {
			return
		}
		if *v == 0 {
			updates[key] = nil
		} else {
			updates[key] = *v
		}
	}
	setOrNull("crossing_id", crossing)
	setOrNull("street_id", street)
	setOrNull("community_id", community)
	setOrNull("road_id", road)
	if roadName != "" {
		updates["road_name"] = roadName
	}
}

// DeviceStats 设备统计（在线/离线数量）
func DeviceStats(c *gin.Context) {
	var onlineCount, offlineCount int64
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&onlineCount)
	model.DB.Model(&model.Device{}).Where("lifecycle_status <> ? AND online_status = ?", "retired", false).Count(&offlineCount)

	ok(c, gin.H{
		"online":  onlineCount,
		"offline": offlineCount,
		"total":   onlineCount + offlineCount,
	})
}

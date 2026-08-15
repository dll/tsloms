package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// DispatchReference 派单参考
// 针对指定设备，聚合其活跃故障、待处理工单、备件耗材、监控媒体，
// 为维修派单提供参考信息。
// query: device_hw_id
func DispatchReference(c *gin.Context) {
	hwID := c.Query("device_hw_id")
	if hwID == "" {
		badRequest(c, "device_hw_id 必填")
		return
	}

	var hwIDUint uint32
	_, _ = fmt.Sscanf(hwID, "%d", &hwIDUint)

	// 活跃故障
	var faults []model.FaultRecord
	model.DB.Where("device_hw_id = ? AND status IN ?", hwIDUint, []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Order("last_seen DESC").Find(&faults)

	// 待处理工单
	var orders []model.WorkOrder
	model.DB.Where("device_hw_id = ? AND status IN ?", hwIDUint, []string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).
		Order("created_at DESC").Find(&orders)

	// 耗材（设备绑定物料，来自统一库存模块）
	var materials []model.Material
	model.DB.Where("device_hw_id = ?", hwIDUint).Find(&materials)

	// 媒体（举证/监控/时间视频）
	var media []model.DeviceMedia
	model.DB.Where("device_hw_id = ?", hwIDUint).Order("created_at DESC").Limit(20).Find(&media)

	ok(c, gin.H{
		"device_hw_id": hwIDUint,
		"faults":       faults,
		"work_orders":  orders,
		"materials":    materials,
		"media":        media,
	})
}

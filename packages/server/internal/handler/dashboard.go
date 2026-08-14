package handler

import (
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// FaultTypeStats 故障类型占比统计（饼图）
// 按故障类型分组统计数量
func FaultTypeStats(c *gin.Context) {
	// 默认统计近 30 天数据
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := fmt.Sscanf(d, "%d", &days); err == nil && n == 1 && days > 0 {
			// 使用 days
		}
	}
	startTime := time.Now().AddDate(0, 0, -days)

	type StatsResult struct {
		FaultType string `json:"fault_type"`
		Count     int64  `json:"count"`
	}

	var results []StatsResult
	model.DB.Model(&model.FaultRecord{}).
		Select("fault_type, COUNT(*) as count").
		Where("first_seen >= ?", startTime).
		Group("fault_type").
		Order("count DESC").
		Find(&results)

	ok(c, gin.H{
		"stats": results,
		"days":  days,
	})
}

// WorkOrderStatusStats 工单状态分布统计（饼图）
// 按工单状态分组统计数量
func WorkOrderStatusStats(c *gin.Context) {
	type StatsResult struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	var results []StatsResult
	model.DB.Model(&model.WorkOrder{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Order("count DESC").
		Find(&results)

	ok(c, gin.H{"stats": results})
}

// FaultTrendStats 故障趋势统计（柱状图）
// 按日/周/月统计故障数量趋势，使用 Go 层分组保证 MySQL/SQLite 兼容
func FaultTrendStats(c *gin.Context) {
	dimension := c.DefaultQuery("dimension", "day")
	days := 7
	if d := c.Query("days"); d != "" {
		if n, err := fmt.Sscanf(d, "%d", &days); err == nil && n == 1 && days > 0 {
		}
	}

	startTime := time.Now().AddDate(0, 0, -days)

	var faults []model.FaultRecord
	model.DB.Where("first_seen >= ?", startTime).Find(&faults)

	counts := make(map[string]int64)
	for _, f := range faults {
		var period string
		switch dimension {
		case "week":
			year, week := f.FirstSeen.ISOWeek()
			period = fmt.Sprintf("%d-W%02d", year, week)
		case "month":
			period = f.FirstSeen.Format("2006-01")
		default:
			period = f.FirstSeen.Format("2006-01-02")
		}
		counts[period]++
	}

	type TrendResult struct {
		Period string `json:"period"`
		Count  int64  `json:"count"`
	}
	var results []TrendResult
	for period, count := range counts {
		results = append(results, TrendResult{Period: period, Count: count})
	}

	// 按时间升序排列结果
	sort.Slice(results, func(i, j int) bool {
		return results[i].Period < results[j].Period
	})

	ok(c, gin.H{
		"trend":     results,
		"dimension": dimension,
		"days":      days,
	})
}

// DeviceFaultRank 设备故障数量排行（柱状图）
// 统计各设备故障数量 Top N
func DeviceFaultRank(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); err == nil && n == 1 && limit > 0 {
			// 使用 limit
		}
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := fmt.Sscanf(d, "%d", &days); err == nil && n == 1 && days > 0 {
			// 使用 days
		}
	}
	startTime := time.Now().AddDate(0, 0, -days)

	type RankResult struct {
		DeviceHwID uint32 `json:"device_hw_id"`
		Count      int64  `json:"count"`
	}

	var results []RankResult
	model.DB.Model(&model.FaultRecord{}).
		Select("device_hw_id, COUNT(*) as count").
		Where("first_seen >= ?", startTime).
		Group("device_hw_id").
		Order("count DESC").
		Limit(limit).
		Find(&results)

	ok(c, gin.H{
		"rank":  results,
		"days":  days,
		"limit": limit,
	})
}

// DashboardOverview 看板总览数据
// 汇总设备在线数、活跃故障数、待处理工单数等关键指标
func DashboardOverview(c *gin.Context) {
	// 设备统计
	var onlineDevices, offlineDevices int64
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&onlineDevices)
	model.DB.Model(&model.Device{}).Where("online_status = ?", false).Count(&offlineDevices)

	// 故障统计
	var activeFaults, resolvedFaults int64
	model.DB.Model(&model.FaultRecord{}).Where("status = ?", "active").Count(&activeFaults)
	model.DB.Model(&model.FaultRecord{}).Where("status = ?", "resolved").Count(&resolvedFaults)

	// 工单统计
	var pendingOrders, processingOrders, completedOrders int64
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusPending).Count(&pendingOrders)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusProcessing).Count(&processingOrders)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusCompleted).Count(&completedOrders)

	// 今日新增故障
	today := time.Now().Truncate(24 * time.Hour)
	var todayFaults int64
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", today).Count(&todayFaults)

	ok(c, gin.H{
		"devices": gin.H{
			"online":  onlineDevices,
			"offline": offlineDevices,
			"total":   onlineDevices + offlineDevices,
		},
		"faults": gin.H{
			"active":   activeFaults,
			"resolved": resolvedFaults,
			"today":    todayFaults,
		},
		"work_orders": gin.H{
			"pending":    pendingOrders,
			"processing": processingOrders,
			"completed":  completedOrders,
		},
	})
}

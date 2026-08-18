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

	// 超时工单统计：pending 超 SLA(24h) 或 processing 超 SLA(48h)
	now := time.Now()
	pendingOverdue := now.Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	procOverdue := now.Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	var overdueOrders int64
	model.DB.Model(&model.WorkOrder{}).
		Where("(status = ? AND created_at < ?) OR (status = ? AND created_at < ?)",
			model.WorkOrderStatusPending, pendingOverdue,
			model.WorkOrderStatusProcessing, procOverdue).
		Count(&overdueOrders)

	ok(c, gin.H{"stats": results, "overdue": overdueOrders})
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
		DeviceHwID string `json:"device_hw_id"`
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

	// 故障统计（活跃=发生/确认/派单，未解决；已解决单独统计）
	var activeFaults, resolvedFaults int64
	model.DB.Model(&model.FaultRecord{}).Where("status IN ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Count(&activeFaults)
	model.DB.Model(&model.FaultRecord{}).Where("status = ?", model.FaultStatusResolved).Count(&resolvedFaults)

	// 工单统计
	var pendingOrders, processingOrders, completedOrders int64
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusPending).Count(&pendingOrders)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusProcessing).Count(&processingOrders)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusCompleted).Count(&completedOrders)

	// 超时工单统计：pending 超 SLA(24h) 或 processing 超 SLA(48h)
	now := time.Now()
	pendingOverdue := now.Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	procOverdue := now.Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	var overdueOrders int64
	model.DB.Model(&model.WorkOrder{}).
		Where("(status = ? AND created_at < ?) OR (status = ? AND created_at < ?)",
			model.WorkOrderStatusPending, pendingOverdue,
			model.WorkOrderStatusProcessing, procOverdue).
		Count(&overdueOrders)

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
			"overdue":    overdueOrders,
		},
	})
}

// WorkOrderAvgClosure 工单平均闭环时长（小时）
// 统计已完成工单从创建到闭环的平均耗时
func WorkOrderAvgClosure(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := fmt.Sscanf(d, "%d", &days); err == nil && n == 1 && days > 0 {
		}
	}
	startTime := time.Now().AddDate(0, 0, -days)

	type closureRow struct {
		CreatedAt time.Time
		ClosedAt  *time.Time
	}
	var rows []closureRow
	model.DB.Model(&model.WorkOrder{}).
		Select("created_at, closed_at").
		Where("status = ? AND closed_at IS NOT NULL AND created_at >= ?", model.WorkOrderStatusCompleted, startTime).
		Find(&rows)

	var totalHours float64
	count := len(rows)
	for _, r := range rows {
		if r.ClosedAt != nil {
			totalHours += r.ClosedAt.Sub(r.CreatedAt).Hours()
		}
	}

	avgHours := 0.0
	if count > 0 {
		avgHours = totalHours / float64(count)
	}

	ok(c, gin.H{
		"avg_hours":       avgHours,
		"completed_count": count,
		"total_hours":     totalHours,
		"days":            days,
	})
}

// AIDashboardOverview AI 智慧大屏聚合数据
// 返回：今日 AI 额度用量、最新预测批次风险分布、高风险设备清单、AI 动作汇总
func AIDashboardOverview(c *gin.Context) {
	cfg := model.GetAIConfig()
	userID := c.GetUint("user_id")

	// 今日额度消耗（当前用户）
	tokens, calls := model.TodayAIConsumed(userID)

	// 最新预测批次风险分布：取所有批次里最近的一批，统计各风险等级设备数
	var latestBatch string
	model.DB.Model(&model.AIPrediction{}).Select("MAX(batch_id)").Scan(&latestBatch)
	riskDist := gin.H{"low": 0, "medium": 0, "high": 0, "critical": 0}
	if latestBatch != "" {
		var rows []struct {
			RiskLevel string
			Count     int64
		}
		model.DB.Model(&model.AIPrediction{}).
			Select("risk_level, COUNT(*) AS count").
			Where("batch_id = ?", latestBatch).
			Group("risk_level").Scan(&rows)
		total := int64(0)
		for _, r := range rows {
			riskDist[r.RiskLevel] = r.Count
			total += r.Count
		}
		riskDist["total"] = total
		riskDist["batch_id"] = latestBatch
	}

	// 最新批次中高/极高风险设备清单（取 top 5）
	var highRisk []gin.H
	if latestBatch != "" {
		var preds []model.AIPrediction
		model.DB.Where("batch_id = ? AND risk_level IN ?", latestBatch, []string{"high", "critical"}).
			Order("health_score ASC").Limit(5).Find(&preds)
		for _, p := range preds {
			highRisk = append(highRisk, gin.H{
				"device_hw_id": p.DeviceHwID,
				"intersection": p.Intersection,
				"health_score": p.HealthScore,
				"risk_level":   p.RiskLevel,
				"predict_type": p.PredictType,
				"remain_days":  p.RemainDays,
			})
		}
	}

	// 今日 AI 动作汇总
	today := time.Now().Truncate(24 * time.Hour)
	type actRow struct {
		Action string
		Count  int64
	}
	var acts []actRow
	model.DB.Model(&model.AIUsage{}).
		Select("action, COUNT(*) AS count").
		Where("created_at >= ?", today).
		Group("action").Scan(&acts)
	actionSummary := gin.H{}
	for _, a := range acts {
		actionSummary[a.Action] = a.Count
	}

	// 近 7 天预测批次趋势（每日有预测的设备数）
	var batchRows []struct {
		BatchID string
		Count   int64
	}
	model.DB.Model(&model.AIPrediction{}).
		Select("batch_id, COUNT(DISTINCT device_hw_id) AS count").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).
		Group("batch_id").Order("batch_id ASC").Scan(&batchRows)
	batchTrend := make([]gin.H, 0, len(batchRows))
	for _, b := range batchRows {
		batchTrend = append(batchTrend, gin.H{"batch_id": b.BatchID, "count": b.Count})
	}

	ok(c, gin.H{
		"config": gin.H{
			"enabled":         cfg.Enabled,
			"provider":        cfg.Provider,
			"day_token_limit": cfg.DayTokenLimit,
			"day_call_limit":  cfg.DayCallLimit,
		},
		"today": gin.H{
			"tokens": tokens,
			"calls":  calls,
		},
		"risk_distribution": riskDist,
		"high_risk_devices": highRisk,
		"action_summary":    actionSummary,
		"batch_trend":       batchTrend,
	})
}

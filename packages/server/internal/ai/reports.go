package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// 各模块运维报告（AI 生成）：日报/库存/成本/故障/工单/设备
// 统一结构：数据快照 + 摘要 + 结论建议，持久化到 ai_reports。
// ============================================================

// DailySnapshot 运维日报快照
type DailySnapshot struct {
	Date            string          `json:"date"`
	Devices         DailyCounts     `json:"devices"` // 设备在线/离线/总数
	Faults          DailyCounts     `json:"faults"`  // 故障活跃/今日新增/已解决
	WorkOrders      WorkOrderCounts `json:"work_orders"`
	OverdueOrders   int64           `json:"overdue_orders"` // 超时工单
	AvgClosureHours float64         `json:"avg_closure_hours"`
	NewExpenses     float64         `json:"new_expenses"`      // 今日新增费用
	RecentInvents   int             `json:"recent_invents"`    // 近30天入库
	RecentUses      int             `json:"recent_uses"`       // 近30天领用
	HealthSummary   string          `json:"health_summary"`    // 预测风险概览(高风险设备数)
	HighRiskDevices []MaterialMin   `json:"high_risk_devices"` // 复用结构：高风险设备简要
}

type DailyCounts struct {
	Active   int64 `json:"active"`
	Total    int64 `json:"total"`
	TodayNew int64 `json:"today_new"`
}

type WorkOrderCounts struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Total      int64 `json:"total"`
}

// BuildDailySnapshot 聚合当日运维快照
func BuildDailySnapshot() (*DailySnapshot, error) {
	s := &DailySnapshot{Date: time.Now().Format("2006-01-02")}
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	model.DB.Model(&model.Device{}).Count(&s.Devices.Total)
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&s.Devices.Active)
	s.Devices.TodayNew = s.Devices.Active // 在线即活跃示意

	model.DB.Model(&model.FaultRecord{}).Where("status IN ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Count(&s.Faults.Active)
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", today).Count(&s.Faults.TodayNew)
	model.DB.Model(&model.FaultRecord{}).Where("status = ?", model.FaultStatusResolved).Count(&s.Faults.Total)

	model.DB.Model(&model.WorkOrder{}).Count(&s.WorkOrders.Total)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusPending).Count(&s.WorkOrders.Pending)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusProcessing).Count(&s.WorkOrders.Processing)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusCompleted).Count(&s.WorkOrders.Completed)

	// 超时工单
	pendingOverdue := now.Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	procOverdue := now.Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	model.DB.Model(&model.WorkOrder{}).
		Where("(status = ? AND created_at < ?) OR (status = ? AND created_at < ?)",
			model.WorkOrderStatusPending, pendingOverdue, model.WorkOrderStatusProcessing, procOverdue).
		Count(&s.OverdueOrders)

	// 平均闭环时长（近30天）
	var closure []struct {
		CreatedAt time.Time
		ClosedAt  *time.Time
	}
	model.DB.Model(&model.WorkOrder{}).
		Select("created_at, closed_at").Where("status = ? AND closed_at IS NOT NULL AND created_at >= ?",
		model.WorkOrderStatusCompleted, time.Now().AddDate(0, 0, -30)).Find(&closure)
	var hrs float64
	n := 0
	for _, c := range closure {
		if c.ClosedAt != nil {
			hrs += c.ClosedAt.Sub(c.CreatedAt).Hours()
			n++
		}
	}
	if n > 0 {
		s.AvgClosureHours = hrs / float64(n)
	}

	model.DB.Model(&model.RepairExpense{}).Where("created_at >= ?", today).
		Select("COALESCE(SUM(amount),0)").Scan(&s.NewExpenses)

	// 库存近30天
	since30 := now.AddDate(0, 0, -30)
	var inC, useC int64
	model.DB.Model(&model.MaterialStock{}).Where("type IN ? AND created_at >= ?",
		[]string{model.StockTypeIn, model.StockTypeGain, model.StockTypeReturn}, since30).
		Select("COALESCE(SUM(quantity),0)").Scan(&inC)
	model.DB.Model(&model.MaterialStock{}).Where("type = ? AND created_at >= ?", model.StockTypeUse, since30).
		Select("COALESCE(SUM(ABS(quantity)),0)").Scan(&useC)
	s.RecentInvents = int(inC)
	s.RecentUses = int(useC)

	// 预测高风险设备
	var latestBatch string
	model.DB.Model(&model.AIPrediction{}).Select("MAX(batch_id)").Scan(&latestBatch)
	if latestBatch != "" {
		var preds []model.AIPrediction
		model.DB.Where("batch_id = ? AND risk_level IN ?", latestBatch, []string{"high", "critical"}).
			Order("health_score ASC").Limit(5).Find(&preds)
		for _, p := range preds {
			s.HighRiskDevices = append(s.HighRiskDevices, MaterialMin{
				ID: uint(p.DeviceHwID), Name: fmt.Sprintf("设备#%d", p.DeviceHwID),
				Code: p.Intersection, Stock: p.HealthScore, Threshold: 0,
				Use30: 0, LastUse: p.PredictType,
			})
		}
		s.HealthSummary = fmt.Sprintf("最新预测批次 %s 中有 %d 台高风险/极高风险设备", latestBatch, len(s.HighRiskDevices))
	} else {
		s.HealthSummary = "尚无预测批次"
	}
	return s, nil
}

// GenerateDailyReport 生成运维日报
func GenerateDailyReport(userID uint) (string, error) {
	snap, err := BuildDailySnapshot()
	if err != nil {
		return "", err
	}
	snapJSON := mustJSON(snap)
	client := NewLLMClient(nil)
	prompt := fmt.Sprintf(
		"你是交通信号灯运维平台的值班主管。基于以下<今日运维日报>数据（JSON），用中文写一段 ≤200字的工作日报，"+
			"结构：今日整体情况；需重点关注（超时工单/高风险设备/异常）；明日工作建议与优先级。\n日报数据：%s",
		snapJSON)
	summary := ""
	source := "规则"
	tokens := 0
	if text, tk, err := client.Ask(userID, "report_daily", prompt); err == nil {
		summary = text
		source = "LLM"
		tokens = tk
	} else {
		summary = buildDailyRuleSummary(snap)
	}
	// 持久化
	report := model.AIReport{
		Module: "daily", Title: "运维日报 " + snap.Date, Period: "day",
		RangeFrom: snap.Date, RangeTo: snap.Date,
		Summary: summary, Data: snapJSON, Insights: mustJSON(splitLines(summary)),
		Source: source, TokensUsed: tokens,
	}
	model.DB.Create(&report)
	return mustJSON(map[string]any{
		"id": report.ID, "title": report.Title, "summary": summary, "source": source,
		"tokens_used": tokens, "data": snapJSON, "insights": splitLines(summary), "created_at": report.CreatedAt,
	}), nil
}

func buildDailyRuleSummary(s *DailySnapshot) string {
	lines := []string{}
	lines = append(lines, fmt.Sprintf("【今日运维日报 %s】", s.Date))
	lines = append(lines, fmt.Sprintf("设备：在线 %d/%d 台；故障：活跃 %d、今日新增 %d、已解决 %d。",
		s.Devices.Active, s.Devices.Total, s.Faults.Active, s.Faults.TodayNew, s.Faults.Total))
	lines = append(lines, fmt.Sprintf("工单：待处理 %d、处理中 %d、已完成 %d，超时 %d 单，平均闭环 %.1f 小时。",
		s.WorkOrders.Pending, s.WorkOrders.Processing, s.WorkOrders.Completed, s.OverdueOrders, s.AvgClosureHours))
	lines = append(lines, fmt.Sprintf("今日新增费用 %.2f 元；近30天入库 %d 件、领用 %d 件。", s.NewExpenses, s.RecentInvents, s.RecentUses))
	if s.OverdueOrders > 0 {
		lines = append(lines, fmt.Sprintf("⚠ 有 %d 张工单超时，请优先跟进处理。", s.OverdueOrders))
	}
	if len(s.HighRiskDevices) > 0 {
		names := ""
		for _, d := range s.HighRiskDevices[:min(5, len(s.HighRiskDevices))] {
			names += d.Name + " "
		}
		lines = append(lines, "⚠ 高风险设备："+names+"，建议优先巡检。")
	}
	return joinLines(lines)
}

// GenerateModuleReport 生成指定模块报告（inventory/cost/fault/workorder/device）
// 复用各自分析快照，LLM 生成摘要，入库。
func GenerateModuleReport(userID uint, module, period string) (string, error) {
	var title string
	var snapJSON string
	var prompt string
	var ruleSummary string

	switch module {
	case "inventory":
		an, err := AnalyzeInventory(userID)
		if err != nil {
			return "", err
		}
		snapJSON = mustJSON(an.Snapshot)
		title = "库存健康报告"
		prompt = "你是库存专家，基于以下库存快照（JSON），用中文写 ≤180字库存报告：库存总量与资金、预警/缺货/滞销、周转趋势、补货建议。\n快照：" + snapJSON
		ruleSummary = an.Insight
	case "cost":
		an, err := AnalyzeCost(userID, 90)
		if err != nil {
			return "", err
		}
		snapJSON = mustJSON(an.Snapshot)
		title = "维修成本报告"
		prompt = "你是成本专家，基于以下维修成本快照（JSON，近90天），用中文写 ≤180字成本报告：成本结构与占比、高成本设备/故障类型、降本建议。\n快照：" + snapJSON
		ruleSummary = an.Insight
	case "fault":
		snapJSON, ruleSummary = buildFaultReportData()
		title = "故障分析报告"
		prompt = "你是运维专家，基于以下故障统计快照（JSON），用中文写 ≤180字故障报告：故障类型分布、趋势、高发设备、改进建议。\n快照：" + snapJSON
	case "workorder":
		snapJSON, ruleSummary = buildWorkOrderReportData()
		title = "工单效能报告"
		prompt = "你是运维主管，基于以下工单统计快照（JSON，近30天），用中文写 ≤180字工单效能报告：数量与状态分布、闭环时长、SLA表现、改进建议。\n快照：" + snapJSON
	case "device":
		snapJSON, ruleSummary = buildDeviceReportData()
		title = "设备健康报告"
		prompt = "你是运维专家，基于以下设备统计快照（JSON），用中文写 ≤180字设备健康报告：设备总量/在线率、离线高发、故障高发、预测风险、建议。\n快照：" + snapJSON
	default:
		return "", fmt.Errorf("不支持的模块: %s", module)
	}

	summary := ruleSummary
	source := "规则"
	tokens := 0
	client := NewLLMClient(nil)
	if text, tk, err := client.Ask(userID, "report_"+module, prompt); err == nil {
		summary = text
		source = "LLM"
		tokens = tk
	}
	rangeFrom, rangeTo := periodRange(period)
	report := model.AIReport{
		Module: module, Title: title, Period: period,
		RangeFrom: rangeFrom, RangeTo: rangeTo,
		Summary: summary, Data: snapJSON, Insights: mustJSON(splitLines(summary)),
		Source: source, TokensUsed: tokens,
	}
	model.DB.Create(&report)
	return mustJSON(map[string]any{
		"id": report.ID, "module": module, "title": title, "summary": summary,
		"source": source, "tokens_used": tokens, "data": snapJSON,
		"insights": splitLines(summary), "created_at": report.CreatedAt,
	}), nil
}

// ---- 模块报告数据 ----

func buildFaultReportData() (string, string) {
	d := map[string]any{}
	// 近30天故障按类型
	var types []struct {
		FaultType string
		Count     int64
	}
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", time.Now().AddDate(0, 0, -30)).
		Select("fault_type, COUNT(*) AS count").Group("fault_type").Order("count DESC").Limit(10).Scan(&types)
	d["type_dist"] = types
	// 高发设备
	var top []struct {
		DeviceHwID uint32
		Count      int64
	}
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", time.Now().AddDate(0, 0, -30)).
		Select("device_hw_id, COUNT(*) AS count").Group("device_hw_id").Order("count DESC").Limit(5).Scan(&top)
	d["top_devices"] = top
	// 近7天趋势
	var trend []struct {
		Date  string
		Count int64
	}
	model.DB.Raw(`SELECT DATE_FORMAT(first_seen,'%m-%d') AS date, COUNT(*) AS count
		FROM fault_records WHERE first_seen >= ? GROUP BY DATE_FORMAT(first_seen,'%m-%d') ORDER BY date`,
		time.Now().AddDate(0, 0, -7)).Scan(&trend)
	d["trend7"] = trend

	rs := fmt.Sprintf("近30天故障高发类型：%s；高发设备：%s。",
		joinFaults(types), joinDevices(top))
	return mustJSON(d), rs
}

func buildWorkOrderReportData() (string, string) {
	since30 := time.Now().AddDate(0, 0, -30)
	d := map[string]any{}
	var statusCounts []struct {
		Status string
		Count  int64
	}
	model.DB.Model(&model.WorkOrder{}).Select("status, COUNT(*) AS count").Group("status").Scan(&statusCounts)
	d["status"] = statusCounts
	// 闭环时长均值
	var closure []struct {
		CreatedAt time.Time
		ClosedAt  *time.Time
	}
	model.DB.Model(&model.WorkOrder{}).Select("created_at, closed_at").
		Where("status = ? AND closed_at IS NOT NULL AND created_at >= ?", model.WorkOrderStatusCompleted, since30).Find(&closure)
	var hrs float64
	n := 0
	for _, c := range closure {
		if c.ClosedAt != nil {
			hrs += c.ClosedAt.Sub(c.CreatedAt).Hours()
			n++
		}
	}
	avg := 0.0
	if n > 0 {
		avg = hrs / float64(n)
	}
	d["avg_closure_hours"] = avg
	d["completed_count"] = n
	// 超时
	var overdue int64
	pendingOverdue := time.Now().Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	procOverdue := time.Now().Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	model.DB.Model(&model.WorkOrder{}).
		Where("(status = ? AND created_at < ?) OR (status = ? AND created_at < ?)",
			model.WorkOrderStatusPending, pendingOverdue, model.WorkOrderStatusProcessing, procOverdue).Count(&overdue)
	d["overdue"] = overdue
	rs := fmt.Sprintf("近30天完成 %d 张工单，平均闭环 %.1f 小时，当前超时 %d 张。", n, avg, overdue)
	return mustJSON(d), rs
}

func buildDeviceReportData() (string, string) {
	d := map[string]any{}
	var online, total int64
	model.DB.Model(&model.Device{}).Count(&total)
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&online)
	d["total"] = total
	d["online"] = online
	d["offline"] = total - online
	// 高故障设备（近30天）
	var top []struct {
		DeviceHwID uint32
		Count      int64
	}
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", time.Now().AddDate(0, 0, -30)).
		Select("device_hw_id, COUNT(*) AS count").Group("device_hw_id").Order("count DESC").Limit(5).Scan(&top)
	d["top_fault_devices"] = top
	// 最新预测风险分布
	var latestBatch string
	model.DB.Model(&model.AIPrediction{}).Select("MAX(batch_id)").Scan(&latestBatch)
	if latestBatch != "" {
		var risk []struct {
			RiskLevel string
			Count     int64
		}
		model.DB.Model(&model.AIPrediction{}).Select("risk_level, COUNT(*) AS count").
			Where("batch_id = ?", latestBatch).Group("risk_level").Scan(&risk)
		d["risk_dist"] = risk
	}
	rs := fmt.Sprintf("设备共 %d 台，在线 %d（在线率 %.0f%%）；高故障设备：%s。",
		total, online, pct(online, total), joinDevices(top))
	return mustJSON(d), rs
}

// ---- 报告历史 ----

// ListReports 查询历史报告
func ListReports(module string, limit int) []model.AIReport {
	q := model.DB.Model(&model.AIReport{}).Order("created_at DESC")
	if module != "" {
		q = q.Where("module = ?", module)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var list []model.AIReport
	q.Limit(limit).Find(&list)
	return list
}

// ---- 工具 ----

func joinFaults(items []struct {
	FaultType string
	Count     int64
}) string {
	out := ""
	for i, t := range items {
		if i >= 3 {
			break
		}
		if i > 0 {
			out += "、"
		}
		out += fmt.Sprintf("%s(%d)", t.FaultType, t.Count)
	}
	return out
}

func joinDevices(items []struct {
	DeviceHwID uint32
	Count      int64
}) string {
	out := ""
	for i, t := range items {
		if i >= 3 {
			break
		}
		if i > 0 {
			out += "、"
		}
		out += fmt.Sprintf("#%d(%d)", t.DeviceHwID, t.Count)
	}
	return out
}

func pct(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}

func periodRange(period string) (string, string) {
	now := time.Now()
	switch period {
	case "week":
		from := now.AddDate(0, 0, -7)
		return from.Format("2006-01-02"), now.Format("2006-01-02")
	case "month":
		from := now.AddDate(0, -1, 0)
		return from.Format("2006-01-02"), now.Format("2006-01-02")
	default:
		return now.Format("2006-01-02"), now.Format("2006-01-02")
	}
}

// 避免 json 未使用
var _ = json.Marshal

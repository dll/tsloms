package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// AI 原生增强：库存与成本智能分析 + 各模块运维报告数据聚合
// 所有分析均为「数据聚合 → LLM 洞察(带规则兜底)」，纯增量、不打断现有流程。
// ============================================================

// ---- 库存健康分析 ----

// InventorySnapshot 库存分析快照（结构化，供 LLM 与前端）
type InventorySnapshot struct {
	TotalKinds    int            `json:"total_kinds"`     // 物料种类数
	TotalStock    int            `json:"total_stock"`     // 总库存件数
	TotalValue    float64        `json:"total_value"`     // 库存总金额
	LowStock      []MaterialMin  `json:"low_stock"`       // 低于阈值预警
	OutOfStock    []MaterialMin  `json:"out_of_stock"`    // 已缺货
	SlowMoving    []MaterialMin  `json:"slow_moving"`     // 近90天无领用/变动（滞销）
	HighTurnover  []MaterialMin  `json:"high_turnover"`   // 领用最多的（高周转，重点备货）
	CategoryValue []NameValue    `json:"category_value"`  // 按分类库存金额
	RecentIn      int            `json:"recent_in"`       // 近30天入库件数
	RecentUse     int            `json:"recent_use"`      // 近30天领用件数
	MonthUseTrend []MonthlyValue `json:"month_use_trend"` // 近6月领用趋势
}

// MaterialMin 物料简要视图
type MaterialMin struct {
	ID        uint    `json:"id"`
	Name      string  `json:"name"`
	Code      string  `json:"code"`
	Category  string  `json:"category"`
	Stock     int     `json:"stock"`
	Threshold int     `json:"threshold"`
	UnitPrice float64 `json:"unit_price"`
	Use30     int     `json:"use_30"`   // 近30天领用
	LastUse   string  `json:"last_use"` // 最近领用时间(空=从未)
}

// NameValue 名称-数值对
type NameValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// MonthlyValue 月度数值
type MonthlyValue struct {
	Month string `json:"month"` // 2006-01
	Value int    `json:"value"`
}

type InventoryAnalysis struct {
	Snapshot   *InventorySnapshot `json:"snapshot"`
	Insight    string             `json:"insight"`  // LLM/规则洞察
	Insights   []string           `json:"insights"` // 要点列表
	Source     string             `json:"source"`
	TokensUsed int                `json:"tokens_used"`
}

// BuildInventorySnapshot 聚合库存健康数据
func BuildInventorySnapshot() (*InventorySnapshot, error) {
	snap := &InventorySnapshot{}
	var mats []model.Material
	if err := model.DB.Where("status = ?", "active").Find(&mats).Error; err != nil {
		return nil, err
	}
	snap.TotalKinds = len(mats)
	since30 := time.Now().AddDate(0, 0, -30)
	since90 := time.Now().AddDate(0, 0, -90)

	catVal := map[string]float64{}
	for _, m := range mats {
		snap.TotalStock += m.Stock
		v := float64(m.Stock) * m.UnitPrice
		snap.TotalValue += v
		catVal[m.Category] += v
		var use30 int64
		model.DB.Model(&model.MaterialStock{}).
			Where("material_id = ? AND type = ? AND created_at >= ?", m.ID, model.StockTypeUse, since30).
			Select("COALESCE(SUM(ABS(quantity)),0)").Scan(&use30)

		// 最近领用时间
		var lastUse time.Time
		model.DB.Model(&model.MaterialStock{}).
			Where("material_id = ? AND type = ?", m.ID, model.StockTypeUse).
			Order("created_at DESC").Limit(1).Pluck("created_at", &lastUse)
		lastUseStr := ""
		if !lastUse.IsZero() {
			lastUseStr = lastUse.Format("2006-01-02")
		}

		mm := MaterialMin{ID: m.ID, Name: m.Name, Code: m.Code, Category: m.Category,
			Stock: m.Stock, Threshold: m.Threshold, UnitPrice: m.UnitPrice,
			Use30: int(use30), LastUse: lastUseStr}
		if m.Threshold > 0 && m.Stock <= m.Threshold {
			snap.LowStock = append(snap.LowStock, mm)
		}
		if m.Stock == 0 {
			snap.OutOfStock = append(snap.OutOfStock, mm)
		}
		// 滞销：有库存但近90天无领用
		if m.Stock > 0 && int(use30) == 0 && (lastUse.IsZero() || lastUse.Before(since90)) {
			snap.SlowMoving = append(snap.SlowMoving, mm)
		}
		if int(use30) > 0 {
			snap.HighTurnover = append(snap.HighTurnover, mm)
		}
	}
	catList := []NameValue{}
	for k, v := range catVal {
		catList = append(catList, NameValue{Name: k, Value: v})
	}
	sort.Slice(catList, func(i, j int) bool { return catList[i].Value > catList[j].Value })
	snap.CategoryValue = catList

	// 近30天入库/领用件数
	var inC, useC int64
	model.DB.Model(&model.MaterialStock{}).Where("type IN ? AND created_at >= ?",
		[]string{model.StockTypeIn, model.StockTypeGain, model.StockTypeReturn}, since30).
		Select("COALESCE(SUM(quantity),0)").Scan(&inC)
	model.DB.Model(&model.MaterialStock{}).Where("type = ? AND created_at >= ?", model.StockTypeUse, since30).
		Select("COALESCE(SUM(ABS(quantity)),0)").Scan(&useC)
	snap.RecentIn = int(inC)
	snap.RecentUse = int(useC)

	// 近6月领用趋势
	for i := 5; i >= 0; i-- {
		monthStart := time.Now().AddDate(0, -i, 0)
		y, m, _ := monthStart.Date()
		monthStr := fmt.Sprintf("%04d-%02d", y, m)
		next := monthStart.AddDate(0, 1, 0)
		var v int64
		model.DB.Model(&model.MaterialStock{}).
			Where("type = ? AND created_at >= ? AND created_at < ?", model.StockTypeUse, monthStart, next).
			Select("COALESCE(SUM(ABS(quantity)),0)").Scan(&v)
		snap.MonthUseTrend = append(snap.MonthUseTrend, MonthlyValue{Month: monthStr, Value: int(v)})
	}
	return snap, nil
}

// AnalyzeInventory 库存健康 AI 分析（LLM 洞察，规则兜底）
func AnalyzeInventory(userID uint) (InventoryAnalysis, error) {
	snap, err := BuildInventorySnapshot()
	if err != nil {
		return InventoryAnalysis{}, err
	}
	// 生成洞察（LLM）
	client := NewLLMClient(nil)
	prompt := fmt.Sprintf(
		"你是交通信号灯运维平台的库存管理专家。基于以下库存快照（JSON），用中文给出 ≤180字的库存健康洞察与补货建议，"+
			"重点关注：低库存/缺货物料、滞销积压、高周转需备货物料、分类库存金额结构。\n库存快照：%s",
		mustJSON(snap),
	)
	insight := ""
	source := "规则"
	tokens := 0
	if text, tk, err := client.Ask(userID, "inventory", prompt); err == nil {
		insight = text
		source = "LLM"
		tokens = tk
	} else {
		insight = buildInventoryRuleInsight(snap)
	}
	res := InventoryAnalysis{Snapshot: snap, Insight: insight, Insights: splitLines(insight),
		Source: source, TokensUsed: tokens}
	return res, nil
}

func buildInventoryRuleInsight(s *InventorySnapshot) string {
	lines := []string{}
	if len(s.OutOfStock) > 0 {
		lines = append(lines, fmt.Sprintf("有 %d 种物料已缺货，需尽快补货：%s。", len(s.OutOfStock), names(s.OutOfStock)))
	} else if len(s.LowStock) > 0 {
		lines = append(lines, fmt.Sprintf("有 %d 种物料低于预警阈值，建议近期采购：%s。", len(s.LowStock), names(s.LowStock)))
	}
	if len(s.SlowMoving) > 0 {
		lines = append(lines, fmt.Sprintf("有 %d 种物料近90天无领用（积压，占资金），建议调整采购或盘点处置：%s。", len(s.SlowMoving), names(s.SlowMoving)))
	}
	if len(s.HighTurnover) > 0 {
		lines = append(lines, fmt.Sprintf("高周转耗材（近30天领用最多，应保证安全库存）：%s。", names(s.HighTurnover)))
	}
	if s.RecentUse > s.RecentIn && s.RecentIn >= 0 {
		lines = append(lines, fmt.Sprintf("近30天领用 %d 件、入库 %d 件，消耗大于补充，库存呈下降趋势，建议加大采购。", s.RecentUse, s.RecentIn))
	} else if s.RecentIn > 0 {
		lines = append(lines, fmt.Sprintf("近30天入库 %d 件、领用 %d 件，库存补充正常。", s.RecentIn, s.RecentUse))
	}
	if len(lines) == 0 {
		lines = append(lines, "库存整体健康：无缺货、无严重预警、无明显积压，请保持按需采购。")
	}
	return joinLines(lines)
}

// ---- 维修成本归因分析 ----

// CostSnapshot 维修成本快照
type CostSnapshot struct {
	TotalAmount  float64        `json:"total_amount"`   // 统计期内总成本
	TotalCount   int64          `json:"total_count"`    // 费用单数
	ByType       []NameValue    `json:"by_type"`        // 按类型(耗材/人工/交通/其它)
	TopDevices   []DeviceCost   `json:"top_devices"`    // 高成本设备 TOP
	TopFaultType []FaultCost    `json:"top_fault_type"` // 按故障类型成本（关联工单→故障）
	Confirmed    float64        `json:"confirmed"`      // 已确认入账
	Unconfirmed  float64        `json:"unconfirmed"`    // 未确认
	AvgPerOrder  float64        `json:"avg_per_order"`  // 均单成本
	Monthly      []MonthlyMoney `json:"monthly"`        // 近6月成本趋势
}

type DeviceCost struct {
	DeviceHwID uint32  `json:"device_hw_id"`
	Total      float64 `json:"total"`
	Count      int64   `json:"count"`
}

type FaultCost struct {
	FaultType string  `json:"fault_type"`
	Total     float64 `json:"total"`
	Count     int64   `json:"count"`
}

type MonthlyMoney struct {
	Month string  `json:"month"`
	Value float64 `json:"value"`
}

type CostAnalysis struct {
	Snapshot   *CostSnapshot `json:"snapshot"`
	Insight    string        `json:"insight"`
	Insights   []string      `json:"insights"`
	Source     string        `json:"source"`
	TokensUsed int           `json:"tokens_used"`
}

// BuildCostSnapshot 聚合成本数据（默认近90天）
func BuildCostSnapshot(days int) (*CostSnapshot, error) {
	if days <= 0 {
		days = 90
	}
	start := time.Now().AddDate(0, 0, -days)
	snap := &CostSnapshot{}

	var total float64
	var count int64
	model.DB.Model(&model.RepairExpense{}).Where("created_at >= ?", start).
		Select("COALESCE(SUM(amount),0)").Scan(&total)
	model.DB.Model(&model.RepairExpense{}).Where("created_at >= ?", start).Count(&count)
	snap.TotalAmount = total
	snap.TotalCount = count

	byType := map[string]float64{}
	typeRows := []struct {
		Type string
		Sum  float64
	}{}
	model.DB.Model(&model.RepairExpense{}).Where("created_at >= ?", start).
		Select("type, COALESCE(SUM(amount),0) AS sum").Group("type").Scan(&typeRows)
	for _, r := range typeRows {
		byType[r.Type] = r.Sum
		snap.ByType = append(snap.ByType, NameValue{Name: typeLabel(r.Type), Value: r.Sum})
	}
	sort.Slice(snap.ByType, func(i, j int) bool { return snap.ByType[i].Value > snap.ByType[j].Value })

	// TOP 设备
	var devRows []DeviceCost
	model.DB.Model(&model.RepairExpense{}).Where("device_hw_id > 0 AND created_at >= ?", start).
		Select("device_hw_id, COALESCE(SUM(amount),0) AS total, COUNT(*) AS count").
		Group("device_hw_id").Order("total DESC").Limit(10).Scan(&devRows)
	snap.TopDevices = devRows

	// 按故障类型（费用关联工单→工单关联故障→故障类型）
	var fcRows []struct {
		FaultType string
		Sum       float64
		N         int64
	}
	model.DB.Table("repair_expenses AS e").
		Joins("JOIN work_orders AS w ON w.id = e.work_order_id").
		Joins("JOIN fault_records AS f ON f.id = w.fault_id").
		Where("e.created_at >= ?", start).
		Select("f.fault_type, COALESCE(SUM(e.amount),0) AS sum, COUNT(*) AS n").
		Group("f.fault_type").Order("sum DESC").Limit(8).Scan(&fcRows)
	for _, r := range fcRows {
		snap.TopFaultType = append(snap.TopFaultType, FaultCost{FaultType: r.FaultType, Total: r.Sum, Count: r.N})
	}

	var confirmed, unconfirmed float64
	model.DB.Model(&model.RepairExpense{}).Where("confirmed = ? AND created_at >= ?", true, start).
		Select("COALESCE(SUM(amount),0)").Scan(&confirmed)
	model.DB.Model(&model.RepairExpense{}).Where("confirmed = ? AND created_at >= ?", false, start).
		Select("COALESCE(SUM(amount),0)").Scan(&unconfirmed)
	snap.Confirmed = confirmed
	snap.Unconfirmed = unconfirmed
	if count > 0 {
		snap.AvgPerOrder = total / float64(count)
	}

	for i := 5; i >= 0; i-- {
		monthStart := time.Now().AddDate(0, -i, 0)
		y, m, _ := monthStart.Date()
		next := monthStart.AddDate(0, 1, 0)
		var v float64
		model.DB.Model(&model.RepairExpense{}).
			Where("created_at >= ? AND created_at < ?", monthStart, next).
			Select("COALESCE(SUM(amount),0)").Scan(&v)
		snap.Monthly = append(snap.Monthly, MonthlyMoney{Month: fmt.Sprintf("%04d-%02d", y, m), Value: v})
	}
	return snap, nil
}

// AnalyzeCost 成本归因 AI 分析
func AnalyzeCost(userID uint, days int) (CostAnalysis, error) {
	snap, err := BuildCostSnapshot(days)
	if err != nil {
		return CostAnalysis{}, err
	}
	client := NewLLMClient(nil)
	prompt := fmt.Sprintf(
		"你是交通信号灯运维平台的成本管理专家。基于以下维修成本快照（JSON，近90天），用中文给出 ≤180字成本归因与降本建议，"+
			"重点关注：成本结构（耗材/人工/交通/其它占比）、高成本设备、高成本故障类型。\n成本快照：%s",
		mustJSON(snap),
	)
	insight := ""
	source := "规则"
	tokens := 0
	if text, tk, err := client.Ask(userID, "cost", prompt); err == nil {
		insight = text
		source = "LLM"
		tokens = tk
	} else {
		insight = buildCostRuleInsight(snap)
	}
	return CostAnalysis{Snapshot: snap, Insight: insight, Insights: splitLines(insight), Source: source, TokensUsed: tokens}, nil
}

func buildCostRuleInsight(s *CostSnapshot) string {
	var mat, lab, tra, oth float64
	for _, t := range s.ByType {
		switch t.Name {
		case "耗材":
			mat = t.Value
		case "人工":
			lab = t.Value
		case "交通":
			tra = t.Value
		case "其它":
			oth = t.Value
		}
	}
	lines := []string{}
	lines = append(lines, fmt.Sprintf("统计期内维修成本合计 %.2f 元（%d 笔），其中耗材 %.2f、人工 %.2f、交通 %.2f、其它 %.2f。",
		s.TotalAmount, s.TotalCount, mat, lab, tra, oth))
	if len(s.TopDevices) > 0 {
		strong := ""
		for _, d := range s.TopDevices[:min(3, len(s.TopDevices))] {
			strong += fmt.Sprintf("#%d(%.2f) ", d.DeviceHwID, d.Total)
		}
		lines = append(lines, "高成本设备 TOP："+strong+"，建议优先核查其高频故障根因。")
	}
	if len(s.TopFaultType) > 0 {
		lines = append(lines, "高成本故障类型："+fmt.Sprintf("%s(%.2f)", s.TopFaultType[0].FaultType, s.TopFaultType[0].Total)+" 等，建议针对性准备备件与预案。")
	}
	if s.Unconfirmed > 0 {
		lines = append(lines, fmt.Sprintf("尚有 %.2f 元费用未确认入账，建议及时核对确认。", s.Unconfirmed))
	}
	return joinLines(lines)
}

// ---- 工具函数 ----

func typeLabel(t string) string {
	switch t {
	case model.ExpenseTypeMaterial:
		return "耗材"
	case model.ExpenseTypeLabor:
		return "人工"
	case model.ExpenseTypeTraffic:
		return "交通"
	default:
		return "其它"
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		cur += string(ch)
		if ch == '\n' || ch == '。' {
			if t := trimSpace(cur); t != "" {
				out = append(out, t)
			}
			cur = ""
		}
	}
	if t := trimSpace(cur); t != "" {
		out = append(out, t)
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func names(ms []MaterialMin) string {
	out := ""
	for i, m := range ms {
		if i >= 5 {
			out += "等"
			break
		}
		if i > 0 {
			out += "、"
		}
		out += m.Name
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

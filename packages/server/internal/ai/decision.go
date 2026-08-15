package ai

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
)

// ============================================================
// L6 AI 自主决策 —— 决策建议中心（智能体级第一步）
// 核心：实时数据 → 运维健康评分 + 多维决策建议（人力排班/备件采购预测/成本优化）
//      内置规则引擎保底，LLM 增强总结；建议可「一键采纳」执行（半自动）。
// 纯只读计算健康/建议；写动作（采纳→生成采购单等）走 ApplyDecision 显式触发。
// ============================================================

// HealthDimension 健康度单项
type HealthDimension struct {
	Key   string  `json:"key"`   // online_rate/open_faults/overdue/closure/cost/inventory
	Name  string  `json:"name"`  // 中文名
	Score float64 `json:"score"` // 0-100
	Level string  `json:"level"` // good/warn/bad
	Hint  string  `json:"hint"`  // 指标描述
}

// OpsHealth 运维健康评分结果
type OpsHealth struct {
	Total      float64           `json:"total"`      // 综合分 0-100
	Level      string            `json:"level"`      // good/warn/bad
	Grade      string            `json:"grade"`      // 良好/关注/告警
	Dimensions []HealthDimension `json:"dimensions"` // 分维度
	Summary    string            `json:"summary"`    // 一句话总结
	At         time.Time         `json:"at"`
}

// DecisionSuggestion 一条决策建议
type DecisionSuggestion struct {
	Category   string      `json:"category"`    // 人力排班/备件采购/成本优化/设备运维
	Title      string      `json:"title"`       // 建议标题
	Detail     string      `json:"detail"`      // 建议详情
	Priority   string      `json:"priority"`    // high/medium/low
	Action     string      `json:"action"`      // 可执行动作类型: purchase/assign/none
	ActionHint string      `json:"action_hint"` // 一键采纳说明
	Data       []NameValue `json:"data"`        // 附数据(如预测采购明细)
}

// DecisionDecision 决策中心结果
type DecisionCenterResult struct {
	Health     *OpsHealth           `json:"health"`
	Decisions  []DecisionSuggestion `json:"decisions"`
	Summary    string               `json:"summary"` // LLM/规则总结
	Source     string               `json:"source"`
	TokensUsed int                  `json:"tokens_used"`
}

// levelOf 0-100 → 等级
func levelOf(score float64) string {
	if score >= 80 {
		return "good"
	}
	if score >= 60 {
		return "warn"
	}
	return "bad"
}

func gradeText(level string) string {
	switch level {
	case "good":
		return "良好"
	case "warn":
		return "关注"
	default:
		return "告警"
	}
}

// ============================================================
// 1. 运维健康评分
// ============================================================

// BuildOpsHealth 基于实时数据计算运维健康评分（纯规则，不依赖 LLM/DB 测试安全）
func BuildOpsHealth() (*OpsHealth, error) {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	snap := &DailySnapshot{Date: now.Format("2006-01-02")}

	// 设备在线率
	model.DB.Model(&model.Device{}).Count(&snap.Devices.Total)
	model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&snap.Devices.Active)

	// 活跃故障
	model.DB.Model(&model.FaultRecord{}).Where("status IN ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Count(&snap.Faults.Active)
	model.DB.Model(&model.FaultRecord{}).Where("first_seen >= ?", today).Count(&snap.Faults.TodayNew)
	model.DB.Model(&model.FaultRecord{}).Where("status = ?", model.FaultStatusResolved).Count(&snap.Faults.Total)

	// 工单 + 超时
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusPending).Count(&snap.WorkOrders.Pending)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusProcessing).Count(&snap.WorkOrders.Processing)
	model.DB.Model(&model.WorkOrder{}).Where("status = ?", model.WorkOrderStatusCompleted).Count(&snap.WorkOrders.Completed)
	model.DB.Model(&model.WorkOrder{}).Count(&snap.WorkOrders.Total)
	pendingOverdue := now.Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	procOverdue := now.Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	model.DB.Model(&model.WorkOrder{}).
		Where("(status = ? AND created_at < ?) OR (status = ? AND created_at < ?)",
			model.WorkOrderStatusPending, pendingOverdue, model.WorkOrderStatusProcessing, procOverdue).
		Count(&snap.OverdueOrders)

	// 平均闭环时长（近30天）
	var closure []struct{ CreatedAt, ClosedAt time.Time }
	model.DB.Model(&model.WorkOrder{}).
		Select("created_at, closed_at").
		Where("status = ? AND closed_at IS NOT NULL AND created_at >= ?",
			model.WorkOrderStatusCompleted, time.Now().AddDate(0, 0, -30)).
		Find(&closure)
	var closureSum float64
	for _, cl := range closure {
		closureSum += cl.ClosedAt.Sub(cl.CreatedAt).Hours()
	}
	if len(closure) > 0 {
		snap.AvgClosureHours = closureSum / float64(len(closure))
	}

	// 近30天成本（repair_expenses 无 occurred_at 列，用 created_at）
	var cost30 float64
	model.DB.Model(&model.RepairExpense{}).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -30)).
		Select("COALESCE(SUM(amount),0)").Scan(&cost30)

	// 库存低库存率
	lowStockRatio := 0.0
	var mats []model.Material
	if err := model.DB.Where("status = ?", "active").Find(&mats).Error; err == nil && len(mats) > 0 {
		low := 0
		for _, m := range mats {
			if m.Stock < m.Threshold {
				low++
			}
		}
		lowStockRatio = float64(low) / float64(len(mats)) * 100
	}

	dims := make([]HealthDimension, 0, 6)

	// 设备在线
	onlineScore := 100.0
	if snap.Devices.Total > 0 {
		rate := float64(snap.Devices.Active) / float64(snap.Devices.Total) * 100
		onlineScore = rate
		if rate >= 80 {
			onlineScore = 80 + (rate-80)/20*20 // 80-100 映射 80-100
		}
	}
	dims = append(dims, HealthDimension{
		Key: "online_rate", Name: "设备在线率",
		Score: clamp01(onlineScore), Level: levelOf(onlineScore),
		Hint: fmt.Sprintf("在线 %d/%d 台", snap.Devices.Active, snap.Devices.Total),
	})

	// 活跃故障（越少越好，0 记 90+）
	openScore := 100.0
	switch {
	case snap.Faults.Active == 0:
		openScore = 90
	case snap.Faults.Active == 1:
		openScore = 80
	case snap.Faults.Active <= 3:
		openScore = 65
	case snap.Faults.Active <= 5:
		openScore = 50
	default:
		openScore = 35
	}
	dims = append(dims, HealthDimension{
		Key: "open_faults", Name: "未解决故障",
		Score: openScore, Level: levelOf(openScore),
		Hint: fmt.Sprintf("活跃故障 %d 起", snap.Faults.Active),
	})

	// 工单超时（越多越差）
	overdueScore := 100.0
	switch {
	case snap.OverdueOrders == 0:
		overdueScore = 95
	case snap.OverdueOrders <= 2:
		overdueScore = 70
	case snap.OverdueOrders <= 5:
		overdueScore = 50
	default:
		overdueScore = 30
	}
	dims = append(dims, HealthDimension{
		Key: "overdue", Name: "工单超时",
		Score: overdueScore, Level: levelOf(overdueScore),
		Hint: fmt.Sprintf("超时工单 %d 个 / 共 %d", snap.OverdueOrders, snap.WorkOrders.Total),
	})

	// 闭环效率（平均闭环时长, ≤24h 优秀, ≤48 良好, ≤72 关注, 更高告警）
	closureScore := 100.0
	switch {
	case snap.AvgClosureHours == 0:
		closureScore = 85
	case snap.AvgClosureHours <= 24:
		closureScore = 95
	case snap.AvgClosureHours <= 48:
		closureScore = 80
	case snap.AvgClosureHours <= 72:
		closureScore = 60
	default:
		closureScore = 45
	}
	dims = append(dims, HealthDimension{
		Key: "closure", Name: "工单闭环",
		Score: closureScore, Level: levelOf(closureScore),
		Hint: fmt.Sprintf("平均闭环 %.1f 小时", snap.AvgClosureHours),
	})

	// 成本健康（近30天成本，越高越差；无数据记中位）
	costScore := 100.0
	switch {
	case cost30 <= 0:
		costScore = 85
	case cost30 < 5000:
		costScore = 90
	case cost30 < 20000:
		costScore = 75
	case cost30 < 50000:
		costScore = 60
	default:
		costScore = 45
	}
	dims = append(dims, HealthDimension{
		Key: "cost", Name: "运维成本",
		Score: costScore, Level: levelOf(costScore),
		Hint: fmt.Sprintf("近30天 %.2f 元", cost30),
	})

	// 库存充足（低库存率越低越好）
	invScore := 100.0
	switch {
	case lowStockRatio <= 5:
		invScore = 90
	case lowStockRatio <= 20:
		invScore = 75
	case lowStockRatio <= 40:
		invScore = 55
	default:
		invScore = 35
	}
	dims = append(dims, HealthDimension{
		Key: "inventory", Name: "备件库存",
		Score: invScore, Level: levelOf(invScore),
		Hint: fmt.Sprintf("低库存物料占比 %.0f%%", lowStockRatio),
	})

	total := 0.0
	for _, d := range dims {
		total += d.Score
	}
	total = total / float64(len(dims))
	lvl := levelOf(total)

	// 一句话总结（规则）
	worst := dims[0]
	for _, d := range dims {
		if d.Score < worst.Score {
			worst = d
		}
	}
	summary := fmt.Sprintf("运维健康综合评分 %.0f 分（%s）。较薄弱项：%s（%s）。",
		total, gradeText(lvl), worst.Name, worst.Hint)

	return &OpsHealth{
		Total: total, Level: lvl, Grade: gradeText(lvl),
		Dimensions: dims, Summary: summary, At: now,
	}, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// ============================================================
// 2. 决策建议（人力排班 / 备件采购预测 / 成本优化 / 设备运维）
// ============================================================

// repairerWorkload 按处理人统计未完成工单
type repairerWorkload struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	OpenCount int    `json:"open_count"`
	Completed int    `json:"completed"`
}

// BuildDecisions 生成决策建议（纯规则，返回结构化、可一键采纳的建议）
func BuildDecisions() ([]DecisionSuggestion, error) {
	now := time.Now()
	since30 := now.AddDate(0, 0, -30)
	decisions := make([]DecisionSuggestion, 0)

	// ---- 人力排班：处理人工作量分布 ----
	var workOrders []model.WorkOrder
	model.DB.Where("status IN ?", []string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).Find(&workOrders)
	load := map[uint]*repairerWorkload{}
	for _, wo := range workOrders {
		if wo.AssigneeID == nil {
			continue
		}
		l, ok := load[*wo.AssigneeID]
		if !ok {
			var u model.User
			name := ""
			if err := model.DB.Select("id, username").First(&u, *wo.AssigneeID).Error; err == nil {
				name = u.Username
			}
			l = &repairerWorkload{UserID: *wo.AssigneeID, Username: name}
			load[*wo.AssigneeID] = l
		}
		l.OpenCount++
	}
	if len(load) > 0 {
		sorted := make([]*repairerWorkload, 0, len(load))
		for _, l := range load {
			sorted = append(sorted, l)
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].OpenCount > sorted[j].OpenCount })
		busiest := sorted[0]
		totalOpen := len(workOrders)
		if busiest.OpenCount >= 3 || (len(sorted) > 1 && busiest.OpenCount >= 2*sorted[len(sorted)-1].OpenCount+1) {
			priority := "medium"
			if busiest.OpenCount >= 4 {
				priority = "high"
			}
			decisions = append(decisions, DecisionSuggestion{
				Category: "人力排班", Title: fmt.Sprintf("维修人员 %s 负载偏高，建议分派", busiest.Username),
				Detail: fmt.Sprintf("处理人 %s 当前承担 %d 个未完成工单（占全部 %d 个），建议将部分任务转派给负载较低的维修人员，或安排增援。",
					busiest.Username, busiest.OpenCount, totalOpen),
				Priority: priority, Action: "assign", ActionHint: "可在工单列表将部分工单指派给空闲人员",
				Data: []NameValue{{Name: busiest.Username, Value: float64(busiest.OpenCount)}},
			})
		}
	}

	// ---- 备件采购预测：近30天领用 + 低库存 → 预测下月需求 ----
	var useAgg []struct {
		MaterialID uint
		Sum        int
	}
	model.DB.Model(&model.MaterialStock{}).
		Select("material_id, SUM(ABS(quantity)) as sum").
		Where("type = ? AND created_at >= ?", model.StockTypeUse, since30).
		Group("material_id").Scan(&useAgg)

	var mats []model.Material
	model.DB.Where("status = ?", "active").Find(&mats)
	matByID := map[uint]model.Material{}
	for _, m := range mats {
		matByID[m.ID] = m
	}
	// 组装预测：近30天消耗 × 1（下月预测同量）vs 当前库存+阈值 → 建议补货
	for _, u := range useAgg {
		m, ok := matByID[u.MaterialID]
		if !ok {
			continue
		}
		forecast := u.Sum // 下月预测 ≈ 本月消耗
		need := forecast - m.Stock
		if forecast > 0 && need > 0 {
			priority := "medium"
			if m.Stock <= m.Threshold || need >= 2*forecast/3 {
				priority = "high"
			}
			decisions = append(decisions, DecisionSuggestion{
				Category: "备件采购", Title: fmt.Sprintf("建议补货：%s", m.Name),
				Detail: fmt.Sprintf("近30天消耗 %d 件，当前库存 %d，预测下月需求约 %d 件，缺口 %d 件，建议提前采购备货（单价 %.2f 元）。",
					u.Sum, m.Stock, forecast, need, m.UnitPrice),
				Priority: priority, Action: "purchase", ActionHint: "可一键生成采购草稿单",
				Data: []NameValue{
					{Name: m.Name, Value: float64(need)},
					{Name: "单价", Value: m.UnitPrice},
				},
			})
		}
	}
	// 低库存但近期无消耗的（低于阈值且库存过少）也提示
	for _, m := range mats {
		if m.Stock < m.Threshold && m.Stock <= 2 {
			decisions = append(decisions, DecisionSuggestion{
				Category: "备件采购", Title: fmt.Sprintf("库存告急：%s", m.Name),
				Detail: fmt.Sprintf("物料 %s 当前库存 %d，低于预警阈值 %d，建议尽快采购补充防止缺料。",
					m.Name, m.Stock, m.Threshold),
				Priority: "high", Action: "purchase", ActionHint: "可一键生成采购草稿单",
				Data: []NameValue{{Name: m.Name, Value: float64(m.Threshold - m.Stock + 1)}},
			})
		}
	}

	// ---- 成本优化：近30天成本按类型/设备归因（repair_expenses 无 occurred_at 列，用 created_at） ----
	var costByType []NameValue
	model.DB.Model(&model.RepairExpense{}).
		Select("type as name, COALESCE(SUM(amount),0) as value").
		Where("created_at >= ?", since30).
		Group("type").Scan(&costByType)
	var costTopDevice []struct {
		DeviceHwID uint32
		Total      float64
	}
	model.DB.Model(&model.RepairExpense{}).
		Select("device_hw_id, COALESCE(SUM(amount),0) as total").
		Where("created_at >= ?", since30).
		Group("device_hw_id").Order("total DESC").Limit(3).Scan(&costTopDevice)

	var maxType NameValue
	for _, t := range costByType {
		if t.Value > maxType.Value {
			maxType = t
		}
	}
	if maxType.Value > 3000 && maxType.Name != "" {
		decisions = append(decisions, DecisionSuggestion{
			Category: "成本优化", Title: fmt.Sprintf("「%s」类成本占比最高，建议优化", typeLabel(maxType.Name)),
			Detail: fmt.Sprintf("近30天「%s」类维修成本 %.2f 元为各类别最高，建议核查是否存在可替代耗材 / 优化维保周期或供应商比价。",
				typeLabel(maxType.Name), maxType.Value),
			Priority: "medium", Action: "none",
			Data: []NameValue{{Name: typeLabel(maxType.Name), Value: maxType.Value}},
		})
	}
	if len(costTopDevice) > 0 && costTopDevice[0].Total > 3000 {
		d := costTopDevice[0]
		decisions = append(decisions, DecisionSuggestion{
			Category: "成本优化", Title: fmt.Sprintf("设备 HW#%d 维修成本最高", d.DeviceHwID),
			Detail: fmt.Sprintf("设备 #%d 近30天维修成本 %.2f 元居首，建议排查该设备是否存在反复故障或已到生命周期末期，考虑更换或重点维护。",
				d.DeviceHwID, d.Total),
			Priority: "medium", Action: "none",
			Data: []NameValue{{Name: fmt.Sprintf("设备#%d", d.DeviceHwID), Value: d.Total}},
		})
	}

	// ---- 设备运维：高风险/离线设备 ----
	var offline int64
	model.DB.Model(&model.Device{}).Where("online_status = ?", false).Count(&offline)
	if offline > 0 {
		decisions = append(decisions, DecisionSuggestion{
			Category: "设备运维", Title: fmt.Sprintf("有 %d 台设备离线，建议核查", offline),
			Detail:   fmt.Sprintf("当前 %d 台设备处于离线状态，建议安排巡检确认供电/网络/通信状态，避免漏报故障。", offline),
			Priority: "medium", Action: "none",
		})
	}

	// 按优先级排序（high 在前）
	sort.SliceStable(decisions, func(i, j int) bool {
		return prioRank(decisions[i].Priority) < prioRank(decisions[j].Priority)
	})

	// 控制返回数量（最多 6 条，避免过量）
	if len(decisions) > 6 {
		decisions = decisions[:6]
	}
	return decisions, nil
}

func prioRank(p string) int {
	switch p {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

// ============================================================
// 3. 决策中心编排（LLM 总结增强 + 规则保底）
// ============================================================

// DecisionCenter 生成运维健康评分 + 决策建议（LLM 总结，规则保底）
func DecisionCenter(userID uint) (*DecisionCenterResult, error) {
	health, err := BuildOpsHealth()
	if err != nil {
		return nil, err
	}
	decisions, err := BuildDecisions()
	if err != nil {
		return nil, err
	}

	res := &DecisionCenterResult{
		Health: health, Decisions: decisions,
		Source: "规则", TokensUsed: 0,
	}

	// LLM 增强总结（仅在有 DB 时调用，避免测试 panic）
	client := NewLLMClient(nil)
	if model.DB != nil && len(decisions) > 0 {
		var lines []string
		lines = append(lines, fmt.Sprintf("健康评分%.0f分(%s)。", health.Total, health.Grade))
		for _, d := range decisions {
			lines = append(lines, fmt.Sprintf("- %s：%s", d.Category, d.Title))
		}
		prompt := "你是交通信号灯运维决策专家。基于以下实时运维健康与决策建议，用不超过120字的中文给出凝练的运营决策要点（抓重点、可执行）。不要输出编号列表外的建议。\n" + strings.Join(lines, "\n")
		if resp, _, err := client.Ask(userID, "decision_summary", prompt); err == nil {
			txt := strings.TrimSpace(stripJSONFence(resp))
			if strings.TrimSpace(txt) != "" && strings.TrimSpace(txt) != "{}" {
				res.Summary = txt
			}
		}
	}
	if res.Summary == "" {
		res.Summary = health.Summary
	}
	return res, nil
}

// ============================================================
// 4. 半自动执行：一键采纳（生成采购草稿单）
// ============================================================

// AdoptDecisionApply 采纳一条建议并执行（当前支持备件采购 → 生成采购草稿单）
// decision: 传入建议的 Data（物料名/数量）+ 可选 supplier_id
// 返回新采购单号
func AdoptDecisionApply(userID uint, category, title string, supplierID uint, items []PurchaseLine) (string, error) {
	if category != "备件采购" {
		return "", fmt.Errorf("暂不支持自动执行该类型建议：%s", category)
	}
	if len(items) == 0 {
		return "", fmt.Errorf("建议中无可执行的采购明细")
	}
	if supplierID == 0 {
		var anySupplier model.Supplier
		if err := model.DB.First(&anySupplier).Error; err != nil {
			return "", fmt.Errorf("无可用供应商，请先在供应商管理中创建")
		}
		supplierID = anySupplier.ID
	}

	var totalAmount float64
	poItems := make([]model.PurchaseOrderItem, 0, len(items))
	for _, it := range items {
		if it.Quantity <= 0 {
			continue
		}
		var m model.Material
		if err := model.DB.Where("name = ?", it.MaterialName).First(&m).Error; err != nil {
			return "", fmt.Errorf("物料「%s」不存在", it.MaterialName)
		}
		price := it.Price
		if price <= 0 {
			price = m.UnitPrice
		}
		amt := float64(it.Quantity) * price
		totalAmount += amt
		poItems = append(poItems, model.PurchaseOrderItem{
			MaterialID: m.ID, MaterialName: m.Name, Quantity: it.Quantity,
			Price: price, Amount: amt,
		})
	}
	if len(poItems) == 0 {
		return "", fmt.Errorf("无有效的采购明细")
	}

	var po model.PurchaseOrder
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		orderNo := model.NextBizNo(tx, "purchase_orders", "PO")
		po = model.PurchaseOrder{
			OrderNo: orderNo, SupplierID: supplierID, Status: model.PurchaseStatusDraft,
			TotalAmount: totalAmount, Operator: "AI决策", Note: "AI 决策中心一键采纳生成",
		}
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		for i := range poItems {
			poItems[i].OrderID = po.ID
			if err := tx.Create(&poItems[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// 记录操作日志（写操作）
	if model.DB != nil {
		var uname string
		var u model.User
		if err := model.DB.Select("id, username").First(&u, userID).Error; err == nil {
			uname = u.Username
		}
		model.DB.Create(&model.OperationLog{
			UserID: userID, Username: uname, Action: model.OpCreate,
			Target: fmt.Sprintf("purchase-order/%d", po.ID), Detail: "AI决策一键采纳：生成采购单 " + po.OrderNo,
		})
	}
	return po.OrderNo, nil
}

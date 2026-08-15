package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// L5 AI 自然语言交互（对话级）
//   - 顶部 AI 助手：用户自然语言查询/命令 → 意图识别 → 工具执行 → 结构化回答
//   - 查询类：路口故障排行、设备状态、工单统计、费用归因（读真实数据）
//   - 命令类：自然语言建故障单、命令式建工单（真实写入）
//   - 兜底：规则回答（提示可用的查询/命令）；无 LLM 密钥时全部规则执行
// ============================================================

// NLIntent LLM 意图识别结果
type NLIntent struct {
	Intent string            `json:"intent"` // query / command / fallback
	Tool   string            `json:"tool"`   // fault_rank/device_status/workorder_stats/expense_summary/create_fault/create_workorder
	Params map[string]string `json:"params"` // 参数（如 days/intersection/hw_id/desc/level/device）
}

// NLAnswer NL 交互最终回答（已执行工具后的结果）
type NLAnswer struct {
	Reply      string         `json:"reply"`       // 面向用户的自然语言回答
	Intent     string         `json:"intent"`      // 实际执行的意图
	Tool       string         `json:"tool"`        // 实际执行的工具
	Data       map[string]any `json:"data"`        // 结构化数据（前端可渲染表格/图表）
	Source     string         `json:"source"`      // LLM / 规则
	TokensUsed int            `json:"tokens_used"` // 本次消耗 token
	DidWrite   bool           `json:"did_write"`   // 是否执行了写操作（命令类）
	CreatedID  uint           `json:"created_id"`  // 命令类创建的记录 ID（故障单/工单）
	Confidence float64        `json:"confidence"`  // 意图置信度
}

// NLKnowledge 知识库条目（RAG：操作手册/设备要点）
type NLKnowledge struct {
	Keywords []string `json:"keywords"`
	Answer   string   `json:"answer"`
}

// nlKnowledgeBase 内置知识库（RAG 检索源），可按需扩展
var nlKnowledgeBase = []NLKnowledge{
	{Keywords: []string{"如何", "新建", "工单", "报修", "流程", "操作"}, Answer: "报修/建单流程：在「故障管理」确认故障后系统自动生成工单，或在「工单管理」新建工单（选择设备，AI 可推荐优先级/备件/维修人）。维修完成后在工单完成弹窗使用 AI 一键生成维修小结。"},
	{Keywords: []string{"设备", "离线", "在线", "状态", "签到", "health"}, Answer: "设备状态查询：可用「最近XX设备状态」等方式让 AI 助手查询指定设备在线/离线、最后签到时间；地图页面有红黄绿信号灯图例展示在线状态。"},
	{Keywords: []string{"采购", "领料", "库存", "缺货", "供应商"}, Answer: "采购/库存：在「库存与成本」模块管理物料与采购；采购新建时 AI 会校验数量/金额合理性并给供应商建议；低库存/缺货会触发站内预警通知。"},
	{Keywords: []string{"固件", "升级", "OTA", "版本"}, Answer: "固件升级：在「固件管理」发布固件包后，可对在线设备发起升级任务（须设备支持 OTA 协议）。"},
	{Keywords: []string{"报告", "日报", "巡检", "通知"}, Answer: "运维日报：系统每日定时（默认 08:00）自动生成运维日报并推送站内通知；也可在 AI 工作台手动生成各模块报告。顶部铃铛可查看日报/预警。"},
	{Keywords: []string{"费用", "成本", "耗材", "统计"}, Answer: "费用归因：可用「最近N天费用汇总」让 AI 助手统计维修费用构成（耗材/人工/交通/其他）与高成本设备/故障类型。"},
}

// InterpretNL 解析用户自然语言，识别意图与工具，并执行对应查询/命令。
// 流程：LLM 识别意图 JSON → 按 tool 执行真实数据工具 → 组回答。
// 无 LLM 密钥/调用失败时：规则识别（关键词匹配）→ 执行工具，保证始终可用。
func InterpretNL(userID uint, text string) NLAnswer {
	text = strings.TrimSpace(text)
	if text == "" {
		return NLAnswer{Reply: "请输入您要查询或操作的内容，例如「最近一周哪些路口故障最多」「查询设备123456状态」「报修：人民路口黄灯不亮，位置在人民路与建设路交叉口」。", Intent: "fallback", Tool: "", Source: "规则"}
	}

	// “怎么/如何/怎样…” 这类操作咨询直接走知识库，避免 LLM 误判为建单命令
	if isHowQuestion(text) {
		return runKnowledge(text)
	}

	client := NewLLMClient(nil)
	// 先试 LLM 意图识别
	if model.DB != nil {
		if it, err := classifyIntent(client, userID, text); err == nil {
			return runTool(userID, it, text)
		}
	}
	// 规则识别兜底
	it := ruleClassify(text)
	return runTool(userID, it, text)
}

// isHowQuestion 判断是否操作咨询类问题（“怎么/如何/怎样”开头）
func isHowQuestion(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "怎么") || strings.HasPrefix(t, "如何") || strings.HasPrefix(t, "怎样") {
		return true
	}
	return strings.Contains(t, "怎么建") || strings.Contains(t, "如何建") || strings.Contains(t, "怎么新建") ||
		strings.Contains(t, "如何新建") || strings.Contains(t, "怎么操作") || strings.Contains(t, "如何操作")
}

// classifyIntent 用 LLM 将自然语言标注为结构化意图 JSON
func classifyIntent(client *LLMClient, userID uint, text string) (NLIntent, error) {
	prompt := `你是运维系统的意图识别器。把用户的话映射为 JSON（只输出 JSON，不要多余文字）。
可选 intent: query(查询)/command(命令建单建故障)/fallback(其他咨询)。
可选 tool:
- fault_rank(路口故障排行,params: days=天数,intersection=路口)
- device_status(设备状态,params: hw_id=设备硬件ID或名称)
- workorder_stats(工单统计,params: days=天数)
- expense_summary(费用归因,params: days=天数)
- ops_health(运维健康评分/决策建议,params: 无)
- anomaly_stream(实时异常流检测:报文告警/故障/超时工单/离线设备,params: hours=小时)
- create_fault(自然语言报修→建故障单,params: desc=故障描述,device=设备ID或路口,hw_id=硬件ID,level=critical/normal)
- create_workorder(命令式建工单,params: device=设备ID或路口,hw_id=硬件ID,note=备注)
- 查询/统计类归 query，创建工单/故障归 command，业务知识/操作咨询归 fallback(tool=知识库)。
用户输入：` + text
	resp, _, err := client.Ask(userID, "nl_intent", prompt)
	if err != nil {
		return NLIntent{}, err
	}
	var it NLIntent
	if err := json.Unmarshal([]byte(stripJSONFence(resp)), &it); err != nil {
		return NLIntent{}, err
	}
	if it.Intent == "" {
		it.Intent = "fallback"
	}
	if it.Tool == "" {
		it.Tool = "kb"
	}
	if it.Params == nil {
		it.Params = map[string]string{}
	}
	return it, nil
}

// ruleClassify 无 LLM 时的关键词规则识别
func ruleClassify(text string) NLIntent {
	it := NLIntent{Intent: "fallback", Tool: "kb", Params: map[string]string{}}
	// 咨询/操作疑问优先（怎么/如何/能..., 不误判为命令）
	if strings.HasPrefix(text, "怎么") || strings.HasPrefix(text, "如何") || strings.HasPrefix(text, "怎样") ||
		strings.Contains(text, "怎么建") || strings.Contains(text, "如何建") || strings.Contains(text, "怎么新建") ||
		strings.Contains(text, "如何新建") || strings.Contains(text, "怎么操作") || strings.Contains(text, "如何操作") {
		return it
	}
	switch {
	case strings.Contains(text, "异常") || strings.Contains(text, "告警") || strings.Contains(text, "实时") && strings.Contains(text, "事件") || strings.Contains(text, "有哪些问题") || strings.Contains(text, "异常流"):
		it.Intent = "query"
		it.Tool = "anomaly_stream"
	case strings.Contains(text, "健康评分") || strings.Contains(text, "健康分") || strings.Contains(text, "运维健康") || strings.Contains(text, "决策建议") || strings.Contains(text, "健康度") || strings.Contains(text, "健康状态"):
		it.Intent = "query"
		it.Tool = "ops_health"
	case strings.Contains(text, "路口") && (strings.Contains(text, "故障") || strings.Contains(text, "排行") || strings.Contains(text, "最多") || strings.Contains(text, "TOP")):
		it.Intent = "query"
		it.Tool = "fault_rank"
		it.Params["days"] = extractDays(text)
	case strings.Contains(text, "费用") || strings.Contains(text, "成本") || strings.Contains(text, "耗材"):
		it.Intent = "query"
		it.Tool = "expense_summary"
		it.Params["days"] = extractDays(text)
	case (strings.Contains(text, "工单") || strings.Contains(text, "工作单")) && (strings.Contains(text, "统计") || strings.Contains(text, "几个") || strings.Contains(text, "多少") || strings.Contains(text, "状态")):
		it.Intent = "query"
		it.Tool = "workorder_stats"
		it.Params["days"] = extractDays(text)
	case strings.Contains(text, "状态") || strings.Contains(text, "在线") || strings.Contains(text, "离线") || (strings.Contains(text, "设备") && strings.Contains(text, "查询")):
		it.Intent = "query"
		it.Tool = "device_status"
		it.Params["hw_id"] = extractHwID(text)
	case strings.Contains(text, "报修") || strings.Contains(text, "故障：") || strings.Contains(text, "故障:") || strings.Contains(text, "不亮") || strings.Contains(text, "坏了"):
		it.Intent = "command"
		it.Tool = "create_fault"
		it.Params["desc"] = cleanDesc(text)
		it.Params["level"] = mapLevel(text)
	case strings.Contains(text, "建工单") || strings.Contains(text, "新建工单") || strings.Contains(text, "建工作单") || strings.Contains(text, "下工单") || strings.Contains(text, "派单"):
		it.Intent = "command"
		it.Tool = "create_workorder"
		it.Params["device"] = text
		it.Params["note"] = text
	}
	return it
}

// runTool 按意图执行真实工具并组回答
func runTool(userID uint, it NLIntent, raw string) NLAnswer {
	// 命令类写操作必须校验业务权限（RBAC 防线：防止自然语言绕过接口权限）
	switch it.Tool {
	case "create_fault":
		if deny, ans := nlRequirePerm(userID, "fault:update"); deny {
			return ans
		}
	case "create_workorder":
		if deny, ans := nlRequirePerm(userID, "workorder:create"); deny {
			return ans
		}
	default:
		// 只读工具（fault_rank/device_status/workorder_stats/expense_summary/ops_health/anomaly_stream）放行
	}
	// 命令类写操作需校验权限（由调用方 RequirePerm 控制读；写操作在此轻量确认设备存在）
	switch it.Tool {
	case "fault_rank":
		return runFaultRank(userID, it)
	case "device_status":
		return runDeviceStatus(userID, it)
	case "workorder_stats":
		return runWorkOrderStats(userID, it)
	case "expense_summary":
		return runExpenseSummary(userID, it)
	case "ops_health":
		return runOpsHealth(userID, it)
	case "anomaly_stream":
		return runAnomalyStream(userID, it)
	case "create_fault":
		return runCreateFault(userID, it, raw)
	case "create_workorder":
		return runCreateWorkOrder(userID, it, raw)
	default: // kb / fallback
		return runKnowledge(raw)
	}
}

// nlRequirePerm NL 命令类工具的业务权限校验。
// 无权限时返回 (true, 权限不足回答)；有权限时返回 (false, 空)。
// 不调用 LLM，纯规则，保证写操作不越权。
func nlRequirePerm(userID uint, perm string) (bool, NLAnswer) {
	// 无数据库（如只读/单测环境）时拒绝写命令，避免 nil 指针与方法内 DB 依赖
	if model.DB == nil {
		return true, NLAnswer{Reply: "系统当前不可用，无法执行写操作", Intent: "command", Tool: "", Source: "规则"}
	}
	perms, err := model.EffectivePermissions(userID)
	if err != nil {
		return true, NLAnswer{Reply: "权限校验失败，请稍后再试", Intent: "command", Tool: "", Source: "规则"}
	}
	if !perms[perm] {
		return true, NLAnswer{Reply: "抱歉，您没有执行该操作的权限（需要权限：" + perm + "），请联系管理员。", Intent: "command", Tool: "", Source: "规则"}
	}
	return false, NLAnswer{}
}

// ---- 查询类工具（只读真实数据） ----

func runFaultRank(userID uint, it NLIntent) NLAnswer {
	days := parseDays(it.Params["days"], 7)
	from := time.Now().AddDate(0, 0, -days)
	var rows []struct {
		Intersection string
		Cnt          int64
	}
	model.DB.Model(&model.FaultRecord{}).
		Joins("LEFT JOIN devices d ON d.hw_id = fault_records.device_hw_id").
		Where("fault_records.first_seen >= ?", from).
		Select("COALESCE(NULLIF(d.intersection,''), '未标注路口') AS intersection, COUNT(*) AS cnt").
		Group("intersection").Order("cnt DESC").Limit(10).Scan(&rows)

	total := int64(0)
	data := []map[string]any{}
	for _, r := range rows {
		total += r.Cnt
		data = append(data, map[string]any{"intersection": r.Intersection, "count": r.Cnt})
	}
	reply := fmt.Sprintf("最近 %d 天共记录故障 %d 起", days, total)
	if len(data) > 0 {
		reply += "。故障最多："
		for i, r := range rows {
			if i == 3 {
				reply += "…"
				break
			}
			reply += fmt.Sprintf("「%s」%d起、", r.Intersection, r.Cnt)
		}
		reply = strings.TrimRight(reply, "、") + "。"
	}
	if len(data) == 0 {
		reply += "，暂无故障数据。"
	}
	return NLAnswer{Reply: reply, Intent: "query", Tool: "fault_rank", Data: map[string]any{"list": data, "days": days, "total": total}, Source: "规则"}
}

func runDeviceStatus(userID uint, it NLIntent) NLAnswer {
	hw := parseHwID(it.Params["hw_id"])
	var list []model.Device
	q := model.DB
	if hw > 0 {
		q = q.Where("hw_id = ?", hw)
	} else {
		term := it.Params["hw_id"]
		if term == "" {
			term = it.Params["device"]
		}
		if term != "" {
			q = q.Where("intersection LIKE ? OR CAST(hw_id AS TEXT) LIKE ?", "%"+term+"%", "%"+term+"%")
		} else {
			q = q.Limit(5)
		}
	}
	q.Order("id").Limit(10).Find(&list)

	if len(list) == 0 {
		return NLAnswer{Reply: "未找到匹配的设备，请提供设备硬件 ID（如 123456）或路口名称。", Intent: "query", Tool: "device_status", Source: "规则"}
	}
	data := []map[string]any{}
	reply := "设备状态："
	for _, d := range list {
		st := "离线"
		if d.OnlineStatus {
			st = "在线"
		}
		watch := ""
		if d.IsWatched {
			watch = "（已关注）"
		}
		last := "-"
		if d.LastCheckinAt != nil {
			last = d.LastCheckinAt.Format("01-02 15:04")
		}
		intersection := d.Intersection
		if intersection == "" {
			intersection = fmt.Sprintf("设备#%d", d.HwID)
		}
		data = append(data, map[string]any{"hw_id": d.HwID, "intersection": intersection, "online": d.OnlineStatus, "last_checkin": last, "watched": d.IsWatched})
		reply += fmt.Sprintf("「%s」%s%s（最后签到 %s）；", intersection, st, watch, last)
	}
	reply = strings.TrimRight(reply, "；") + "。"
	return NLAnswer{Reply: reply, Intent: "query", Tool: "device_status", Data: map[string]any{"list": data}, Source: "规则"}
}

func runWorkOrderStats(userID uint, it NLIntent) NLAnswer {
	var wo model.WorkOrder
	total := int64(0)
	model.DB.Model(&wo).Count(&total)

	stats := map[string]int64{}
	var rows []struct {
		Status string
		Cnt    int64
	}
	model.DB.Model(&wo).Select("status, COUNT(*) AS cnt").Group("status").Scan(&rows)
	for _, r := range rows {
		stats[r.Status] = r.Cnt
	}

	overdue := int64(0)
	overdueH := time.Now().Add(-24 * time.Hour)
	model.DB.Model(&wo).Where("status = ? AND created_at < ?", model.WorkOrderStatusPending, overdueH).Count(&overdue)
	overdueProc := int64(0)
	overdueH2 := time.Now().Add(-48 * time.Hour)
	model.DB.Model(&wo).Where("status = ? AND created_at < ?", model.WorkOrderStatusProcessing, overdueH2).Count(&overdueProc)

	reply := fmt.Sprintf("当前工单共 %d 个：待处理 %d、处理中 %d、已完成 %d、已驳回 %d", total,
		stats[model.WorkOrderStatusPending], stats[model.WorkOrderStatusProcessing],
		stats[model.WorkOrderStatusCompleted], stats[model.WorkOrderStatusRejected])
	if overdue+overdueProc > 0 {
		reply += fmt.Sprintf("；超时 %d 个（待处理超 24h %d、处理中超 48h %d）", overdue+overdueProc, overdue, overdueProc)
	}
	return NLAnswer{Reply: reply, Intent: "query", Tool: "workorder_stats", Data: map[string]any{"total": total, "stats": stats, "overdue": overdue + overdueProc}, Source: "规则"}
}

func runExpenseSummary(userID uint, it NLIntent) NLAnswer {
	days := parseDays(it.Params["days"], 30)
	from := time.Now().AddDate(0, 0, -days)
	var rows []struct {
		Type   string
		Amount float64
		Cnt    int64
	}
	model.DB.Model(&model.RepairExpense{}).
		Where("created_at >= ?", from).
		Select("type, SUM(amount) AS amount, COUNT(*) AS cnt").
		Group("type").Scan(&rows)

	total := float64(0)
	byType := map[string]float64{}
	for _, r := range rows {
		byType[r.Type] = r.Amount
		total += r.Amount
	}
	reply := fmt.Sprintf("最近 %d 天维修费用合计 ¥%.2f", days, total)
	if len(byType) > 0 {
		reply += "（"
		for _, t := range []string{"material", "labor", "traffic", "other"} {
			if v, ok := byType[t]; ok {
				reply += fmt.Sprintf("%s ¥%.2f、", typeText(t), v)
			}
		}
		reply = strings.TrimRight(reply, "、") + "）。"
	}
	if len(byType) == 0 {
		reply += "，暂无费用记录。"
	}
	return NLAnswer{Reply: reply, Intent: "query", Tool: "expense_summary", Data: map[string]any{"days": days, "total": total, "by_type": byType}, Source: "规则"}
}

// runOpsHealth 运维健康评分 + 决策建议（L6 决策中心入口，只读）
func runOpsHealth(userID uint, it NLIntent) NLAnswer {
	health, err := BuildOpsHealth()
	if err != nil {
		return NLAnswer{Reply: "健康评分失败：" + err.Error(), Intent: "query", Tool: "ops_health", Source: "规则"}
	}
	reply := health.Summary
	// 附决策建议摘要（最多3条）
	decisions, _ := BuildDecisions()
	if len(decisions) > 0 {
		reply += " 决策建议："
		for i, d := range decisions {
			if i >= 3 {
				break
			}
			reply += fmt.Sprintf("%s（%s）、", d.Title, d.Priority)
		}
		reply = strings.TrimRight(reply, "、") + "。"
	} else {
		reply += " 暂无明显需干预的决策项。"
	}
	return NLAnswer{
		Reply: reply, Intent: "query", Tool: "ops_health",
		Data:   map[string]any{"health_total": health.Total, "level": health.Grade, "decision_count": len(decisions)},
		Source: "规则",
	}
}

// runAnomalyStream 实时异常流检测（L6，只读）
// 返回最近窗内异常事件摘要：报文告警/无效、活跃故障、超时工单、离线设备
func runAnomalyStream(userID uint, it NLIntent) NLAnswer {
	hours := parseDays(it.Params["hours"], 24)
	res, err := BuildAnomalyStream(hours, 20)
	if err != nil {
		return NLAnswer{Reply: "异常流检测失败：" + err.Error(), Intent: "query", Tool: "anomaly_stream", Source: "规则"}
	}
	reply := "最近" + fmt.Sprintf("%d", hours) + "小时发现" + fmt.Sprintf("%d", res.Total) + "个异常事件（" + res.Summary + "）："
	if len(res.Events) > 0 {
		for i, e := range res.Events {
			if i == 5 {
				reply += "…"
				break
			}
			reply += fmt.Sprintf("[%s]%s、", e.Level, e.Title)
		}
		reply = strings.TrimRight(reply, "、") + "。"
	}
	return NLAnswer{
		Reply: reply, Intent: "query", Tool: "anomaly_stream",
		Data:   map[string]any{"total": res.Total, "by_level": res.ByLevel, "events": res.Events},
		Source: "规则",
	}
}

// ---- 命令类工具（真实写入） ----

func runCreateFault(userID uint, it NLIntent, raw string) NLAnswer {
	desc := it.Params["desc"]
	hw := parseHwID(it.Params["hw_id"])
	if hw == 0 {
		hw = parseHwID(it.Params["device"]) // LLM 可能把设备ID放在 device 参数
	}
	if hw == 0 {
		hw = parseHwID(extractHwID(raw)) // 直接从原文兜底提取设备 ID
	}
	var device *model.Device
	if hw > 0 {
		var d model.Device
		if err := model.DB.Where("hw_id = ?", hw).First(&d).Error; err == nil {
			device = &d
		}
	}
	// 尝试从描述/设备参数解析路口
	if device == nil {
		term := it.Params["device"]
		if term == "" {
			term = extractIntersection(raw)
		}
		if term != "" {
			var d model.Device
			if err := model.DB.Where("intersection LIKE ?", "%"+term+"%").First(&d).Error; err == nil {
				device = &d
			}
		}
	}
	if device == nil && hw == 0 {
		return NLAnswer{Reply: "未找到对应设备，无法建故障单。请提供设备硬件 ID（如 123456）或路口名称，或改为「报修：{路口}{问题}」。", Intent: "command", Tool: "create_fault", Source: "规则"}
	}
	if device == nil {
		// 有 hw 但无设备
		return NLAnswer{Reply: fmt.Sprintf("设备 #%d 不存在，请检查硬件 ID。", hw), Intent: "command", Tool: "create_fault", Source: "规则"}
	}

	level := it.Params["level"]
	if level == "" {
		level = mapLevel(raw)
	}
	faultType := detectFaultType(raw)
	if desc == "" {
		desc = cleanDesc(raw)
	}
	now := time.Now()
	f := model.FaultRecord{
		DeviceHwID:  device.HwID,
		ErrCode:     detectErrCode(raw),
		FaultType:   faultType,
		FaultLevel:  level,
		FirstSeen:   now,
		LastSeen:    now,
		Status:      model.FaultStatusConfirmed,
		ConfirmedAt: &now,
	}
	if err := model.DB.Create(&f).Error; err != nil {
		return NLAnswer{Reply: "创建故障单失败：" + err.Error(), Intent: "command", Tool: "create_fault", Source: "规则"}
	}
	reply := fmt.Sprintf("已创建故障单 #%d（设备#%d，%s，等级%s）", f.ID, device.HwID, faultType, level)
	if desc != "" {
		reply += "。描述：" + desc
	}
	return NLAnswer{Reply: reply, Intent: "command", Tool: "create_fault", Data: map[string]any{"fault_id": f.ID, "device_hw_id": device.HwID, "fault_type": faultType, "level": level}, Source: "规则", DidWrite: true, CreatedID: f.ID}
}

func runCreateWorkOrder(userID uint, it NLIntent, raw string) NLAnswer {
	var device *model.Device
	hw := parseHwID(it.Params["hw_id"])
	if hw == 0 {
		hw = parseHwID(it.Params["device"]) // LLM 可能把设备ID放在 device 参数
	}
	if hw == 0 {
		hw = parseHwID(extractHwID(raw)) // 直接从原文兜底提取设备 ID
	}
	if hw > 0 {
		var d model.Device
		if err := model.DB.Where("hw_id = ?", hw).First(&d).Error; err == nil {
			device = &d
		}
	}
	if device == nil {
		term := it.Params["device"]
		if term == "" {
			term = extractIntersection(raw)
		}
		if term != "" {
			var d model.Device
			if err := model.DB.Where("intersection LIKE ?", "%"+term+"%").First(&d).Error; err == nil {
				device = &d
			}
		}
	}
	if device == nil {
		return NLAnswer{Reply: "未找到目标设备，无法建工单。请提供设备硬件 ID 或路口名称。", Intent: "command", Tool: "create_workorder", Source: "规则"}
	}

	// 复用现有工单编号生成与创建逻辑
	orderNo := model.NextOrderNo(model.DB)
	wo := model.WorkOrder{
		OrderNo:    orderNo,
		DeviceHwID: device.HwID,
		Status:     model.WorkOrderStatusPending,
	}
	note := it.Params["note"]
	if note == "" {
		note = cleanDesc(raw)
	}
	if note != "" {
		wo.Result = "待处理：" + note
	}
	if err := model.DB.Create(&wo).Error; err != nil {
		return NLAnswer{Reply: "创建工单失败：" + err.Error(), Intent: "command", Tool: "create_workorder", Source: "规则"}
	}
	reply := fmt.Sprintf("已创建工单 %s（设备#%d，%s，待处理）", wo.OrderNo, device.HwID, device.Intersection)
	return NLAnswer{Reply: reply, Intent: "command", Tool: "create_workorder", Data: map[string]any{"work_order_id": wo.ID, "order_no": wo.OrderNo, "device_hw_id": device.HwID, "intersection": device.Intersection}, Source: "规则", DidWrite: true, CreatedID: wo.ID}
}

// ---- 知识库/RAG 兜底 ----

func runKnowledge(raw string) NLAnswer {
	for _, k := range nlKnowledgeBase {
		for _, kw := range k.Keywords {
			if strings.Contains(raw, kw) {
				return NLAnswer{Reply: k.Answer, Intent: "fallback", Tool: "kb", Source: "规则"}
			}
		}
	}
	return NLAnswer{
		Reply:  "我无法自动执行这个请求。可尝试：\n• 查询：「最近7天哪些路口故障最多」「查询设备123456状态」「工单统计」「最近30天费用」\n• 命令：「报修：人民路口红不亮」「给设备123456建工单」\n• 咨询：操作流程、设备、采购、固件、报告等问题（如「怎么新建工单？」）",
		Intent: "fallback", Tool: "kb", Source: "规则",
	}
}

// ---- 参数解析助手 ----

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

func extractDays(text string) string {
	// 匹配 “N天/周/月” 或纯数字
	var n string
	if i := strings.Index(text, "天"); i > 0 {
		for j := i - 1; j >= 0 && text[j] >= '0' && text[j] <= '9'; j-- {
			n = string(text[j]) + n
		}
	}
	if n == "" {
		if i := strings.Index(text, "周"); i > 0 {
			for j := i - 1; j >= 0 && text[j] >= '0' && text[j] <= '9'; j-- {
				n = string(text[j]) + n
			}
			if n != "" {
				return fmt.Sprintf("%d", atoi(n)*7)
			}
		}
	}
	if n == "" {
		return "7"
	}
	return n
}

func parseDays(s string, def int) int {
	if s == "" {
		return def
	}
	v := atoi(s)
	if v <= 0 {
		return def
	}
	if v > 365 {
		v = 365
	}
	return v
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func extractHwID(text string) string {
	// 优先匹配“设备N”显式模式（设备1/设备123 等），长度不限
	for _, seg := range []string{"设备#", "设备"} {
		if i := strings.Index(text, seg); i >= 0 {
			rest := text[i+len(seg):]
			num := ""
			for _, ch := range rest {
				if ch < '0' || ch > '9' {
					break
				}
				num += string(ch)
			}
			if num != "" {
				return num
			}
		}
	}
	// 退而求其次：匹配 4-10 位纯数字（作为 hw_id）
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !(r >= '0' && r <= '9') }) {
		if len(w) >= 4 && len(w) <= 10 {
			return w
		}
	}
	return ""
}

func parseHwID(s string) uint32 {
	if s == "" {
		return 0
	}
	var v uint32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + uint32(c-'0')
	}
	return v
}

func extractIntersection(text string) string {
	// 简单提取“xxx路口/xxx交叉口”模式（rune 安全，支持中文全角分隔符）
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		// 匹配“…路口”或“…交叉口”尾部
		end := -1
		if runes[i] == '路' && i+1 < len(runes) && runes[i+1] == '口' {
			end = i + 2
		} else if runes[i] == '交' && i+2 < len(runes) && runes[i+1] == '叉' && runes[i+2] == '口' {
			end = i + 3
		}
		if end < 0 {
			continue
		}
		start := i
		for start > 0 {
			prev := runes[start-1]
			if prev == '：' || prev == ':' || prev == '，' || prev == ',' || prev == ' ' || prev == '的' {
				break
			}
			if end-start >= 14 {
				break
			}
			start--
		}
		if end > start {
			return strings.TrimSpace(string(runes[start:end]))
		}
	}
	return ""
}

func mapLevel(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "critical") || strings.Contains(text, "严重") || strings.Contains(text, "危急") || strings.Contains(text, "全灭") || strings.Contains(text, "不亮") {
		return "critical"
	}
	return "normal"
}

func cleanDesc(text string) string {
	s := strings.TrimSpace(text)
	for _, p := range []string{"报修：", "报修:", "故障：", "故障:", "上报故障：", "故障上报："} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
			break
		}
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func detectFaultType(text string) string {
	switch {
	case strings.Contains(text, "黄"):
		return "黄灯故障"
	case strings.Contains(text, "红"):
		return "红灯故障"
	case strings.Contains(text, "绿"):
		return "绿灯故障"
	case strings.Contains(text, "不亮"):
		return "灯组不亮"
	case strings.Contains(text, "闪"):
		return "灯组闪烁"
	case strings.Contains(text, "断"):
		return "线路故障"
	case strings.Contains(text, "供电"):
		return "供电故障"
	default:
		return "自然语言报修"
	}
}

func detectErrCode(text string) int8 {
	switch {
	case strings.Contains(text, "红"):
		return -1
	case strings.Contains(text, "黄"):
		return -2
	case strings.Contains(text, "绿"):
		return -3
	case strings.Contains(text, "供电"):
		return -5
	default:
		return -9
	}
}

func typeText(t string) string {
	switch t {
	case "material":
		return "耗材"
	case "labor":
		return "人工"
	case "traffic":
		return "交通"
	case "other":
		return "其他"
	default:
		return t
	}
}

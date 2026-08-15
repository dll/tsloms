package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// L6 实时异常流检测
// MQTT 数据流实时分析：聚合报文日志(告警/无效帧) + 活跃故障 + 未完成工单 + 离线设备
// 按时间倒序输出异常事件流，供 AI 工作台「实时异常流」页与 NL 助手「最近异常」查询
// 纯规则实现（确定性），不依赖 LLM；LLM 仅用于结论摘要增强
// ============================================================

// AnomalyEvent 一条异常事件
type AnomalyEvent struct {
	ID        uint    `json:"id"`
	Time      string  `json:"time"`       // 事件发生时间 (ISO)
	Kind      string  `json:"kind"`       // packet_alarm / packet_invalid / fault / workorder_overdue / device_offline
	Level     string  `json:"level"`      // critical / major / minor / info
	DeviceHw  uint32  `json:"device_hw_id"`
	Title     string  `json:"title"`
	Detail    string  `json:"detail"`
	BizType   string  `json:"biz_type"`   // 关联业务类型: fault / workorder / device / packet
	BizID     uint    `json:"biz_id"`
}

// AnomalyStreamResult 实时异常流检测结果
type AnomalyStreamResult struct {
	Events      []AnomalyEvent `json:"events"`       // 时间倒序
	Total       int            `json:"total"`
	ByLevel     map[string]int `json:"by_level"`     // 按等级统计
	Summary     string         `json:"summary"`      // 总体结论摘要（LLM 增强或规则）
	Source      string         `json:"source"`
	TokensUsed  int            `json:"tokens_used"`
	GeneratedAt string         `json:"generated_at"`
}

// BuildAnomalyStream 构建最近 N 小时的实时异常流
// windowHours 默认 24；maxEvents 默认 50
func BuildAnomalyStream(windowHours int, maxEvents int) (*AnomalyStreamResult, error) {
	if windowHours <= 0 {
		windowHours = 24
	}
	if maxEvents <= 0 {
		maxEvents = 50
	}
	if model.DB == nil {
		return &AnomalyStreamResult{
			Events: []AnomalyEvent{}, ByLevel: map[string]int{},
			Summary: "实时异常流不可用（数据库未连接）", Source: "规则", GeneratedAt: time.Now().Format(time.RFC3339),
		}, nil
	}

	since := time.Now().Add(-time.Duration(windowHours) * time.Hour)
	var events []AnomalyEvent

	// 1) 报文异常：最近窗内的告警命令(0x01)与无效/校验失败帧（SQL 层先过滤，避免漏报）
	var pktLogs []model.PacketLog
	if err := model.DB.Where("received_at >= ? AND (cmd_type = ? OR valid = ?)", since, uint8(0x01), false).
		Order("received_at DESC").Limit(maxEvents).Find(&pktLogs).Error; err != nil {
		return nil, fmt.Errorf("查询报文异常失败: %w", err)
	}
	for _, pl := range pktLogs {
		kind, title, detail := classifyPacket(pl)
		if kind == "" {
			continue
		}
		events = append(events, AnomalyEvent{
			ID: pl.ID, Time: pl.ReceivedAt.Format(time.RFC3339), Kind: kind,
			Level: pktLevel(kind), DeviceHw: pl.DeviceHwID, Title: title, Detail: detail,
			BizType: "packet", BizID: pl.ID,
		})
	}

	// 2) 活跃故障（未解决且在本时间窗内，避免查询范围外漏报/串扰）
	var faults []model.FaultRecord
	if err := model.DB.Where("status IN ? AND last_seen >= ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}, since).Order("last_seen DESC").Limit(maxEvents).Find(&faults).Error; err != nil {
		return nil, fmt.Errorf("查询活跃故障失败: %w", err)
	}
	for _, f := range faults {
		lvl := "major"
		if f.FaultLevel == "critical" {
			lvl = "critical"
		}
		events = append(events, AnomalyEvent{
			ID: f.ID, Time: f.LastSeen.Format(time.RFC3339), Kind: "fault", Level: lvl,
			DeviceHw: f.DeviceHwID, Title: fmt.Sprintf("故障：%s", f.FaultType),
			Detail: fmt.Sprintf("错误码 %d，最后上报 %s（去重窗口内持续存在）", f.ErrCode, f.LastSeen.Format("01-02 15:04")),
			BizType: "fault", BizID: f.ID,
		})
	}

	// 3) 超时工单（复用模型层 SLA 判定：pending 超 24h / processing 超 48h）
	var wos []model.WorkOrder
	if err := model.DB.Where("status IN ?", []string{model.WorkOrderStatusPending, model.WorkOrderStatusProcessing}).Find(&wos).Error; err != nil {
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}
	var overdueDetail []string
	for _, wo := range wos {
		if hours := model.WorkOrderOverdueHours(&wo); hours > 0 {
			overdueDetail = append(overdueDetail, fmt.Sprintf("%s 超时 %.1f 小时", wo.OrderNo, hours))
		}
	}
	if len(overdueDetail) > 0 {
		events = append(events, AnomalyEvent{
			Time: time.Now().Format(time.RFC3339), Kind: "workorder_overdue", Level: "critical",
			Title:  fmt.Sprintf("%d 张工单超时未闭环", len(overdueDetail)),
			Detail: "超时工单：" + strings.Join(overdueDetail[:min(3, len(overdueDetail))], "、") + "，建议优先跟进处置",
			BizType: "workorder",
		})
	}

	// 4) 离线设备
	var offline int64
	if err := model.DB.Model(&model.Device{}).Where("online_status = ?", false).Count(&offline).Error; err != nil {
		return nil, fmt.Errorf("查询离线设备失败: %w", err)
	}
	if offline > 0 {
		events = append(events, AnomalyEvent{
			Time: time.Now().Format(time.RFC3339), Kind: "device_offline", Level: "major",
			Title: fmt.Sprintf("%d 台设备离线", offline),
			Detail: "存在离线设备，可能漏报故障，建议核查供电/网络/通信", BizType: "device",
		})
	}

	// 时间倒序排序
	sortEventsDesc(events)

	// 截断（先截断再统计，保证 by_level 合计与 total 一致）
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}

	byLevel := map[string]int{}
	for _, e := range events {
		byLevel[e.Level]++
	}

	res := &AnomalyStreamResult{
		Events: events, Total: len(events), ByLevel: byLevel,
		Source: "规则", TokensUsed: 0, GeneratedAt: time.Now().Format(time.RFC3339),
	}
	res.Summary = ruleAnomalySummary(res)
	return res, nil
}

// classifyPacket 对报文字段分类：返回 kind/title/detail；非异常返回空 kind
// 命令类型与 mqtt 包协议一致：0x00=签到 / 0x01=告警 / 0x03=上电（见 internal/mqtt/commands.go）
func classifyPacket(pl model.PacketLog) (kind, title, detail string) {
	const (
		cmdCheckin = 0x00
		cmdAlarm   = 0x01
	)
	switch pl.CmdType {
	case cmdAlarm: // CMD_ALARM 告警命令 0x01
		kind = "packet_alarm"
		title = "设备告警报文"
		detail = "收到设备告警上报，触发故障研判"
	case cmdCheckin: // CMD_CHECKIN 签到 0x00
		// 签到本身不异常，跳过
		return "", "", ""
	default:
		if !pl.Valid {
			kind = "packet_invalid"
			title = "无效报文"
			detail = "校验未通过的报文帧，可能为干扰或通信异常"
		}
	}
	return
}

func pktLevel(kind string) string {
	switch kind {
	case "packet_invalid":
		return "major"
	case "packet_alarm":
		return "critical"
	default:
		return "minor"
	}
}

// sortEventsDesc 按时间倒序排序（稳定）
func sortEventsDesc(events []AnomalyEvent) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j-1].Time < events[j].Time; j-- {
			events[j-1], events[j] = events[j], events[j-1]
		}
	}
}

// ruleAnomalySummary 规则结论摘要
func ruleAnomalySummary(res *AnomalyStreamResult) string {
	var parts []string
	if res.Total == 0 {
		return "最近无异常事件，系统运行平稳。"
	}
	if res.ByLevel["critical"] > 0 {
		parts = append(parts, fmt.Sprintf("存在 %d 项严重异常", res.ByLevel["critical"]))
	}
	if res.ByLevel["major"] > 0 {
		parts = append(parts, fmt.Sprintf("%d 项重要异常", res.ByLevel["major"]))
	}
	if res.ByLevel["minor"] > 0 {
		parts = append(parts, fmt.Sprintf("%d 项次要异常", res.ByLevel["minor"]))
	}
	return "实时异常流：共 " + fmt.Sprintf("%d", res.Total) + " 个事件（" + strings.Join(parts, "、") + "），建议优先处置严重与重要级别异常。"
}

package ai

import (
	"fmt"

	"github.com/tsloms/server/internal/model"
)

// LifecycleEvent 生命周期事件（时间线节点）
type LifecycleEvent struct {
	Time  string `json:"time"`
	Type  string `json:"type"` // install/fault/workorder/repair/scrap/offline
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

// LifecycleResult 设备全流程溯源结果
type LifecycleResult struct {
	DeviceHwID   uint32           `json:"device_hw_id"`
	Intersection string           `json:"intersection"`
	Timeline     []LifecycleEvent `json:"timeline"`
	Summary      string           `json:"summary"` // LLM生成的生命周期画像
	Source       string           `json:"source"`
	TokensUsed   int              `json:"tokens_used"`
}

// BuildLifecycle 聚合单台设备全流程数据 → 时间线 + LLM 溯源画像
func BuildLifecycle(userID uint, dev *model.Device) LifecycleResult {
	res := LifecycleResult{DeviceHwID: dev.HwID, Intersection: dev.Intersection}

	// 1) 安装
	if dev.InstalledAt != nil {
		res.Timeline = append(res.Timeline, LifecycleEvent{
			Time: dev.InstalledAt.Format("2006-01-02"), Type: "install",
			Title: "设备安装", Desc: fmt.Sprintf("固件版本 %d.%d", swMajor(dev.SwVersion), swMinor(dev.SwVersion)),
		})
	}

	// 2) 故障记录
	var faults []model.FaultRecord
	model.DB.Where("device_hw_id = ?", dev.HwID).Order("created_at ASC").Find(&faults)
	faultCount := len(faults)
	for _, f := range faults {
		res.Timeline = append(res.Timeline, LifecycleEvent{
			Time: f.CreatedAt.Format("2006-01-02 15:04"), Type: "fault",
			Title: fmt.Sprintf("故障码%d（%s）", f.ErrCode, f.FaultType),
			Desc:  fmt.Sprintf("等级%s，状态%s", f.FaultLevel, f.Status),
		})
	}

	// 3) 工单（维修）
	var orders []model.WorkOrder
	model.DB.Where("device_hw_id = ?", dev.HwID).Order("created_at ASC").Find(&orders)
	for _, o := range orders {
		res.Timeline = append(res.Timeline, LifecycleEvent{
			Time: o.CreatedAt.Format("2006-01-02 15:04"), Type: "workorder",
			Title: fmt.Sprintf("工单 %s", o.OrderNo),
			Desc:  fmt.Sprintf("状态%s，结果：%s", o.Status, o.Result),
		})
	}

	// 4) 离线事件（按报文间隙推断）
	// （简化：已在规则引擎预测中体现离线频率，此处时间线以故障/工单为主）

	// LLM 溯源画像
	ctx := fmt.Sprintf(
		"【交通信号灯全流程生命周期溯源】设备ID:%d 路口:%s 在线:%v\n"+
			"安装至今累计故障 %d 次，工单 %d 张，当前故障状态活跃。\n"+
			"请用中文输出一段 ≤180字的生命周期画像与溯源分析，包括：总体健康状况、高频故障项、维修闭环情况、老化风险、后续保养建议。\n",
		dev.HwID, dev.Intersection, dev.OnlineStatus, faultCount, len(orders),
	)

	cfg := model.GetAIConfig()
	client := NewLLMClient(cfg)
	if text, tokens, err := client.Ask(userID, "lifecycle", ctx); err == nil {
		res.Summary = text
		res.Source = "LLM"
		res.TokensUsed = tokens
	} else {
		res.Summary = fmt.Sprintf("规则画像：该设备累计故障 %d 次、维修工单 %d 张，"+
			"当前%s。建议按历史高频故障类型制定保养计划。", faultCount, len(orders),
			map[bool]string{true: "在线", false: "离线"}[dev.OnlineStatus])
		res.Source = "规则降级"
	}

	return res
}

// ParseTimelineEvents 把时间线转成前端友好结构
func (r LifecycleResult) TypeCount() map[string]int {
	m := map[string]int{}
	for _, e := range r.Timeline {
		m[e.Type]++
	}
	return m
}

func swMajor(v uint32) uint32 { return (v >> 28) & 0xF }
func swMinor(v uint32) uint32 { return (v >> 24) & 0xF }

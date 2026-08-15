package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// L4 流程 Copilot 扩展（嵌入式，不打断人工操作）：
//   - 设备新建/编辑 Copilot：参数解释 + 配置建议（辅助填写）
//   - 建单 Copilot：基于故障推荐 优先级/备件/处理步骤/建议维修人
//   - 采购 Copilot：合理性校验 + 供应商建议
//   统一：LLM 生成，规则兜底，返回结构化 {tips/...} 供前端 AiCopilot 展示。
// ============================================================

// DeviceCopilotAdvice 设备新建/编辑 AI 辅助
type DeviceCopilotAdvice struct {
	Summary string   `json:"summary"` // 设备情况小结
	Hints   []string `json:"hints"`   // 填写/配置建议（逐条）
	Issues  []string `json:"issues"`  // 潜在问题/校验提醒
	Source  string   `json:"source"`
}

// WorkOrderCreateAdvice 建单 Copilot（基于关联故障）
type WorkOrderCreateAdvice struct {
	FaultID      uint     `json:"fault_id"`
	DeviceHwID   uint32   `json:"device_hw_id"`
	Priority     string   `json:"priority"` // P0/P1/P2
	PriorityText string   `json:"priority_text"`
	Parts        []string `json:"parts"`         // 建议预领备件
	Steps        []string `json:"steps"`         // 处理步骤建议
	RepairerHint string   `json:"repairer_hint"` // 建议维修人员提示
	Summary      string   `json:"summary"`
	Source       string   `json:"source"`
}

// PurchaseCopilotAdvice 采购 Copilot
type PurchaseCopilotAdvice struct {
	Summary      string   `json:"summary"`       // 采购合理性小结
	Checks       []string `json:"checks"`        // 校验项（数量/价格/金额）
	Suggestions  []string `json:"suggestions"`   // 改进建议
	SupplierHint string   `json:"supplier_hint"` // 供应商建议
	Source       string   `json:"source"`
}

// SuggestDeviceCopilot 设备 Copilot：依据提交的设备字段给出填写与配置建议
func SuggestDeviceCopilot(userID uint, input map[string]any) (DeviceCopilotAdvice, error) {
	hw := uint32(0)
	if v, ok := fieldUint32(input, "hw_id"); ok {
		hw = v
	}
	intersection := fieldStr(input, "intersection")
	swVer, _ := fieldUint32(input, "sw_version")
	confVer, _ := fieldUint32(input, "conf_version")
	lat, _ := fieldFloat(input, "lat")
	lng, _ := fieldFloat(input, "lng")

	// 规则兜底：基础校验
	adv := DeviceCopilotAdvice{Source: "规则"}
	if hw == 0 {
		adv.Summary = "设备硬件 ID（hw_id）为空，请填写出厂唯一硬件编号后保存。"
		adv.Issues = append(adv.Issues, "hw_id 不能为空（唯一）。")
	} else {
		adv.Summary = fmt.Sprintf("设备 #%d%s%s", hw, ifStr(intersection != "", "（"+intersection+"）", ""), ifStr(swVer > 0, "，固件版本位域 "+fmt.Sprint(swVer), ""))
		if intersection == "" {
			adv.Issues = append(adv.Issues, "建议填写路口位置描述，便于地图定位与按路口检索。")
		}
	}
	if swVer == 0 && confVer == 0 {
		adv.Hints = append(adv.Hints, "固件/配置版本可在设备上线签到后自动回填，新建阶段可暂不填。")
	}
	if lat == 0 && lng == 0 {
		adv.Hints = append(adv.Hints, "经纬度可留空，稍后在路口定位中设置。")
	}
	// 网络号/站点号提示
	if nc := fieldInt(input, "network_code"); nc > 0 || fieldInt(input, "station_code") > 0 {
		adv.Hints = append(adv.Hints, "已填网络号/站点号，请确保与现场组网一致，避免协议解析错位。")
	}
	if len(adv.Hints) == 0 {
		adv.Hints = append(adv.Hints, "设备信息完整，可直接保存。")
	}

	// LLM 增强（需数据库取配置；无数据库时直接走规则库）
	if model.DB != nil {
		client := NewLLMClient(nil)
		var payload []byte
		payload, _ = json.Marshal(input)
		prompt := "你是交通信号灯设备运维专家。基于以下设备台账录入信息，用中文给出 3-5 条简洁的填写/配置建议与潜在问题提醒。设备字段：" + string(payload)
		if text, tk, err := client.Ask(userID, "advice_device", prompt); err == nil {
			adv.Source = "LLM"
			adv.Summary = firstLine(text)
			for _, line := range splitLinesFull(text) {
				line = trimSpace(line)
				if line == "" {
					continue
				}
				item := strings.TrimLeft(line, "-•1234567890.、 ")
				if item != line {
					adv.Hints = append(adv.Hints, item)
				}
			}
			if len(adv.Hints) == 0 {
				adv.Hints = append(adv.Hints, text)
			}
			_ = tk
		}
	}
	return adv, nil
}

// SuggestWorkOrderCreate 建单 Copilot：基于关联故障推荐建单要素
func SuggestWorkOrderCreate(userID uint, faultID uint) (WorkOrderCreateAdvice, error) {
	var f model.FaultRecord
	if err := model.DB.First(&f, faultID).Error; err != nil {
		return WorkOrderCreateAdvice{}, err
	}
	parts := recentPartsForDevice(f.DeviceHwID)
	badHints := strings.ToLower(strings.Join(parts, " "))

	adv := WorkOrderCreateAdvice{FaultID: f.ID, DeviceHwID: f.DeviceHwID}
	adv.Priority = mapPriorityFault(f.FaultLevel)
	adv.PriorityText = priorityText(adv.Priority)
	adv.Parts = parts
	adv.Steps = []string{fmt.Sprintf("按故障码 %d（%s）对应预案排查对应灯组", f.ErrCode, f.FaultType)}
	if len(badHints) > 0 {
		adv.Steps = append(adv.Steps, "携带常用备件现场处置，必要时更换后复测")
	}
	adv.Steps = append(adv.Steps, "完成后在工单内填写维修结果并闭环")
	adv.Summary = fmt.Sprintf("设备 #%d 故障码 %d（%s），等级 %s，建议按 %s 建单处理。",
		f.DeviceHwID, f.ErrCode, f.FaultType, f.FaultLevel, adv.Priority)
	if adv.Priority == "P0" {
		adv.RepairerHint = "紧急故障，建议指派空闲运维立即处理。"
	} else {
		adv.RepairerHint = "建议指派擅长该类灯组的运维。"
	}
	adv.Source = "规则"

	if model.DB != nil {
		client := NewLLMClient(nil)
		prompt := fmt.Sprintf(
			"你是交通信号灯运维主管。基于故障：设备#%d，故障码%d（%s），等级%s。请输出建单建议：\n"+
				"优先级：P0/P1/P2\n处理步骤：（请以『处理步骤：』开头，每行一条，用数字序号）\n预领备件：（用顿号分隔）\n推荐维修人员：...",
			f.DeviceHwID, f.ErrCode, f.FaultType, f.FaultLevel)
		if text, tk, err := client.Ask(userID, "advice_wo_create", prompt); err == nil {
			adv.Source = "LLM"
			if ps := extractAdvList(text, "处理步骤"); len(ps) > 0 {
				adv.Steps = ps
			}
			if pv := extractAdv(text, "优先级"); pv != "" {
				if strings.Contains(pv, "P0") {
					adv.Priority = "P0"
					adv.PriorityText = priorityText("P0")
				} else if strings.Contains(pv, "P1") {
					adv.Priority = "P1"
					adv.PriorityText = priorityText("P1")
				}
			}
			if ps := extractAdvList(text, "预领备件"); len(ps) > 0 {
				adv.Parts = ps
			}
			if rh := extractAdv(text, "推荐维修人员"); rh != "" {
				adv.RepairerHint = rh
			}
			_ = tk
		}
	}
	return adv, nil
}

// SuggestPurchaseCopilot 采购 Copilot：合理性校验 + 供应商建议
func SuggestPurchaseCopilot(userID uint, items []PurchaseLine, supplierID uint) PurchaseCopilotAdvice {
	adv := PurchaseCopilotAdvice{Source: "规则"}
	total := 0.0
	for _, it := range items {
		amt := float64(it.Quantity) * it.Price
		total += amt
		if it.Quantity <= 0 {
			adv.Checks = append(adv.Checks, fmt.Sprintf("「%s」数量须 > 0。", it.MaterialName))
		}
		if it.Price < 0 {
			adv.Checks = append(adv.Checks, fmt.Sprintf("「%s」单价不能为负。", it.MaterialName))
		}
	}
	if total <= 0 && len(items) > 0 {
		adv.Checks = append(adv.Checks, "采购总额为 0，请确认数量与单价。")
	}
	adv.Summary = fmt.Sprintf("本次采购共 %d 项，合计 ¥%.2f。", len(items), total)
	if len(adv.Checks) == 0 {
		adv.Suggestions = append(adv.Suggestions, "明细校验通过，可正常提交；建议核对库存后再确认入库。")
	} else {
		adv.Suggestions = append(adv.Suggestions, "存在校验告警，请逐项核对后提交。")
	}
	if supplierID > 0 {
		var s model.Supplier
		if err := model.DB.First(&s, supplierID).Error; err == nil {
			adv.SupplierHint = "当前供应商：" + s.Name + "。"
		}
	} else {
		adv.SupplierHint = "尚未选择供应商，请从供应商列表选择（建议选用 active 状态的供应商）。"
	}

	if model.DB != nil {
		client := NewLLMClient(nil)
		payload, _ := json.Marshal(items)
		prompt := "你是物资采购审核员。检查以下采购明细（名称/数量/单价）的合理性，用中文给出校验结论、数量/价格改进建议、供应商选择建议：\n" + string(payload)
		if text, tk, err := client.Ask(userID, "advice_purchase", prompt); err == nil {
			adv.Source = "LLM"
			for _, line := range splitLinesFull(text) {
				line = trimSpace(line)
				if line == "" {
					continue
				}
				item := strings.TrimLeft(line, "-•1234567890.、 ")
				if item != "" && item != line {
					adv.Suggestions = append(adv.Suggestions, item)
				}
			}
			_ = tk
		}
	}
	return adv
}

// PurchaseLine 采购明细（供 Copilot 输入使用）
type PurchaseLine struct {
	MaterialName string  `json:"material_name"`
	Quantity     int     `json:"quantity"`
	Price        float64 `json:"price"`
}

// ---- 字段取值助手 ----

func fieldStr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func fieldInt(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func fieldUint32(m map[string]any, k string) (uint32, bool) {
	switch v := m[k].(type) {
	case float64:
		return uint32(v), true
	case int:
		return uint32(v), true
	case string:
		var n uint64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return uint32(n), true
		}
	}
	return 0, false
}

func fieldFloat(m map[string]any, k string) (float64, bool) {
	v, ok := m[k].(float64)
	return v, ok
}

func ifStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func firstLine(s string) string {
	for _, line := range splitLinesFull(s) {
		if trimSpace(line) != "" {
			return trimSpace(line)
		}
	}
	return s
}

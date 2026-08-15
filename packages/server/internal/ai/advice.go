package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================
// 核心运维流程 AI 增强（嵌入式，不打断人工操作）：
//   - 故障诊断建议（确认/派单时）：摘要+优先级+预案+建议备件
//   - 工单 Copilot（处理中）：根因预判+处理步骤+备件预领+维修小结
// ============================================================

// FaultAdvice 故障级 AI 建议
type FaultAdvice struct {
	FaultID      uint     `json:"fault_id"`
	DeviceHwID   uint32   `json:"device_hw_id"`
	Summary      string   `json:"summary"`  // 故障摘要
	Priority     string   `json:"priority"` // P0/P1/P2
	PriorityText string   `json:"priority_text"`
	Plan         string   `json:"plan"`    // 应对预案/步骤
	Parts        []string `json:"parts"`   // 建议备件
	Content      string   `json:"content"` // 完整文本
	Source       string   `json:"source"`
	TokensUsed   int      `json:"tokens_used"`
}

// WorkOrderAdvice 工单级 AI 建议（Copilot）
type WorkOrderAdvice struct {
	WorkOrderID uint     `json:"work_order_id"`
	DeviceHwID  uint32   `json:"device_hw_id"`
	RootCause   string   `json:"root_cause"` // 根因预判
	Steps       []string `json:"steps"`      // 处理步骤
	Parts       []string `json:"parts"`      // 备件预领
	Summary     string   `json:"summary"`    // 维修小结（完成阶段）
	Content     string   `json:"content"`
	Source      string   `json:"source"`
	TokensUsed  int      `json:"tokens_used"`
}

// SuggestFaultAdvice 对单条故障生成 AI 建议（确认/派单辅助）
// 依据故障记录 + 设备历史 + 近期耗材使用，LLM 生成，规则兜底。
func SuggestFaultAdvice(userID uint, faultID uint) (FaultAdvice, error) {
	var f model.FaultRecord
	if err := model.DB.First(&f, faultID).Error; err != nil {
		return FaultAdvice{}, err
	}
	dev := deviceBrief(f.DeviceHwID)

	// 该设备历史工单/耗材（用于备件推荐）
	parts := recentPartsForDevice(f.DeviceHwID)

	ctx := buildFaultCtx(&f, dev, parts)

	client := NewLLMClient(nil)
	adv := FaultAdvice{FaultID: f.ID, DeviceHwID: f.DeviceHwID}
	adv.Parts = parts
	adv.Priority = mapPriorityFault(f.FaultLevel)
	adv.PriorityText = priorityText(adv.Priority)
	adv.Plan = buildFaultRulePlan(&f)

	prompt := fmt.Sprintf(
		"你是交通信号灯运维专家。基于以下故障信息，用中文生成故障处置建议，输出格式：\n"+
			"故障摘要：...\n优先级：P0/P1/P2\n应对预案：...\n建议备件：...\n\n故障信息：\n%s", ctx)
	if text, tk, err := client.Ask(userID, "advice_fault", prompt); err == nil {
		adv.Content = text
		adv.Source = "LLM"
		adv.TokensUsed = tk
		adv.Summary = extractAdv(text, "故障摘要")
		adv.Plan = extractAdv(text, "应对预案")
		if ps := extractAdvList(text, "建议备件"); len(ps) > 0 {
			adv.Parts = ps
		}
	} else {
		adv.Content = adv.Summary + "\n" + adv.Plan + "\n建议备件：" + joinStr(parts)
		adv.Source = "规则"
	}

	// 持久化建议
	persistAdvice(userID, "fault", f.ID, f.DeviceHwID, "confirm", adv.Priority, adv.Content, adv.Source, adv.TokensUsed)
	return adv, nil
}

// SuggestWorkOrderAdvice 工单 Copilot：根因预判 + 处理步骤 + 备件预领 + 维修小结
// stage: copilot(处理协助) / summary(完成小结)
func SuggestWorkOrderAdvice(userID uint, woID uint, stage string) (WorkOrderAdvice, error) {
	var wo model.WorkOrder
	if err := model.DB.First(&wo, woID).Error; err != nil {
		return WorkOrderAdvice{}, err
	}
	if stage == "" {
		stage = "copilot"
	}
	dev := deviceBrief(wo.DeviceHwID)
	parts := recentPartsForDevice(wo.DeviceHwID)

	// 关联故障与历史领料
	var faultDesc string
	var f model.FaultRecord
	if wo.FaultID > 0 {
		if err := model.DB.First(&f, wo.FaultID).Error; err == nil {
			faultDesc = fmt.Sprintf("故障码%d(%s) 等级%s", f.ErrCode, f.FaultType, f.FaultLevel)
		}
	}
	var usedParts []struct {
		MaterialName string
		Qty          int
	}
	model.DB.Model(&model.MaterialStock{}).Where("work_order_id = ? AND type = ?", wo.ID, model.StockTypeUse).
		Select("material_name, SUM(ABS(quantity)) AS qty").Group("material_name").Scan(&usedParts)

	adv := WorkOrderAdvice{WorkOrderID: wo.ID, DeviceHwID: wo.DeviceHwID, Parts: parts}
	client := NewLLMClient(nil)

	var prompt, ruleText string
	if stage == "summary" {
		// 维修小结
		ruleText = buildSummaryRule(&wo, &f, usedParts)
		prompt = fmt.Sprintf(
			"你是运维主管。基于以下工单信息，用中文生成一段 ≤150字维修小结（结果说明、处理内容、耗材用量、遗留问题）。\n"+
				"工单#%s 设备#%d 关联故障：%s\n已领耗材：%s\n维修结果：%s",
			wo.OrderNo, wo.DeviceHwID, faultDesc, joinUsed(usedParts), wo.Result)
		adv.Summary = ruleText
		adv.Steps = []string{}
	} else {
		ruleText = buildCopilotRule(&wo, &f, parts)
		prompt = fmt.Sprintf(
			"你是交通信号灯运维专家。工单#%s 设备#%d（%s）关联故障：%s，当前%s。"+
				"请用中文输出：\n根因预判：...\n处理步骤：（逐条）\n建议备件：...\n\n设备历史耗材：%s",
			wo.OrderNo, wo.DeviceHwID, dev, faultDesc, wo.Status, joinStr(parts))
		adv.Summary = ruleText
	}

	if text, tk, err := client.Ask(userID, "advice_workorder_"+stage, prompt); err == nil {
		adv.Content = text
		adv.Source = "LLM"
		adv.TokensUsed = tk
		if stage == "summary" {
			adv.Summary = text
		} else {
			adv.RootCause = extractAdv(text, "根因预判")
			adv.Steps = extractAdvList(text, "处理步骤")
			if ps := extractAdvList(text, "建议备件"); len(ps) > 0 {
				adv.Parts = ps
			}
		}
	} else {
		adv.Content = ruleText
		adv.Source = "规则"
		if stage == "summary" {
			adv.Summary = ruleText
		} else {
			adv.Steps = []string{ruleText}
		}
	}

	persistAdvice(userID, "workorder", wo.ID, wo.DeviceHwID, stage, mapStagePriority(stage), adv.Content, adv.Source, adv.TokensUsed)
	return adv, nil
}

// ---- 规则兜底 ----

func buildFaultRulePlan(f *model.FaultRecord) string {
	return fmt.Sprintf("按故障码%d（%s）对应的维修预案处理；优先检查对应灯组供电与灯珠状态，"+
		"必要时上门检修并上传现场图片。", f.ErrCode, f.FaultType)
}

func buildCopilotRule(wo *model.WorkOrder, f *model.FaultRecord, parts []string) string {
	line := fmt.Sprintf("工单#%s 设备#%d 待处理。建议：核实故障复现情况，按预案排查；", wo.OrderNo, wo.DeviceHwID)
	if f != nil && f.ErrCode != 0 {
		line += fmt.Sprintf("重点检查故障码%d（%s）对应灯组；", f.ErrCode, f.FaultType)
	}
	if len(parts) > 0 {
		line += "建议预领备件：" + joinStr(parts) + "。"
	} else {
		line += "先到现场判断后再领料。"
	}
	return line
}

func buildSummaryRule(wo *model.WorkOrder, f *model.FaultRecord, used []struct {
	MaterialName string
	Qty          int
}) string {
	line := fmt.Sprintf("工单#%s 已完成闭环", wo.OrderNo)
	if f != nil && f.ErrCode != 0 {
		line += fmt.Sprintf("（故障码%d %s）", f.ErrCode, f.FaultType)
	}
	line += "。"
	if len(used) > 0 {
		line += "本次领用耗材：" + joinUsed(used) + "。"
	}
	if wo.Result != "" {
		line += "维修结果：" + wo.Result + "。"
	} else {
		line += "请补充维修结果说明。"
	}
	return line
}

// ---- 数据助手 ----

func deviceBrief(hw uint32) string {
	var d model.Device
	if err := model.DB.Where("hw_id = ?", hw).First(&d).Error; err == nil && d.Intersection != "" {
		return d.Intersection
	}
	return ""
}

// recentPartsForDevice 该设备历史领用过的物料（去重，供备件推荐）
func recentPartsForDevice(hw uint32) []string {
	var rows []struct {
		MaterialName string
		N            int64
	}
	model.DB.Model(&model.MaterialStock{}).
		Joins("JOIN work_orders AS w ON w.id = material_stocks.work_order_id").
		Where("w.device_hw_id = ? AND material_stocks.type = ?", hw, model.StockTypeUse).
		Select("material_name, COUNT(*) AS n").Group("material_name").
		Order("n DESC").Limit(5).Scan(&rows)
	out := []string{}
	for _, r := range rows {
		out = append(out, r.MaterialName)
	}
	return out
}

func buildFaultCtx(f *model.FaultRecord, dev string, parts []string) string {
	return fmt.Sprintf(
		"设备#%d（%s）故障码%d（%s），等级%s，状态%s，首次出现%s，电流 R=%d Y=%d G=%d。\n历史耗材：%s",
		f.DeviceHwID, dev, f.ErrCode, f.FaultType, f.FaultLevel, f.Status,
		f.FirstSeen.Format("01-02 15:04"), f.CurrentR, f.CurrentY, f.CurrentG, joinStr(parts))
}

func mapPriorityFault(level string) string {
	if level == "critical" {
		return "P0"
	}
	return "P1"
}

func mapStagePriority(stage string) string {
	switch stage {
	case "summary":
		return "P3"
	case "copilot":
		return "P1"
	default:
		return "P2"
	}
}

func priorityText(p string) string {
	switch p {
	case "P0":
		return "紧急（24小时内处理）"
	case "P1":
		return "优先（3天内处理）"
	case "P2":
		return "计划（1周内处理）"
	default:
		return "常规"
	}
}

func persistAdvice(userID uint, bizType string, bizID uint, hw uint32, stage, priority, content, source string, tokens int) {
	op := ""
	if userID > 0 {
		var u model.User
		if err := model.DB.Select("username").First(&u, userID).Error; err == nil {
			op = u.Username
		}
	}
	model.DB.Create(&model.AIAdvice{
		BizType: bizType, BizID: bizID, DeviceHwID: hw, Stage: stage,
		Priority: priority, Summary: "", Content: content, Source: source,
		TokensUsed: tokens, Operator: op,
	})
}

// ListAdvices 查询流程建议历史
func ListAdvices(bizType string, bizID uint, limit int) []model.AIAdvice {
	q := model.DB.Model(&model.AIAdvice{}).Order("created_at DESC")
	if bizType != "" {
		q = q.Where("biz_type = ?", bizType)
		if bizID > 0 {
			q = q.Where("biz_id = ?", bizID)
		}
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var list []model.AIAdvice
	q.Limit(limit).Find(&list)
	return list
}

// ---- 文本提取 ----

func extractAdv(text, key string) string {
	for _, line := range splitLines(text) {
		trim := trimSpace(line)
		if strings.HasPrefix(trim, key) {
			rest := strings.TrimLeft(trim[len(key):], " ：:　\t")
			if rest != "" {
				return rest
			}
		}
	}
	return ""
}

func extractAdvList(text, key string) []string {
	out := []string{}
	in := false
	for _, line := range splitLinesFull(text) {
		trim := trimSpace(line)
		if strings.HasPrefix(trim, key) {
			in = true
			rest := strings.TrimLeft(trim[len(key):], " ：:　\t")
			if rest != "" {
				out = append(out, rest)
			}
			continue
		}
		if in {
			if trim == "" || strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "•") ||
				strings.HasPrefix(trim, "　") || (len(trim) > 0 && trim[0] == '1') {
				item := strings.TrimLeft(trim, "-•1234567890.、　 ")
				if item != "" {
					out = append(out, item)
				}
				continue
			}
			break
		}
	}
	return out
}

func splitLinesFull(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		if ch == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func joinStr(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "、"
		}
		out += s
	}
	return out
}

func joinUsed(used []struct {
	MaterialName string
	Qty          int
}) string {
	out := ""
	for i, u := range used {
		if i > 0 {
			out += "、"
		}
		out += fmt.Sprintf("%s x%d", u.MaterialName, u.Qty)
	}
	if out == "" {
		out = "无"
	}
	return out
}

var _ = time.Now
var _ = json.Marshal

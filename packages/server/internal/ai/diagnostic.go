package ai

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/tsloms/server/internal/model"
)

// DiagnosticResult AI 故障诊断结果
type DiagnosticResult struct {
	Summary   string   `json:"summary"`   // 诊断结论
	Cause     string   `json:"cause"`     // 成因
	Solution  string   `json:"solution"`  // 解决方案
	Confidence float64 `json:"confidence"` // 置信度 0-1
	Source    string   `json:"source"`    // LLM / 规则降级
	TokensUsed int     `json:"tokens_used"`
}

// DiagnoseFeedback 基于问题反馈（文字+图片）进行 AI 故障诊断
// mediaDir 为本地媒体根目录（用于把 URL 解析为本地文件并转 base64 给多模态）
func DiagnoseFeedback(userID uint, fb *model.Feedback, mediaDir string, images []string) DiagnosticResult {
	// 组装设备/路口上下文
	ctx := fmt.Sprintf(
		"【问题反馈】标题：%s\n内容：%s\n路口：%s\n关联设备ID：%s\n",
		fb.Title, fb.Content, fb.Intersection, strOr(fb.DeviceHwID, "-"),
	)
	// 补充该设备最近故障
	var faults []model.FaultRecord
	if fb.DeviceHwID != nil {
		model.DB.Where("device_hw_id = ?", *fb.DeviceHwID).
			Order("created_at DESC").Limit(5).Find(&faults)
	}
	if len(faults) > 0 {
		ctx += "【该设备最近故障】\n"
		for _, f := range faults {
			ctx += fmt.Sprintf("- 故障码%d(%s) 时间%s\n", f.ErrCode, f.FaultType, f.CreatedAt.Format("01-02 15:04"))
		}
	}

	// 尝试 LLM 多模态诊断（有图片）或文本诊断
	cfg := model.GetAIConfig()
	client := NewLLMClient(cfg)

	// 处理本地图片 → base64 data URL
	dataURLs := imagePathsToDataURLs(images)

	prompt := "你是交通信号灯运维专家。请根据以下问题反馈" +
		"（结合设备最近故障记录）给出故障诊断。请用中文简洁输出，格式：\n" +
		"诊断结论：...\n成因分析：...\n解决方案：...\n建议备件：...\n\n" + ctx

	if len(dataURLs) > 0 {
		if text, tokens, err := client.AskVision(userID, "diagnose", prompt, dataURLs); err == nil {
			return DiagnosticResult{Summary: extractDiag(text, "诊断结论"), Cause: extractDiag(text, "成因分析"),
				Solution: text, Confidence: 0.85, Source: "LLM(多模态)", TokensUsed: tokens}
		}
	}
	if text, tokens, err := client.Ask(userID, "diagnose", prompt); err == nil {
		return DiagnosticResult{Summary: extractDiag(text, "诊断结论"), Cause: extractDiag(text, "成因分析"),
			Solution: text, Confidence: 0.8, Source: "LLM", TokensUsed: tokens}
	}

	// 规则降级诊断
	return ruleDiagnose(fb, faults)
}

// ruleDiagnose 无 LLM 时的规则兜底诊断
func ruleDiagnose(fb *model.Feedback, faults []model.FaultRecord) DiagnosticResult {
	summary := "基于规则引擎的初步判断"
	cause := "未调用大模型，依据历史故障码推断"
	solution := "建议现场检查信号灯供电与灯珠状态，优先排查断电/断路线路；必要时上传现场图片以启用AI视觉诊断。"
	if len(faults) > 0 {
		f := faults[0]
		summary = fmt.Sprintf("推断为故障码%d（%s）引发的灯光异常", f.ErrCode, f.FaultType)
		solution = "请按该故障类型对应的维修预案处理，建议派单并准备相应备件。"
	}
	return DiagnosticResult{Summary: summary, Cause: cause, Solution: solution, Confidence: 0.6, Source: "规则降级"}
}

// imagePathsToDataURLs 将本地媒体文件路径转为 base64 data URL（支持 jpg/png）
func imagePathsToDataURLs(paths []string) []string {
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		mime := "image/jpeg"
		lp := strings.ToLower(p)
		if strings.HasSuffix(lp, ".png") {
			mime = "image/png"
		} else if strings.HasSuffix(lp, ".gif") {
			mime = "image/gif"
		}
		out = append(out, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(b))
	}
	return out
}

// extractDiag 从 LLM 输出中提取指定段落
func extractDiag(text, key string) string {
	lines := strings.Split(text, "\n")
	var buf []string
	started := false
	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, key) {
			started = true
			buf = append(buf, strings.TrimPrefix(trim, key+":"))
			continue
		}
		if started {
			if strings.Contains(trim, "：") && !strings.HasPrefix(trim, "-") && !strings.HasPrefix(trim, "•") {
				break
			}
			if trim == "" {
				continue
			}
			buf = append(buf, trim)
		}
	}
	if len(buf) == 0 {
		return text
	}
	return strings.Join(buf, " ")
}

func strOr(p *uint32, def string) string {
	if p != nil {
		return fmt.Sprintf("%d", *p)
	}
	return def
}

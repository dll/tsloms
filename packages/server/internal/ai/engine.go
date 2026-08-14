package ai

import (
	"math"
	"time"
)

// DeviceFacts 单台设备的预测输入（规则引擎汇总）
type DeviceFacts struct {
	HwID           uint32
	Intersection   string
	AgeDays        int     // 灯龄(天)
	Online         bool    // 当前是否在线
	FaultCount     int     // 历史活跃故障数
	RecentFaults   int     // 近30天故障数
	FaultTypes     []string // 近期故障类型集合
	AvgCurrentR    float64 // 平均红灯电流
	AvgCurrentY    float64
	AvgCurrentG    float64
	MaxCurrent     float64 // 历史最大电流(任一灯组)
	OfflineCount   int     // 近30天离线次数
	HasMediaAnomaly bool   // 是否有关联异常媒体/反馈
}

// Prediction 单设备预测结果
type Prediction struct {
	DeviceHwID    uint32  `json:"device_hw_id"`
	Intersection  string  `json:"intersection"`
	HealthScore   int     `json:"health_score"` // 0-100
	RiskLevel     string  `json:"risk_level"`   // low/medium/high/critical
	PredictType   string  `json:"predict_type"` // 预测故障类型
	RemainDays    int     `json:"remain_days"`  // 剩余寿命(天)
	Confidence    float64 `json:"confidence"`   // 0-1
	Factors       []string `json:"factors"`     // 风险因子（人类可读）
	Plan          string  `json:"plan"`         // 应对预案
	Source        string  `json:"source"`       // 规则/LLM
}

// riskLabels 风险等级中文
var riskLabels = map[string]string{
	"low":      "低",
	"medium":   "中",
	"high":     "高",
	"critical": "极高",
}

// RiskLabel 导出风险等级中文
func RiskLabel(l string) string { return riskLabels[l] }

// predictTypeText 预测故障类型中文（复用故障含义）
func predictTypeText(t string) string {
	switch t {
	case "lamp_off": return "灯灭(可能断路/灯珠烧毁)"
	case "power_loss": return "断电(供电线路故障)"
	case "timeout": return "亮灯超时(控制器老化)"
	case "abnormal_on": return "异常同亮(驱动短路)"
	case "dim": return "缺亮(灯珠衰减)"
	case "offline": return "离线(通信/供电中断)"
	default: return "综合退化"
	}
}

// PredictDevice 规则引擎预测单台设备（离线，不耗 LLM 额度）
// 按灯龄、故障频率、电流异常、离线次数综合评分
func PredictDevice(f DeviceFacts) Prediction {
	// 健康分从 100 起扣
	score := 100.0
	maxRisk := 0.0            // 累积风险评分(用于等级)
	var riskSources []string

	// 1) 灯龄老化：超过5年(1825天)开始明显衰减
	if f.AgeDays > 0 {
		age := f.AgeDays / 365.0
		switch {
		case age >= 10:
			score -= 35; maxRisk += 45
			riskSources = append(riskSources, "灯龄超过10年，达设计寿命上限")
		case age >= 7:
			score -= 25; maxRisk += 30
			riskSources = append(riskSources, "灯龄7-10年，接近寿命末期")
		case age >= 5:
			score -= 15; maxRisk += 18
			riskSources = append(riskSources, "灯龄5-7年，进入衰减期")
		case age >= 3:
			score -= 8; maxRisk += 8
		}
	}

	// 2) 历史故障频率
	switch {
	case f.RecentFaults >= 4:
		score -= 30; maxRisk += 40
		riskSources = append(riskSources, "近30天故障频繁(4次以上)")
	case f.RecentFaults >= 2:
		score -= 18; maxRisk += 22
		riskSources = append(riskSources, "近30天多次故障")
	case f.RecentFaults >= 1:
		score -= 10; maxRisk += 12
	}

	// 3) 电流异常：过大(老化/短路)或近期异常
	curMax := math.Max(f.AvgCurrentR, math.Max(f.AvgCurrentY, f.AvgCurrentG))
	switch {
	case curMax >= 900:
		score -= 25; maxRisk += 35
		riskSources = append(riskSources, "电流偏高，灯珠驱动负载大")
	case curMax >= 650 && curMax < 900:
		score -= 12; maxRisk += 15
	}

	// 4) 离线次数：通信/供电不稳定
	switch {
	case f.OfflineCount >= 5:
		score -= 20; maxRisk += 25
		riskSources = append(riskSources, "多次离线，供电/通信不稳定")
	case f.OfflineCount >= 2:
		score -= 10; maxRisk += 12
	case f.OfflineCount >= 1:
		score -= 5
	}

	// 5) 关联异常媒体/反馈
	if f.HasMediaAnomaly {
		score -= 10; maxRisk += 12
		riskSources = append(riskSources, "有异常媒体举证或反馈")
	}

	// 6) 当前离线本身
	if !f.Online {
		score -= 8
		riskSources = append(riskSources, "当前离线")
	}

	// 收敛分数
	if score < 0 { score = 0 }
	health := int(math.Round(score))

	// 风险等级（按 maxRisk 累积分）
	risk := "low"
	var predictType string
	switch {
	case maxRisk >= 50:
		risk = "critical"
		predictType = "power_loss"
	case maxRisk >= 32:
		risk = "high"
		predictType = "lamp_off"
	case maxRisk >= 18:
		risk = "medium"
		predictType = "timeout"
	default:
		risk = "low"
		predictType = "dim"
	}

	// 若近期有具体故障类型，优先预测同类复发
	if len(f.FaultTypes) > 0 {
		predictType = f.FaultTypes[0]
	}

	// 剩余寿命预估（天）：健康分线性映射 0~3650 天
	remain := int(float64(health) / 100.0 * 3650.0)

	// 置信度
	conf := 0.55 + 0.35*math.Min(1.0, float64(f.FaultCount+f.RecentFaults+1)/5.0)
	if conf > 0.92 { conf = 0.92 }

	// 应对预案（规则兜底，LLM 增强时覆盖）
	plan := buildPlan(risk, predictTypeText(predictType), health, remain)

	return Prediction{
		DeviceHwID:   f.HwID,
		Intersection: f.Intersection,
		HealthScore:  health,
		RiskLevel:    risk,
		PredictType:  predictTypeText(predictType),
		RemainDays:   remain,
		Confidence:   conf,
		Factors:      riskSources,
		Plan:         plan,
		Source:       "规则",
	}
}

// buildPlan 生成应对预案文本
func buildPlan(risk, faultType string, health, remain int) string {
	priority := map[string]string{
		"critical": "【紧急】24小时内安排现场检修",
		"high":     "【优先】3天内安排检修",
		"medium":   "【计划】1周内纳入检修计划",
		"low":      "【常规】纳入周期巡检",
	}[risk]
	return priority + "；预测故障：" + faultType + "；健康分" +
		itoa(health) + "/100，预估剩余寿命约" + itoa(remain) + "天。" +
		"建议：检查对应灯组供电与灯珠状态，备件准备，优先排查断电/断路线路。"
}

func itoa(n int) string {
	if n == 0 { return "0" }
	neg := n < 0
	if neg { n = -n }
	var b []byte
	for n > 0 { b = append([]byte{byte('0' + n%10)}, b...); n /= 10 }
	if neg { b = append([]byte{'-'}, b...) }
	return string(b)
}

// NowAgeDays 计算灯龄（天）
func NowAgeDays(installed *time.Time) int {
	if installed == nil { return 0 }
	return int(time.Since(*installed).Hours() / 24)
}

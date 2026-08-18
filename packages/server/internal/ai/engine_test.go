package ai

import (
	"testing"
	"time"
)

// 构造基础无风险设备
func baseFacts() DeviceFacts {
	return DeviceFacts{
		HwID:         "1",
		Intersection: "人民路与建设路交叉口",
		AgeDays:      100,
		Online:       true,
	}
}

func TestPredictDevice_LowRisk(t *testing.T) {
	f := baseFacts()
	p := PredictDevice(f)
	if p.Source != "规则" {
		t.Errorf("期望规则引擎 source=规则, got=%s", p.Source)
	}
	if p.RiskLevel != "low" {
		t.Errorf("新设备无风险, 期望 low, got=%s", p.RiskLevel)
	}
	if p.HealthScore < 80 {
		t.Errorf("新设备健康分应较高, got=%d", p.HealthScore)
	}
	if p.RemainDays <= 0 {
		t.Errorf("剩余寿命应>0, got=%d", p.RemainDays)
	}
	if p.Confidence <= 0 || p.Confidence > 1 {
		t.Errorf("置信度应在(0,1], got=%f", p.Confidence)
	}
	if p.Plan == "" {
		t.Error("预案不应为空")
	}
}

func TestPredictDevice_HighRiskOldAge(t *testing.T) {
	f := baseFacts()
	f.AgeDays = 365 * 12 // 灯龄12年，达寿命上限
	f.RecentFaults = 5
	f.OfflineCount = 6
	p := PredictDevice(f)
	if p.RiskLevel != "critical" && p.RiskLevel != "high" {
		t.Errorf("重风险设备应 critical/high, got=%s", p.RiskLevel)
	}
	if p.HealthScore >= 60 {
		t.Errorf("重风险设备健康分应低, got=%d", p.HealthScore)
	}
	// 高负载电流
	f2 := baseFacts()
	f2.AgeDays = 365 * 8
	f2.AvgCurrentR = 950
	f2.RecentFaults = 3
	p2 := PredictDevice(f2)
	if p2.RiskLevel == "low" {
		t.Error("老化+高电流不应 low")
	}
}

func TestPredictDevice_Offline(t *testing.T) {
	f := baseFacts()
	f.Online = false
	p := PredictDevice(f)
	if !contains(p.Factors, "当前离线") {
		t.Errorf("离线设备应含'当前离线'因子, got=%v", p.Factors)
	}
}

func TestPredictDevice_FaultTypeRecurrence(t *testing.T) {
	f := baseFacts()
	f.AgeDays = 365 * 6
	f.RecentFaults = 2
	f.FaultTypes = []string{"lamp_off"}
	p := PredictDevice(f)
	if p.PredictType == "" {
		t.Error("预测类型不应为空")
	}
}

func TestRiskLabel(t *testing.T) {
	if RiskLabel("high") != "高" {
		t.Error("RiskLabel(high) 应为 高")
	}
	if RiskLabel("unknown") != "" {
		t.Error("RiskLabel(unknown) 应为空")
	}
}

func TestNowAgeDays(t *testing.T) {
	installed := time.Now().AddDate(0, 0, -10)
	d := NowAgeDays(&installed)
	if d < 9 || d > 11 {
		t.Errorf("10天前安装应约10天, got=%d", d)
	}
	if NowAgeDays(nil) != 0 {
		t.Error("nil 时间应返回0")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

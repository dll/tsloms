package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

func TestSwVersion(t *testing.T) {
	// 固件版本号高位 4bit=major, 次 4bit=minor
	if swMajor(0x12000000) != 1 {
		t.Errorf("swMajor(0x12...) 应=1, got=%d", swMajor(0x12000000))
	}
	if swMinor(0x12000000) != 2 {
		t.Errorf("swMinor(0x12...) 应=2, got=%d", swMinor(0x12000000))
	}
	if swMajor(0) != 0 || swMinor(0) != 0 {
		t.Error("0 版本号应全为0")
	}
}

func TestRuleDiagnose_NoFaults(t *testing.T) {
	fb := &model.Feedback{Title: "灯不亮", Content: "红灯全灭", Intersection: "人民路"}
	r := ruleDiagnose(fb, nil)
	if r.Source != "规则降级" {
		t.Errorf("expected 规则降级, got=%s", r.Source)
	}
	if r.Summary == "" || r.Solution == "" {
		t.Error("规则诊断结论/方案不应为空")
	}
}

func TestRuleDiagnose_WithFaults(t *testing.T) {
	faults := []model.FaultRecord{
		{ErrCode: 5, FaultType: "lamp_off", Status: "active", FaultLevel: "high",
			CreatedAt: time.Now()},
	}
	fb := &model.Feedback{Title: "灯不亮", Content: "红灯全灭"}
	r := ruleDiagnose(fb, faults)
	if !strings.Contains(r.Summary, "故障码5") {
		t.Errorf("应包含故障码5, got=%s", r.Summary)
	}
	// 应从本地时间线偏移：CreatedAt 相对 — 这里无需解析，仅验证 summary 带故障码
}

func TestImagePathsToDataURLs(t *testing.T) {
	dir := t.TempDir()
	// 合法小 PNG
	pngPath := filepath.Join(dir, "a.png")
	os.WriteFile(pngPath, []byte{0x89, 0x50, 0x4E, 0x47}, 0o644)
	// 太大文件（>4MB）应被跳过
	bigPath := filepath.Join(dir, "big.png")
	os.WriteFile(bigPath, make([]byte, 5*1024*1024), 0o644)
	// 不存在的文件应跳过

	urls := imagePathsToDataURLs([]string{pngPath, bigPath, filepath.Join(dir, "nope.png"), ""})
	if len(urls) != 1 {
		t.Fatalf("应只有 1 个合法 dataURL, got=%d", len(urls))
	}
	if !strings.HasPrefix(urls[0], "data:image/png;base64,") {
		t.Errorf("PNG 前缀错误: %s", urls[0][:30])
	}
	if strings.Contains(urls[0], "big") {
		t.Error("不应包含大文件")
	}
}

func TestExtractDiag(t *testing.T) {
	text := "诊断结论：灯丝故障\n成因分析：电压不稳\n解决方案：更换灯珠\n建议备件：LED灯珠\n"
	if got := extractDiag(text, "诊断结论"); got != "灯丝故障" {
		t.Errorf("extract 结论错误: %q", got)
	}
	if got := extractDiag(text, "成因分析"); got != "电压不稳" {
		t.Errorf("extract 成因错误: %q", got)
	}
	// 不存在的 key 返回原文
	if got := extractDiag(text, "不存在"); got != text {
		t.Errorf("未知 key 应返回原文")
	}
}

func TestLifecycleTypeCount(t *testing.T) {
	r := LifecycleResult{Timeline: []LifecycleEvent{
		{Type: "install"}, {Type: "fault"}, {Type: "fault"}, {Type: "workorder"},
	}}
	m := r.TypeCount()
	if m["fault"] != 2 || m["install"] != 1 || m["workorder"] != 1 {
		t.Errorf("TypeCount 统计错误: %v", m)
	}
	if r.TypeCount()["nonexist"] != 0 {
		t.Error("不存在的类型应为0")
	}
}

func TestStrOr(t *testing.T) {
	var p *uint32
	if strOr(p, "-") != "-" {
		t.Error("nil 指针应返回默认值")
	}
	v := uint32(7)
	if strOr(&v, "-") != "7" {
		t.Error("指针应返回数字")
	}
}

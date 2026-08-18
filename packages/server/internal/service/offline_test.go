package service

import (
	"context"
	"testing"
	"time"

	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// OfflineCheck.runOnce：超时离线设备被置离线，未超时保持在线
func TestOfflineCheckRunOnce(t *testing.T) {
	model.InitTestDB()

	// 造 3 台设备：一台超时在线、一台在线未超时、一台已离线
	past := time.Now().Add(-20 * time.Minute)
	recent := time.Now().Add(-1 * time.Minute)
	model.DB.Create(&model.Device{HwID: "O001", OnlineStatus: true, LastCheckinAt: &past})
	model.DB.Create(&model.Device{HwID: "O002", OnlineStatus: true, LastCheckinAt: &recent})
	model.DB.Create(&model.Device{HwID: "O003", OnlineStatus: false, LastCheckinAt: &past})

	cfg := &config.Config{OfflineAfterMin: 6}
	oc := NewOfflineCheck(cfg)
	if oc == nil {
		t.Fatal("NewOfflineCheck nil")
	}
	oc.runOnce()

	var o1, o2, o3 model.Device
	model.DB.Where("hw_id = ?", "O001").First(&o1)
	model.DB.Where("hw_id = ?", "O002").First(&o2)
	model.DB.Where("hw_id = ?", "O003").First(&o3)
	if o1.OnlineStatus {
		t.Error("O001 超时应被置离线")
	}
	if !o2.OnlineStatus {
		t.Error("O002 未超时应保持在线")
	}
	if o3.OnlineStatus {
		t.Error("O003 已离线不应被改在线")
	}
}

// OfflineCheck.Start/Done：启动后取消 ctx 应触发 Done 关闭
func TestOfflineCheckStartDone(t *testing.T) {
	model.InitTestDB()
	cfg := &config.Config{OfflineAfterMin: 6}
	oc := NewOfflineCheck(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	oc.Start(ctx)
	done := oc.Done()
	select {
	case <-done:
		t.Error("Done 不应在取消前关闭")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
		// 正常关闭
	case <-time.After(2 * time.Second):
		t.Error("取消 ctx 后 Done 应在 2s 内关闭")
	}
}

// filterHighRiskAIDevices：仅保留 AI 预测为 high/critical 的设备
func TestFilterHighRiskAIDevices(t *testing.T) {
	model.InitTestDB()

	model.DB.Create(&model.Device{HwID: "H001", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: "H002", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: "H003", OnlineStatus: true})

	model.DB.Create(&model.AIPrediction{DeviceHwID: "H001", RiskLevel: "high"})
	model.DB.Create(&model.AIPrediction{DeviceHwID: "H002", RiskLevel: "low"})

	candidates := []model.Device{
		{HwID: "H001"}, {HwID: "H002"}, {HwID: "H003"},
	}
	out := filterHighRiskAIDevices(candidates)
	if len(out) != 1 || out[0].HwID != "H001" {
		t.Errorf("应仅保留 H001, got %+v", out)
	}

	// 空输入
	if got := filterHighRiskAIDevices(nil); got != nil {
		t.Errorf("空输入应返回 nil")
	}
	// 无任何高危
	model.DB.Create(&model.Device{HwID: "H004", OnlineStatus: true})
	if got := filterHighRiskAIDevices([]model.Device{{HwID: "H004"}}); len(got) != 0 {
		t.Errorf("无高危应返回空, got %+v", got)
	}
}

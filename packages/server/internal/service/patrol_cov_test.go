package service

import (
	"context"
	"testing"
	"time"

	"github.com/tsloms/server/internal/ai"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

func svcSeed(t *testing.T) {
	t.Helper()
	model.InitTestDB()
	op := model.User{Username: "op_svc", Role: model.RoleOperator, Status: model.UserStatusEnabled, PasswordHash: "x"}
	model.DB.Create(&op)
	model.DB.Create(&model.Device{HwID: "6001", OnlineStatus: true})
	// 低库存 + 缺货物料
	model.DB.Create(&model.Material{Code: "L1", Name: "低库存灯珠", Status: "active", Stock: 2, Threshold: 5})
	model.DB.Create(&model.Material{Code: "O1", Name: "缺货电源", Status: "active", Stock: 0, Threshold: 3})
}

func TestPatrol_CheckStockAlerts(t *testing.T) {
	svcSeed(t)
	p := NewPatrolService()
	alerts := 0
	p.checkStockAlerts(&alerts)
	if alerts != 2 {
		t.Errorf("低库存+缺货应共 2 个告警, got %d", alerts)
	}
	// lowStockNames 两个分支
	if len(p.lowStockNames(true)) != 1 {
		t.Errorf("低库存(>0)应 1 个, got %v", p.lowStockNames(true))
	}
	if len(p.lowStockNames(false)) != 1 {
		t.Errorf("缺货(<=0)应 1 个, got %v", p.lowStockNames(false))
	}
}

func TestPatrol_BuildReportContent(t *testing.T) {
	p := NewPatrolService()
	s := &ai.DailySnapshot{Date: "2026-08-16"}
	c := p.buildReportContent(s)
	if c == "" {
		t.Error("日报内容为空")
	}
}

func TestPatrol_CheckAlerts(t *testing.T) {
	svcSeed(t)
	p := NewPatrolService()
	// 超时工单
	now := time.Now()
	model.DB.Create(&model.WorkOrder{OrderNo: "WOov", Status: model.WorkOrderStatusPending, CreatedAt: now.Add(-48 * time.Hour)})
	// 高风险设备
	model.DB.Create(&model.AIPrediction{DeviceHwID: "6001", BatchID: "B1", RiskLevel: "high", HealthScore: 40, PredictType: "x"})
	s := &ai.DailySnapshot{Date: "2026-08-16", OverdueOrders: 2,
		HighRiskDevices: []ai.MaterialMin{{ID: 1, Name: "设备#6001"}}}
	p.checkAlerts(s)
	// 无告警分支
	p.checkAlerts(&ai.DailySnapshot{})
}

func TestPatrol_RunPatrol(t *testing.T) {
	svcSeed(t)
	p := NewPatrolService()
	p.patrol() // 生成日报 + 通知 + 异常
	// 应有通知生成
	var cnt int64
	model.DB.Model(&model.Notification{}).Count(&cnt)
	if cnt < 1 {
		t.Errorf("巡检后应有通知, got %d", cnt)
	}
}

func TestPatrol_DoneChannel(t *testing.T) {
	p := NewPatrolService()
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	cancel()
	select {
	case <-p.Done():
		// 正常停止
	case <-time.After(2 * time.Second):
		t.Error("Done 通道未关闭")
	}
}

func TestOfflineCheck_RunOnce(t *testing.T) {
	model.InitTestDB()
	// 超时设备（在线 + last_checkin 很久前）→ 置离线
	old := time.Now().Add(-30 * time.Minute)
	model.DB.Create(&model.Device{HwID: "7001", OnlineStatus: true, LastCheckinAt: &old})
	// 未超时设备
	recent := time.Now()
	model.DB.Create(&model.Device{HwID: "7002", OnlineStatus: true, LastCheckinAt: &recent})
	// 无签到设备（last_checkin NULL）不受影响
	model.DB.Create(&model.Device{HwID: "7003", OnlineStatus: true})

	cfg := config.Load()
	cfg.OfflineAfterMin = 6
	o := NewOfflineCheck(cfg)
	o.runOnce()

	var d1, d2 model.Device
	model.DB.Where("hw_id = ?", "7001").First(&d1)
	model.DB.Where("hw_id = ?", "7002").First(&d2)
	if d1.OnlineStatus {
		t.Error("超时设备应置离线")
	}
	if !d2.OnlineStatus {
		t.Error("未超时设备应保持在线")
	}
}

func TestOfflineCheck_NilDB(t *testing.T) {
	oldDB := model.DB
	model.DB = nil
	defer func() { model.DB = oldDB }()
	cfg := config.Load()
	cfg.OfflineAfterMin = 1
	o := NewOfflineCheck(cfg)
	o.runOnce() // 应安全返回
	if o.timeout <= 0 {
		t.Error("timeout 应>0")
	}
}

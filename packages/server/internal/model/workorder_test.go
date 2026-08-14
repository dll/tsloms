package model

import (
	"strings"
	"testing"
	"time"
)

func TestNextOrderNo_Sequential(t *testing.T) {
	db := InitTestDB()
	prefix := "WO" + time.Now().Format("20060102")
	db.Create(&WorkOrder{OrderNo: prefix + "0001", Status: WorkOrderStatusPending})
	db.Create(&WorkOrder{OrderNo: prefix + "0002", Status: WorkOrderStatusPending})

	no := NextOrderNo(db)
	if !strings.HasPrefix(no, prefix) {
		t.Errorf("工单编号 %s 前缀应为 %s", no, prefix)
	}
	if !strings.HasSuffix(no, "0003") {
		t.Errorf("工单编号 %s 序列应为 0003", no)
	}
}

func TestNextOrderNo_NewDateStartsAt0001(t *testing.T) {
	db := InitTestDB()
	// 模拟昨天的一张工单，不应计入今天的序列
	suffix := time.Now().AddDate(0, 0, -1).Format("20060102")
	db.Create(&WorkOrder{OrderNo: "WO" + suffix + "0001", Status: WorkOrderStatusPending})

	no := NextOrderNo(db)
	if !strings.HasSuffix(no, "0001") {
		t.Errorf("昨日工单不应计今日序列, 实际 %s 期望后缀 0001", no)
	}
}

func TestWorkOrderConstants(t *testing.T) {
	if WorkOrderStatusPending != "pending" ||
		WorkOrderStatusProcessing != "processing" ||
		WorkOrderStatusCompleted != "completed" ||
		WorkOrderStatusRejected != "rejected" {
		t.Error("工单状态常量定义不完整")
	}
}

func TestWorkOrderOverdueHours(t *testing.T) {
	now := time.Now()
	// pending 未超时（刚创建）
	fresh := WorkOrder{Status: WorkOrderStatusPending, CreatedAt: now}
	if h := WorkOrderOverdueHours(&fresh); h != 0 {
		t.Errorf("新工单不应超时, got=%v", h)
	}
	// pending 超 25 小时
	p25 := WorkOrder{Status: WorkOrderStatusPending, CreatedAt: now.Add(-25 * time.Hour)}
	if h := WorkOrderOverdueHours(&p25); h <= 0 {
		t.Errorf("pending 超25h应超时, got=%v", h)
	}
	// processing 未超时（23h < 48h SLA）
	p23 := WorkOrder{Status: WorkOrderStatusProcessing, CreatedAt: now.Add(-23 * time.Hour)}
	if h := WorkOrderOverdueHours(&p23); h != 0 {
		t.Errorf("processing 23h未超48h SLA, got=%v", h)
	}
	// processing 超 49 小时
	p49 := WorkOrder{Status: WorkOrderStatusProcessing, CreatedAt: now.Add(-49 * time.Hour)}
	if h := WorkOrderOverdueHours(&p49); h <= 0 {
		t.Errorf("processing 超49h应超时, got=%v", h)
	}
	// completed 不参与超时
	completed := WorkOrder{Status: WorkOrderStatusCompleted, CreatedAt: now.Add(-200 * time.Hour)}
	if h := WorkOrderOverdueHours(&completed); h != 0 {
		t.Errorf("已完成工单不计超时, got=%v", h)
	}
	// nil 不 panic
	if h := WorkOrderOverdueHours(nil); h != 0 {
		t.Errorf("nil 应返回0, got=%v", h)
	}
}

func TestUserRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" || RoleOperator != "operator" || RoleViewer != "viewer" {
		t.Error("用户角色常量定义不完整")
	}
}

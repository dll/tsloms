package model

import (
	"strings"
	"sync"
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

// TestEnsureActiveWorkOrder_SingleReuse M1 幂等：EnsureActiveWorkOrder 对同一故障重复调用只返回同一张活跃工单，
// 不新建；且 fault.work_order_id 只回填一次并指向该单。
func TestEnsureActiveWorkOrder_SingleReuse(t *testing.T) {
	db := InitTestDB()

	// 准备一个故障记录
	now := time.Now()
	f := FaultRecord{DeviceHwID: 1, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: FaultStatusOccurred, FirstSeen: now, LastSeen: now, RecognitionStatus: RecognitionConfirmed}
	if err := db.Create(&f).Error; err != nil {
		t.Fatalf("建故障失败: %v", err)
	}

	// 首次派单：新建一条活跃工单
	wo1 := EnsureActiveWorkOrder(db, f.ID, f.DeviceHwID)
	if wo1 == nil {
		t.Fatal("首次派单应新建工单")
	}
	var cnt int64
	db.Model(&WorkOrder{}).Where("fault_id = ?", f.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("首次派单后工单数 = %d, 期望 1", cnt)
	}
	var ff FaultRecord
	db.First(&ff, f.ID)
	if ff.WorkOrderID == nil || *ff.WorkOrderID != wo1.ID {
		t.Fatalf("fault 应回写 work_order_id=%d, got %v", wo1.ID, ff.WorkOrderID)
	}
	if ff.Status != FaultStatusConfirmed {
		t.Errorf("回写后 status 应 confirmed, got %s", ff.Status)
	}

	// 重复派单：复用同一单，不新建
	wo2 := EnsureActiveWorkOrder(db, f.ID, f.DeviceHwID)
	if wo2 == nil || wo2.ID != wo1.ID {
		t.Fatalf("重复派单应复用原单(id=%d), got %+v", wo1.ID, wo2)
	}
	db.Model(&WorkOrder{}).Where("fault_id = ?", f.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("重复派单后工单数应仍为 1, got %d", cnt)
	}
}

// TestEnsureActiveWorkOrder_HistoricalDoesNotBlockReDispatch M1 语义：
// 同一故障完结/驳回的历史工单（FaultActiveScope=NULL）不参与活跃唯一，
// 故障复现后可重新派一张新活跃工单（与历史工单并存）。
func TestEnsureActiveWorkOrder_HistoricalDoesNotBlockReDispatch(t *testing.T) {
	db := InitTestDB()
	now := time.Now()
	f := FaultRecord{DeviceHwID: 2, ErrCode: -2, FaultType: "power_loss", FaultLevel: "critical",
		Status: FaultStatusConfirmed, FirstSeen: now, LastSeen: now, RecognitionStatus: RecognitionConfirmed}
	db.Create(&f)

	// 历史已完成工单（不占活跃位）
	closed := WorkOrder{OrderNo: "WO_HIS_1", FaultID: f.ID, DeviceHwID: f.DeviceHwID,
		Status: WorkOrderStatusCompleted, FaultActiveScope: nil}
	if err := db.Create(&closed).Error; err != nil {
		t.Fatalf("建历史工单失败: %v", err)
	}

	// 故障复现→重新派单：应允许新活跃工单成功（历史单不阻挠）
	var ff FaultRecord
	db.First(&ff, f.ID)
	ff.WorkOrderID = nil // 模拟故障重新进入可派单状态
	ff.Status = FaultStatusOccurred
	db.Save(&ff)

	wo := EnsureActiveWorkOrder(db, f.ID, f.DeviceHwID)
	if wo == nil {
		t.Fatal("历史工单不应阻挠新活跃工单的创建")
	}
	if wo.ID == closed.ID {
		t.Fatal("不应复用了历史已完成工单")
	}
	if wo.Status != WorkOrderStatusPending {
		t.Errorf("新工单状态应 pending, got %s", wo.Status)
	}
}

// TestEnsureActiveWorkOrder_ConcurrentConverges M1 并发幂等：
// 同一故障被多个并发入口同时派单，最终只收敛为一【活跃】工单，fault 只指向该单。
// 依赖 uk_wo_active_scope(fault_active_scope) 唯一索引 + 条件回写。
func TestEnsureActiveWorkOrder_ConcurrentConverges(t *testing.T) {
	db := InitTestDB()
	now := time.Now()
	f := FaultRecord{DeviceHwID: 3, ErrCode: -3, FaultType: "lamp_off", FaultLevel: "critical",
		Status: FaultStatusOccurred, FirstSeen: now, LastSeen: now, RecognitionStatus: RecognitionConfirmed}
	db.Create(&f)

	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			EnsureActiveWorkOrder(db, f.ID, f.DeviceHwID)
		}()
	}
	wg.Wait()

	var cnt int64
	db.Model(&WorkOrder{}).
		Where("fault_id = ? AND status IN ?", f.ID, []string{WorkOrderStatusPending, WorkOrderStatusProcessing}).
		Count(&cnt)
	if cnt != 1 {
		t.Fatalf("并发派单后活跃工单数 = %d, 期望 1", cnt)
	}
	var ff FaultRecord
	db.First(&ff, f.ID)
	if ff.WorkOrderID == nil {
		t.Error("并发派单后 fault 应回写 work_order_id")
	}
}

func TestUserRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" || RoleOperator != "operator" || RoleViewer != "viewer" {
		t.Error("用户角色常量定义不完整")
	}
}

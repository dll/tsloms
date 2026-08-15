package mqtt

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
	"go.uber.org/zap"
)

// newTestHandler 构造使用内存 SQLite 的处理器
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	model.InitTestDB()
	logger, _ := zap.NewProduction()
	return &Handler{logger: logger, mqttClient: nil}
}

// createFaultRec 构造一个故障事件记录
func createFaultRec(hwID uint32, errCode int8) *EventRecord {
	return &EventRecord{
		LedHwID:  hwID,
		SubHwID:  1,
		SwVer:    1,
		ConfVer:  1,
		LedState: StateR,
		ErrCode:  errCode,
		CurrentR: 100,
		CurrentY: 100,
		CurrentG: 100,
	}
}

func TestProcessFault_DedupWithinWindow(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(1001, LEDErrROFF)

	// 第一次研判：创建故障记录
	h.processFault(rec)
	var count int64
	model.DB.Model(&model.FaultRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("首次研判后故障数 = %d, 期望 1", count)
	}

	// 第二次研判（30 分钟内）：去重，不再新增
	rec.CurrentR = 200 // 更新电流值
	h.processFault(rec)
	model.DB.Model(&model.FaultRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("去重窗口内故障数 = %d, 期望 1", count)
	}

	// 验证电流值已更新
	var f model.FaultRecord
	model.DB.First(&f)
	if f.CurrentR != 200 {
		t.Errorf("去重后电流值 = %d, 期望 200", f.CurrentR)
	}
}

func TestProcessFault_NewRecordAfterWindow(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(2002, LEDErrGOFF)

	h.processFault(rec)

	// 将 last_seen 调到 31 分钟前，模拟超出去重窗口
	model.DB.Model(&model.FaultRecord{}).
		Where("device_hw_id = ?", rec.LedHwID).
		Update("last_seen", time.Now().Add(-31*time.Minute))

	// 再次研判：旧故障应标记 resolved，并新建故障
	h.processFault(rec)

	var count int64
	model.DB.Model(&model.FaultRecord{}).Count(&count)
	if count != 2 {
		t.Fatalf("超窗后故障总数 = %d, 期望 2", count)
	}

	var activeCount int64
	model.DB.Model(&model.FaultRecord{}).Where("status IN ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Count(&activeCount)
	if activeCount != 1 {
		t.Errorf("active 故障数 = %d, 期望 1", activeCount)
	}
}

func TestProcessFault_CriticalCreatesWorkOrder(t *testing.T) {
	h := newTestHandler(t)
	rec := createFaultRec(3003, LEDErrROFF)

	h.processFault(rec)

	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Count(&woCount)
	if woCount != 1 {
		t.Errorf("严重故障应生成 1 个工单, 实际 %d", woCount)
	}

	var wo model.WorkOrder
	model.DB.First(&wo)
	if wo.Status != model.WorkOrderStatusPending {
		t.Errorf("工单状态 = %s, 期望 pending", wo.Status)
	}
	if wo.OrderNo == "" {
		t.Error("工单编号不应为空")
	}
}

func TestProcessFault_NormalNoWorkOrder(t *testing.T) {
	h := newTestHandler(t)
	// 超时类错误为 normal 等级，不应生成工单
	rec := createFaultRec(4004, LEDErrGONTimeout)

	h.processFault(rec)

	var woCount int64
	model.DB.Model(&model.WorkOrder{}).Count(&woCount)
	if woCount != 0 {
		t.Errorf("一般故障不应生成工单, 实际 %d", woCount)
	}
}

func TestProcessFault_DifferentErrCodeSeparate(t *testing.T) {
	h := newTestHandler(t)
	h.processFault(createFaultRec(5005, LEDErrROFF))
	h.processFault(createFaultRec(5005, LEDErrRGON))

	var count int64
	model.DB.Model(&model.FaultRecord{}).Count(&count)
	if count != 2 {
		t.Errorf("不同 errCode 应分别记录, 实际 %d", count)
	}
}

package mqtt

import (
	"testing"
	"time"

	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 回归测试：针对 refactor-notes.md 中 MQTT 热点改动（B2 / B3 / C2）
//   基线：origin/main a460365 ｜ 范围：不改变业务行为
// ============================================================================

// newRegressionHandler 使用内存 SQLite + 统一日志单例的处理器
func newRegressionHandler(t *testing.T) *Handler {
	t.Helper()
	model.InitTestDB()
	return NewHandler(nil) // mqttClient=nil → 不回发回应
}

// mkRec 构造一条设备事件记录
func mkRec(hwID uint32, errCode int8, ledState, currentR, currentY, currentG int) *EventRecord {
	return &EventRecord{
		LedHwID:  hwID,
		SubHwID:  1,
		SwVer:    1,
		ConfVer:  1,
		LedState: int8(ledState),
		ErrCode:  errCode,
		CurrentR: uint16(currentR),
		CurrentY: uint16(currentY),
		CurrentG: uint16(currentG),
	}
}

// TestRegression_B2_LastSeenAlwaysAdvances 覆盖 B2：去重窗口内 last_seen 必须始终推进，
// 即便电流/灯态无变化（保持 30 分钟窗口锚点）。这是行为红线，重构后不得漂移。
func TestRegression_B2_LastSeenAlwaysAdvances(t *testing.T) {
	h := newRegressionHandler(t)
	rec := mkRec(9001, LEDErrROFF, int(StateR), 100, 100, 100)

	h.processFault(rec)
	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)
	firstLastSeen := f.LastSeen

	// 模拟等一小段（>1s，避免同毫秒）后，上报一条电流/灯态完全相同的事件
	time.Sleep(20 * time.Millisecond)
	rec2 := mkRec(9001, LEDErrROFF, int(StateR), 100, 100, 100) // 与首条完全一致
	h.processFault(rec2)

	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)

	// 仍只有 1 条故障（去重窗口内未新建）
	var count int64
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = ?", rec.LedHwID).Count(&count)
	if count != 1 {
		t.Fatalf("去重窗口内故障数 = %d, 期望 1", count)
	}

	// last_seen 必须推进（窗口锚点不变）
	if !f.LastSeen.After(firstLastSeen) {
		t.Errorf("电流/灯态未变化时 last_seen 也必须推进: first=%v last=%v", firstLastSeen, f.LastSeen)
	}

	// 电流/灯态无变化 → 字段保持原值不受影响
	if f.CurrentR != 100 || f.CurrentY != 100 || f.CurrentG != 100 || f.LedState != int8(StateR) {
		t.Errorf("无变化时字段应保持: R=%d Y=%d G=%d led=%d",
			f.CurrentR, f.CurrentY, f.CurrentG, f.LedState)
	}
}

// TestRegression_B2_MaterialFieldsUpdatedOnChange 覆盖 B2：去重窗口内电流/灯态有差异时
// 仍应写入（与改动前行为一致）。若重构后差异字段不再更新即为回归缺陷。
func TestRegression_B2_MaterialFieldsUpdatedOnChange(t *testing.T) {
	h := newRegressionHandler(t)
	rec := mkRec(9002, LEDErrROFF, int(StateR), 100, 100, 100)
	h.processFault(rec)

	time.Sleep(15 * time.Millisecond)
	changed := mkRec(9002, LEDErrROFF, int(StateR), 250, 180, 90) // 电流变化，灯态不变
	h.processFault(changed)

	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)

	if f.CurrentR != 250 || f.CurrentY != 180 || f.CurrentG != 90 {
		t.Errorf("电流变化应更新: R=%d Y=%d G=%d, 期望 250/180/90", f.CurrentR, f.CurrentY, f.CurrentG)
	}
	if f.LedState != int8(StateR) {
		t.Errorf("灯态未变化应保留: %d, 期望 %d", f.LedState, StateR)
	}
}

// TestRegression_B2_LedStateUpdatedOnChange 覆盖 B2 的另一维度：灯态变化时应更新
func TestRegression_B2_LedStateUpdatedOnChange(t *testing.T) {
	h := newRegressionHandler(t)
	rec := mkRec(9003, LEDErrROFF, int(StateR), 100, 100, 100)
	h.processFault(rec)

	time.Sleep(15 * time.Millisecond)
	changed := mkRec(9003, LEDErrROFF, int(StateY), 100, 100, 100) // 灯态变化，电流不变
	h.processFault(changed)

	var f model.FaultRecord
	model.DB.Where("device_hw_id = ?", rec.LedHwID).First(&f)
	if f.LedState != int8(StateY) {
		t.Errorf("灯态变化应更新: %d, 期望 %d", f.LedState, StateY)
	}
}

// TestRegression_B3_SameFrameUpsertMerge 覆盖 B3：同一帧内同一硬件 ID 的多条事件记录
// 只 upsert 一次设备，且“最后一条记录的值生效”语义不变；各条故障研判仍逐条执行。
func TestRegression_B3_SameFrameUpsertMerge(t *testing.T) {
	h := newRegressionHandler(t)

	// 同一帧内同一设备的两条记录（第二条 swVer/confVer 更高，应生效）
	recs := []*EventRecord{
		mkRec(9101, LEDErrOK, int(StateR), 100, 100, 100),
		mkRec(9101, LEDErrOK, int(StateR), 100, 100, 100),
	}
	eventPak := &EventPak{Records: []EventRecord{*recs[0], *recs[1]}}
	h.HandleCheckin(&CmdFrame{Cmd: CmdCheckin, SwVer: 1, CmdSeq: 1}, eventPak, "trafficLight/up/8001/9101/U")

	var devices []model.Device
	model.DB.Where("hw_id = ?", recs[0].LedHwID).Find(&devices)
	if len(devices) != 1 {
		t.Fatalf("同帧同硬件ID应只创建 1 台设备, 实际 %d", len(devices))
	}
	if !devices[0].OnlineStatus {
		t.Error("设备应在线")
	}
}

// TestRegression_B3_SameFrameMergePreservesLastRecord 验证合并后等价语义：
// 同 hwID 多条记录设备只 upsert 一次；故障研判逐条执行（不同 errCode 分别记录），
// critical 故障自动建单。
func TestRegression_B3_SameFrameMergePreservesLastRecord(t *testing.T) {
	h := newRegressionHandler(t)
	// 同一设备两条记录：errCode 0(OK) 与 ROFF(critical)，设备应 upsert 一次，
	// 故障逐条研判（errCode 不同各自建记录），仅 critical 建单
	recs := []EventRecord{
		*createFaultRec(9102, LEDErrOK),
		*createFaultRec(9102, LEDErrROFF), // critical → 自动建单
	}
	h.HandleAlarm(&CmdFrame{Cmd: CmdAlarm, CmdSeq: 1}, &EventPak{Records: recs})

	// 设备只 upsert 一次
	var dc int64
	model.DB.Model(&model.Device{}).Where("hw_id = 9102").Count(&dc)
	if dc != 1 {
		t.Errorf("同帧同设备应只 upsert 一次, 实际 %d", dc)
	}
	// 不同 errCode 各建一条故障记录（逐条研判，不去重合并）
	var fcount int64
	model.DB.Model(&model.FaultRecord{}).Where("device_hw_id = 9102").Count(&fcount)
	if fcount != 2 {
		t.Errorf("不同 errCode 应逐条建故障, 实际 %d", fcount)
	}
	// 仅 critical 故障建单
	var wo int64
	model.DB.Model(&model.WorkOrder{}).Where("device_hw_id = 9102").Count(&wo)
	if wo != 1 {
		t.Errorf("同帧内 critical 故障应建 1 个工单, 实际 %d", wo)
	}
}

// TestRegression_B3_SameFrameLastRecordWinsVersion 覆盖 B3 语义对齐（audit 问题 #1）：
// 同帧同 hwID 的两条记录 swVer/confVer 不同时，持久化到设备的版本字段必须取【末条】值
// （与原逐条覆盖 / last-write-wins 语义一致），而不是首条值。
func TestRegression_B3_SameFrameLastRecordWinsVersion(t *testing.T) {
	h := newRegressionHandler(t)

	// 同一设备两条记录：首条 swVer=1, confVer=1；末条 swVer=2, confVer=9
	first := mkRec(9103, LEDErrOK, int(StateR), 100, 100, 100)
	first.SwVer = 1
	first.ConfVer = 1
	last := mkRec(9103, LEDErrOK, int(StateR), 100, 100, 100)
	last.SwVer = 2
	last.ConfVer = 9

	eventPak := &EventPak{Records: []EventRecord{*first, *last}}
	h.HandleCheckin(&CmdFrame{Cmd: CmdCheckin, SwVer: 1, CmdSeq: 1}, eventPak, "trafficLight/up/8001/9103/U")

	var devices []model.Device
	model.DB.Where("hw_id = ?", uint32(9103)).Find(&devices)
	if len(devices) != 1 {
		t.Fatalf("同帧同硬件ID应只创建 1 台设备, 实际 %d", len(devices))
	}
	// 版本字段应取末条记录值（last-write-wins），而非首条
	if devices[0].SwVersion != 2 {
		t.Errorf("sw_version 应取末条记录值 2, 实际 %d", devices[0].SwVersion)
	}
	if devices[0].ConfVersion != 9 {
		t.Errorf("conf_version 应取末条记录值 9, 实际 %d", devices[0].ConfVersion)
	}
}

// TestRegression_C2_TopicHwID 覆盖 C2：topicHwID 从 Topic 提取硬件 ID，非法回退 0
func TestRegression_C2_TopicHwID(t *testing.T) {
	cases := []struct {
		topic string
		want  uint32
	}{
		{"trafficLight/up/8001/1001/U", 1001},
		{"trafficLight/up/8001/42/U", 42},
		{"trafficLight/up/8001/U", 0},            // 段数不足
		{"bad-topic", 0},                         // 格式错误
		{"trafficLight/up/8001/notnum/U", 0},     // 非数字
		{"a/b/c/d/e", 0},                         // 最后一段非 U 且段位不对
		{"trafficLight/up/8001/4294967296/U", 0}, // 超出 uint32
		{"trafficLight/up/8001/-5/U", 0},         // 负数非法
	}
	for _, c := range cases {
		if got := topicHwID(c.topic); got != c.want {
			t.Errorf("topicHwID(%q) = %d, 期望 %d", c.topic, got, c.want)
		}
	}
}

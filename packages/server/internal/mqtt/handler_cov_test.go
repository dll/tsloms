package mqtt

import (
	"encoding/binary"
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/tsloms/server/internal/model"
)

// buildCheckinFrame 构造一条合法的签到告警帧（带1条事件记录）
func buildCheckinFrame(cmd uint8, hwID uint32, errCode int8) []byte {
	rec := make([]byte, EventRecordLen)
	binary.BigEndian.PutUint32(rec[0:4], hwID)
	binary.BigEndian.PutUint32(rec[8:12], 0x01020000) // swVer
	binary.BigEndian.PutUint32(rec[12:16], 1)         // confVer
	rec[16] = 0x83                                    // ledState
	rec[17] = byte(errCode)                           // errCode
	binary.BigEndian.PutUint16(rec[18:20], 300)       // currentR
	binary.BigEndian.PutUint16(rec[20:22], 200)       // currentY
	binary.BigEndian.PutUint16(rec[22:24], 100)       // currentG

	// 事件包头
	eventPak := make([]byte, EventPakHeaderLen)
	binary.BigEndian.PutUint16(eventPak[0:2], 1) // 1 条记录
	binary.BigEndian.PutUint16(eventPak[2:4], EventRecordLen)
	eventPak = append(eventPak, rec...)

	return BuildCmdFrame(cmd, 0x01020000, 0x0001, 0, eventPak)
}

func mqttHandler(t *testing.T) *Handler {
	t.Helper()
	model.InitTestDB()
	return NewHandler(nil) // mqttClient=nil → 不回发回应，纯 DB 处理
}

// msgStub 实现 paho MQTT.Message 接口的最小桩
func msgStub(topic string, payload []byte) MQTT.Message {
	return &msgStubT{topic: topic, payload: payload}
}

type msgStubT struct {
	topic   string
	payload []byte
}

func (m *msgStubT) Duplicate() bool   { return false }
func (m *msgStubT) Qos() byte         { return 1 }
func (m *msgStubT) Retained() bool    { return false }
func (m *msgStubT) Topic() string     { return m.topic }
func (m *msgStubT) MessageID() uint16 { return 1 }
func (m *msgStubT) Payload() []byte   { return m.payload }
func (m *msgStubT) Ack()              {}

func TestMqttHandleCheckin_UpsertDevice(t *testing.T) {
	h := mqttHandler(t)
	payload := buildCheckinFrame(CmdCheckin, 8001, 0) // 正常签到 errCode=0
	h.HandleMessage(nil, msgStub("trafficLight/up/8001/U", payload))

	var d model.Device
	if err := model.DB.Where("hw_id = ?", 8001).First(&d).Error; err != nil {
		t.Fatalf("应创建设备: %v", err)
	}
	if !d.OnlineStatus {
		t.Error("设备应在线")
	}
	// 报文日志已记录
	var pl int64
	model.DB.Model(&model.PacketLog{}).Where("device_hw_id = ?", 8001).Count(&pl)
	if pl < 1 {
		t.Errorf("应记录报文日志, got %d", pl)
	}
}

func TestMqttHandleAlarm_CriticalFault(t *testing.T) {
	h := mqttHandler(t)
	// 告警：严重故障（errCode 对应 critical）
	payload := buildCheckinFrame(CmdAlarm, 8002, -1)
	h.HandleMessage(nil, msgStub("trafficLight/up/8002/U", payload))

	var f model.FaultRecord
	if err := model.DB.Where("device_hw_id = ?", 8002).First(&f).Error; err != nil {
		t.Fatalf("应创建故障: %v", err)
	}
	if f.FaultLevel != "critical" {
		t.Errorf("严重故障 level=%s", f.FaultLevel)
	}
	// critical 自动生成工单
	var wo int64
	model.DB.Model(&model.WorkOrder{}).Where("fault_id = ?", f.ID).Count(&wo)
	if wo != 1 {
		t.Errorf("critical 应生成工单, got %d", wo)
	}
	// 数据链路闭环：确认故障 → 自动生成预警记录（可转工单/忽略）
	var warn int64
	model.DB.Model(&model.Warning{}).Where("source = ? AND fault_id = ?", model.WarningSourceFault, f.ID).Count(&warn)
	if warn != 1 {
		t.Errorf("确认故障应生成 1 条预警, got %d", warn)
	}
}

func TestMqttHandleAlarm_NoEvents(t *testing.T) {
	h := mqttHandler(t)
	// 无事件记录的告警帧（datLen 无数据）
	frame := BuildCmdFrame(CmdAlarm, 0, 1, 0, nil)
	h.HandleAlarm(&CmdFrame{Cmd: CmdAlarm, CmdSeq: 1}, nil) // eventPak=nil 直接返回
	_ = frame
}

func TestMqttHandlePowerOn(t *testing.T) {
	h := mqttHandler(t)
	payload := buildCheckinFrame(CmdPowerOn, 8003, 0)
	h.HandleMessage(nil, msgStub("trafficLight/up/8003/U", payload))
	var d model.Device
	if err := model.DB.Where("hw_id = ?", 8003).First(&d).Error; err != nil {
		t.Fatalf("上电报备应创建设备: %v", err)
	}
}

func TestMqttHandleCheckFW(t *testing.T) {
	h := mqttHandler(t)
	f := model.FirmwarePackage{Version: "v2.0.0", Major: 2, Minor: 0, Published: true, SwVersion: (2 << 28)}
	model.DB.Create(&f)
	// 设备 swVer 低 → 有可升级
	low := &CmdFrame{Cmd: CmdCheckFW, SwVer: 0x01000000, CmdSeq: 1}
	h.HandleCheckFW(low, "trafficLight/up/8001/U")
	// 设备已是最新 → 无新版本（mqttClient nil → sendFWCheckAck 直接返回）
	latest := &CmdFrame{Cmd: CmdCheckFW, SwVer: (2 << 28), CmdSeq: 2}
	h.HandleCheckFW(latest, "trafficLight/up/8001/U")
	// 无固件 → 无新版本
	model.DB.Where("1=1").Delete(&model.FirmwarePackage{})
	h.HandleCheckFW(&CmdFrame{Cmd: CmdCheckFW, SwVer: 0, CmdSeq: 3}, "trafficLight/up/8001/U")
}

func TestMqttHandleGetFW(t *testing.T) {
	h := mqttHandler(t)
	f := model.FirmwarePackage{Version: "v3.0.0", Major: 3, Minor: 0, Published: true, SwVersion: (3 << 28)}
	model.DB.Create(&f)
	h.HandleGetFW(&CmdFrame{Cmd: CmdGetFW, SwVer: 0, CmdSeq: 1}, "trafficLight/up/8001/U")
	// 无固件分支
	model.DB.Where("1=1").Delete(&model.FirmwarePackage{})
	h.HandleGetFW(&CmdFrame{Cmd: CmdGetFW, SwVer: 0, CmdSeq: 2}, "trafficLight/up/8001/U")
}

func TestMqttHandleMessage_InvalidPayload(t *testing.T) {
	h := mqttHandler(t)
	// 非法短包 → 记录无效报文
	h.HandleMessage(nil, msgStub("trafficLight/up/8001/U", []byte{0x01, 0x02}))
	// 校验和错误长包 → 无效报文
	bad := BuildCmdFrame(CmdCheckin, 0, 1, 0, nil)
	bad[0] = 0x00 // 破坏 token
	h.HandleMessage(nil, msgStub("trafficLight/up/8001/U", bad))
}

func TestMqttHelpers(t *testing.T) {
	// buildParsedResult
	h := mqttHandler(t)
	f := &CmdFrame{Cmd: CmdCheckin, SwVer: 1, CmdSeq: 2, DatLen: 0, UserVal: 5}
	j1 := h.buildParsedResult(f, nil)
	if j1 == "" {
		t.Error("buildParsedResult nil event 空")
	}
	ep := &EventPak{Records: []EventRecord{{LedHwID: 1, ErrCode: -1}}}
	j2 := h.buildParsedResult(f, ep)
	if j2 == "" {
		t.Error("buildParsedResult with event 空")
	}
	// buildDownTopic
	if d := buildDownTopic("trafficLight/up/8001/U", 7); d != "trafficLight/up/8001/D" {
		t.Errorf("buildDownTopic=%q", d)
	}
	if d := buildDownTopic("/U", 9); d == "/D" {
		t.Errorf("buildDownTopic 异常分支: %q", d)
	}
	// swVer 版本提取
	if swVerMajor(0x34000000) != 3 || swVerMinor(0x04000000) != 4 {
		t.Error("swVer 位域提取错误")
	}
	// topicHwID（替代原恒返回 0 的 frameHwID）：从 Topic 提取硬件 ID，非法返回 0
	if topicHwID("trafficLight/up/8001/1001/U") != 1001 {
		t.Error("topicHwID 应从 Topic 提取硬件 ID")
	}
	if topicHwID("bad-topic") != 0 {
		t.Error("topicHwID 非法 Topic 应返回 0")
	}
	if topicHwID("trafficLight/up/8001/notnum/U") != 0 {
		t.Error("topicHwID 非数字硬件 ID 应返回 0")
	}
	// HandleCheckin/PowerOn 直接调用（regenlog 不 panic）
	h.HandleCheckin(&CmdFrame{Cmd: CmdCheckin, SwVer: 1, CmdSeq: 1}, nil, "trafficLight/up/8001/U")
	h.HandlePowerOn(&CmdFrame{Cmd: CmdPowerOn, SwVer: 1, CmdSeq: 1}, nil, "trafficLight/up/8001/U")
	// HandleAlarm 带记录但无 DB 事件
	model.DB.Create(&model.Device{HwID: 8009})
	rec := EventRecord{LedHwID: 8009, ErrCode: -4}
	h.HandleAlarm(&CmdFrame{Cmd: CmdAlarm, CmdSeq: 3}, &EventPak{Records: []EventRecord{rec}})
	// upsertDevice nil-DB 安全
	oldDB := model.DB
	model.DB = nil
	h.upsertDevice(rec, time.Now())
	h.processFault(&rec)
	h.createWorkOrder(&model.FaultRecord{ID: 1, DeviceHwID: 1})
	h.logPacket(1, nil, 0, 0, "", true)
	model.DB = oldDB
}

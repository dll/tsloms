package mqtt

import (
	"encoding/binary"
	"errors"
	"sync"
)

// errNoActiveHandler 未注册活跃消息处理器（服务未启动 MQTT 处理器）
var errNoActiveHandler = errors.New("active mqtt handler not registered")

// ---------------------------------------------------------------------------
// 检测器接入：数据模拟 / CSV 回放 / 状态上报
// ---------------------------------------------------------------------------
// 设计：Mock 与 CSV 均构造合法协议帧后，复用真实 MQTT 消息处理链路
// （HandleMessage），从而获得与真实硬件完全一致的解析→研判→故障→工单流水，
// 且不依赖真实 Broker 在线（本地直接分派，便于无硬件联调）。
// 真实硬件接入则走既有 MQTT 订阅（trafficLight/+/+/+/U）不变。

var (
	activeHandlerMu sync.RWMutex
	activeHandler   *Handler
	simTopicPrefix  = "trafficLight"
	simNetCode      = "0"
	simStationCode  = "0"
)

// RegisterActiveHandler 注册当前运行的消息处理器（由 cmd/server 启动期调用一次）。
// 供数据模拟/CSV 回放/状态上报复用真实研判链路。
func RegisterActiveHandler(h *Handler) {
	activeHandlerMu.Lock()
	defer activeHandlerMu.Unlock()
	activeHandler = h
}

// ActiveHandler 获取已注册的活跃处理器（可能为 nil，表示未注册）。
func ActiveHandler() *Handler {
	activeHandlerMu.RLock()
	defer activeHandlerMu.RUnlock()
	return activeHandler
}

// SetSimTopic 设置模拟/回放使用的上行 Topic 参数（网络号/站点号/前缀）。
func SetSimTopic(prefix, netCode, stationCode string) {
	simTopicPrefix = prefix
	simNetCode = netCode
	simStationCode = stationCode
}

// SubscribeTopic 返回真实设备订阅模式（{prefix}/+/+/+/U）。
func SubscribeTopic() string {
	return simTopicPrefix + "/+/+/+/U"
}

// SimTrace 是一次模拟/回放对固定一条 EVENT_RECORD 的说明。
type SimTrace struct {
	Topic    string `json:"topic"`
	Cmd      string `json:"cmd"`
	HwID     uint32 `json:"hw_id"`
	ErrCode  int8   `json:"err_code"`
	LedState int8   `json:"led_state"`
}

// BuildCheckinPayload 构造一条 CHECKIN 上行帧（含 1 条 EVENT_RECORD）。
// 参数：hwID、cmdSeq、errCode、ledState、currentR/Y/G。
// 返回完整 CMD_FRAME（token=0x55、校验和=0xFF）。
func BuildCheckinPayload(hwID uint32, cmdSeq uint16, errCode, ledState int8, curR, curY, curG uint16) []byte {
	rec := buildEventRecord(hwID, errCode, ledState, curR, curY, curG)
	eventPak := buildEventPak([]EventRecord{rec})
	return BuildCmdFrame(CmdCheckin, 0x01020304, cmdSeq, 0, eventPak)
}

// BuildAlarmPayload 构造一条 ALARM 上行帧（含 1 条 EVENT_RECORD）。
func BuildAlarmPayload(hwID uint32, cmdSeq uint16, errCode, ledState int8, curR, curY, curG uint16) []byte {
	rec := buildEventRecord(hwID, errCode, ledState, curR, curY, curG)
	eventPak := buildEventPak([]EventRecord{rec})
	return BuildCmdFrame(CmdAlarm, 0x01020304, cmdSeq, 0, eventPak)
}

// BuildPowerOnPayload 构造一条上电报告帧（无事件数据）。
func BuildPowerOnPayload(hwID uint32, cmdSeq uint16) []byte {
	_ = hwID // 上电帧事件数据为空，Topic 承载 hwId
	return BuildCmdFrame(CmdPowerOn, 0x01020304, cmdSeq, 0, nil)
}

// buildEventRecord 构造一条 EVENT_RECORD（24 字节，含 ledState + errCode + current[3]）。
func buildEventRecord(hwID uint32, errCode, ledState int8, curR, curY, curG uint16) EventRecord {
	return EventRecord{
		LedHwID:  hwID,
		SubHwID:  0,
		SwVer:    0x01020304,
		ConfVer:  0x26080101,
		LedState: ledState,
		ErrCode:  errCode,
		CurrentR: curR,
		CurrentY: curY,
		CurrentG: curG,
	}
}

// buildEventPak 构造 EVENT_PAK（eventRecordNum + datLen + records）。
func buildEventPak(records []EventRecord) []byte {
	recBytes := marshalEventRecords(records)
	buf := make([]byte, 0, 4+len(recBytes))
	_tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(_tmp, uint16(len(records)))
	buf = append(buf, _tmp...)
	binary.LittleEndian.PutUint16(_tmp, uint16(len(recBytes)))
	buf = append(buf, _tmp...)
	buf = append(buf, recBytes...)
	return buf
}

// marshalEventRecords 序列化 EVENT_RECORD 列表（与 parser 的 ParseEventPak 对称）。
func marshalEventRecords(records []EventRecord) []byte {
	out := make([]byte, 0, 24*len(records))
	for _, r := range records {
		chunk := make([]byte, 24)
		binary.LittleEndian.PutUint32(chunk[0:4], r.LedHwID)
		binary.LittleEndian.PutUint32(chunk[4:8], r.SubHwID)
		binary.LittleEndian.PutUint32(chunk[8:12], r.SwVer)
		binary.LittleEndian.PutUint32(chunk[12:16], r.ConfVer)
		chunk[16] = uint8(r.LedState)
		chunk[17] = uint8(int8(r.ErrCode))
		binary.LittleEndian.PutUint16(chunk[18:20], r.CurrentR)
		binary.LittleEndian.PutUint16(chunk[20:22], r.CurrentY)
		binary.LittleEndian.PutUint16(chunk[22:24], r.CurrentG)
		out = append(out, chunk...)
	}
	return out
}

// DispatchFrame 将一帧 payload 作为上行消息投递给活跃处理器（复用真实链路）。
// 通过构造 paho MQTT.Message 走 handler.HandleMessage（解析→研判→故障→工单）。
func DispatchFrame(hwID uint32, cmd uint8, payload []byte) (SimTrace, error) {
	h := ActiveHandler()
	if h == nil {
		return SimTrace{}, errNoActiveHandler
	}
	topic := simTopicPrefix + "/" + simNetCode + "/" + simStationCode + "/" + itoa(hwID) + "/U"
	msg := &simMessage{topic: topic, payload: payload}
	h.HandleMessage(nil, msg)
	cmdName := "unknown"
	switch cmd {
	case CmdCheckin:
		cmdName = "checkin"
	case CmdAlarm:
		cmdName = "alarm"
	case CmdPowerOn:
		cmdName = "power_on"
	}
	return SimTrace{Topic: topic, Cmd: cmdName, HwID: hwID}, nil
}

// simMessage 实现 paho MQTT.Message 的最小接口。
type simMessage struct {
	topic   string
	payload []byte
}

func (m *simMessage) Duplicate() bool   { return false }
func (m *simMessage) Qos() byte         { return 1 }
func (m *simMessage) Retained() bool    { return false }
func (m *simMessage) Topic() string     { return m.topic }
func (m *simMessage) MessageID() uint16 { return 0 }
func (m *simMessage) Payload() []byte   { return m.payload }
func (m *simMessage) Ack()              {}
func (m *simMessage) String() string    { return m.topic }

// itoa 简易无符号整数转十进制字符串。
func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

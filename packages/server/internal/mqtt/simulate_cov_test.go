package mqtt

import (
	"strings"
	"sync"
	"testing"
)

// 覆盖 simulate.go 中未命中分支：RegisterActiveHandler / ActiveHandler /
// SetSimTopic / SubscribeTopic / DispatchFrame / simMessage 接口 / itoa。

func TestSimulate_RegisterAndActiveHandler(t *testing.T) {
	old := activeHandler
	oldPrefix, oldNet, oldStation := simTopicPrefix, simNetCode, simStationCode
	t.Cleanup(func() {
		activeHandler = old
		simTopicPrefix, simNetCode, simStationCode = oldPrefix, oldNet, oldStation
	})

	// ActiveHandler 未注册时返回 nil
	if h := ActiveHandler(); h != nil {
		t.Fatalf("未注册时 ActiveHandler 应返回 nil，got %v", h)
	}

	// 注册一个空 Handler（不触发真实链路），ActiveHandler 应取到
	RegisterActiveHandler(&Handler{})
	if ActiveHandler() == nil {
		t.Fatal("注册后 ActiveHandler 不应为 nil")
	}

	// 并发安全读写（Race 检测）
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := ActiveHandler()
			if h != nil {
				RegisterActiveHandler(h)
			}
		}()
	}
	wg.Wait()
}

func TestSimulate_SetSimTopicAndSubscribe(t *testing.T) {
	oldPrefix, oldNet, oldStation := simTopicPrefix, simNetCode, simStationCode
	t.Cleanup(func() {
		simTopicPrefix, simNetCode, simStationCode = oldPrefix, oldNet, oldStation
	})

	// 默认
	SetSimTopic("trafficLight", "0", "0")
	if got := SubscribeTopic(); got != "trafficLight/+/+/+/U" {
		t.Errorf("SubscribeTopic = %q", got)
	}

	// 自定义
	SetSimTopic("tl", "3", "7")
	if got := SubscribeTopic(); got != "tl/+/+/+/U" {
		t.Errorf("SubscribeTopic = %q", got)
	}
}

func TestSimulate_DispatchFrameNoHandler(t *testing.T) {
	old := activeHandler
	t.Cleanup(func() { activeHandler = old })
	activeHandler = nil

	// 未注册活跃处理器应返回 errNoActiveHandler
	_, err := DispatchFrame(1, CmdCheckin, []byte{0x55})
	if err == nil {
		t.Fatal("未注册 handler 时 DispatchFrame 应返回错误")
	}
	if err != errNoActiveHandler {
		t.Fatalf("错误应为 errNoActiveHandler，got %v", err)
	}
}

func TestSimulate_itoa(t *testing.T) {
	cases := []struct {
		in   uint32
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{1001, "1001"},
		{4294967295, "4294967295"},
	}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSimulate_simMessageInterface(t *testing.T) {
	msg := &simMessage{topic: "trafficLight/0/0/1/U", payload: []byte{0x01, 0x02}}
	if msg.Duplicate() {
		t.Error("Duplicate 应为 false")
	}
	if msg.Qos() != 1 {
		t.Errorf("Qos = %d, want 1", msg.Qos())
	}
	if msg.Retained() {
		t.Error("Retained 应为 false")
	}
	if msg.Topic() != "trafficLight/0/0/1/U" {
		t.Errorf("Topic = %q", msg.Topic())
	}
	if msg.MessageID() != 0 {
		t.Errorf("MessageID = %d, want 0", msg.MessageID())
	}
	if len(msg.Payload()) != 2 {
		t.Errorf("Payload len = %d, want 2", len(msg.Payload()))
	}
	msg.Ack() // 不应 panic
	if !strings.HasSuffix(msg.String(), "/1/U") {
		t.Errorf("String = %q", msg.String())
	}
}

// 构建函数：Build*Payload 系列走一次（覆盖 buildEventRecord/buildEventPak/marshal 等）
func TestSimulate_BuildPayloadHelpers(t *testing.T) {
	rec := EventRecord{LedHwID: 7, SubHwID: 0, SwVer: 0x01020304, ConfVer: 0x26080101,
		LedState: StateG, ErrCode: 0, CurrentR: 800, CurrentY: 700, CurrentG: 600}
	pak := buildEventPak([]EventRecord{rec})
	if len(pak) == 0 {
		t.Fatal("buildEventPak 返回空")
	}
	recs := marshalEventRecords([]EventRecord{rec})
	if len(recs) != EventRecordLen {
		t.Errorf("marshalEventRecords len = %d, want %d", len(recs), EventRecordLen)
	}

	ci := BuildCheckinPayload(7, 1, 0, StateG, 800, 700, 600)
	if len(ci) == 0 {
		t.Fatal("BuildCheckinPayload empty")
	}
	al := BuildAlarmPayload(7, 2, 1, StateR, 800, 700, 600)
	if len(al) == 0 {
		t.Fatal("BuildAlarmPayload empty")
	}
	po := BuildPowerOnPayload(7, 3)
	if len(po) == 0 {
		t.Fatal("BuildPowerOnPayload empty")
	}
}

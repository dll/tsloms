package mqtt

import (
	"encoding/binary"
	"testing"
)

// buildFrame 构造一个合法的命令帧（自动计算校验和）
func buildFrame(t *testing.T, cmd uint8, swVer uint32, cmdSeq uint16, userVal uint32, data []byte) []byte {
	t.Helper()
	frame := BuildCmdFrame(cmd, swVer, cmdSeq, userVal, data)
	// 完整性自检：校验和必须为 0xFF
	var sum uint16
	for _, b := range frame {
		sum += uint16(b)
	}
	if uint8(sum) != CmdChecksumValid {
		t.Fatalf("internal: BuildCmdFrame 校验和错误 %02X", uint8(sum))
	}
	return frame
}

// buildEventPakData 构造 EVENT_PAK 数据部分
func buildEventPakData(t *testing.T, records []EventRecord) []byte {
	t.Helper()
	data := make([]byte, EventPakHeaderLen+len(records)*EventRecordLen)
	binary.LittleEndian.PutUint16(data[0:2], uint16(len(records)))
	binary.LittleEndian.PutUint16(data[2:4], uint16(len(records)*EventRecordLen))
	for i, rec := range records {
		off := EventPakHeaderLen + i*EventRecordLen
		binary.LittleEndian.PutUint32(data[off:off+4], rec.LedHwID)
		binary.LittleEndian.PutUint32(data[off+4:off+8], rec.SubHwID)
		binary.LittleEndian.PutUint32(data[off+8:off+12], rec.SwVer)
		binary.LittleEndian.PutUint32(data[off+12:off+16], rec.ConfVer)
		data[off+16] = uint8(rec.LedState)
		data[off+17] = uint8(rec.ErrCode)
		binary.LittleEndian.PutUint16(data[off+18:off+20], rec.CurrentR)
		binary.LittleEndian.PutUint16(data[off+20:off+22], rec.CurrentY)
		binary.LittleEndian.PutUint16(data[off+22:off+24], rec.CurrentG)
	}
	return data
}

func TestParseCmdFrame_ValidCheckin(t *testing.T) {
	rec := EventRecord{
		LedHwID: 1001, SubHwID: 1, SwVer: 0x01020304, ConfVer: 0x26081401,
		LedState: StateR, ErrCode: LEDErrOK, CurrentR: 800, CurrentY: 700, CurrentG: 600,
	}
	data := buildEventPakData(t, []EventRecord{rec})
	frame := buildFrame(t, CmdCheckin, 0x01020304, 1, 0, data)

	parsed, err := ParseCmdFrame(frame)
	if err != nil {
		t.Fatalf("ParseCmdFrame 失败: %v", err)
	}
	if parsed.Token != CmdToken {
		t.Errorf("token = %02X, 期望 %02X", parsed.Token, CmdToken)
	}
	if parsed.Cmd != CmdCheckin {
		t.Errorf("cmd = %02X, 期望 %02X", parsed.Cmd, CmdCheckin)
	}
	if parsed.CmdSeq != 1 {
		t.Errorf("cmdSeq = %d, 期望 1", parsed.CmdSeq)
	}
	if len(parsed.Data) != len(data) {
		t.Errorf("data 长度 = %d, 期望 %d", len(parsed.Data), len(data))
	}
}

func TestParseCmdFrame_ChecksumError(t *testing.T) {
	rec := EventRecord{LedHwID: 1, ErrCode: LEDErrOK, LedState: StateR}
	data := buildEventPakData(t, []EventRecord{rec})
	frame := buildFrame(t, CmdAlarm, 1, 2, 0, data)
	// 篡改一个数据字节，破坏校验和
	frame[len(frame)-1] ^= 0xFF

	if _, err := ParseCmdFrame(frame); err == nil {
		t.Fatal("期望校验和错误，实际解析成功")
	}
}

func TestParseCmdFrame_BadToken(t *testing.T) {
	rec := EventRecord{LedHwID: 1}
	data := buildEventPakData(t, []EventRecord{rec})
	frame := buildFrame(t, CmdCheckin, 1, 1, 0, data)
	frame[0] = 0x54 // 破坏魔术字

	if _, err := ParseCmdFrame(frame); err == nil {
		t.Fatal("期望魔术字错误，实际解析成功")
	}
}

func TestParseCmdFrame_TooShort(t *testing.T) {
	if _, err := ParseCmdFrame([]byte{0x55, 0x01}); err == nil {
		t.Fatal("期望长度不足错误，实际解析成功")
	}
}

func TestParseCmdFrame_DatLenMismatch(t *testing.T) {
	rec := EventRecord{LedHwID: 1}
	data := buildEventPakData(t, []EventRecord{rec})
	frame := buildFrame(t, CmdAlarm, 1, 1, 0, data)
	// 篡改 datLen，使其与实际负载不符（同时破坏校验和，但长度检查先于校验和）
	// 重新计算校验和以让长度检查第一个触发
	frame[10] = 0xFF
	frame[11] = 0xFF
	// 修正校验和
	var sum uint16
	for _, b := range frame[:3] {
		sum += uint16(b)
	}
	for _, b := range frame[4:] {
		sum += uint16(b)
	}
	frame[3] = CmdChecksumValid - uint8(sum)

	if _, err := ParseCmdFrame(frame); err == nil {
		t.Fatal("期望 datLen 不匹配错误，实际解析成功")
	}
}

func TestParseEventPak_Valid(t *testing.T) {
	recs := []EventRecord{
		{LedHwID: 1, LedState: StateG, ErrCode: LEDErrOK, CurrentR: 100, CurrentY: 200, CurrentG: 300},
		{LedHwID: 2, LedState: StateR, ErrCode: LEDErrROFF, CurrentR: 0, CurrentY: 0, CurrentG: 0},
	}
	data := buildEventPakData(t, recs)

	pak, err := ParseEventPak(data)
	if err != nil {
		t.Fatalf("ParseEventPak 失败: %v", err)
	}
	if pak.EventRecordNum != 2 {
		t.Errorf("记录数 = %d, 期望 2", pak.EventRecordNum)
	}
	if len(pak.Records) != 2 {
		t.Fatalf("Records 长度 = %d, 期望 2", len(pak.Records))
	}
	if pak.Records[1].ErrCode != LEDErrROFF {
		t.Errorf("errCode = %d, 期望 %d", pak.Records[1].ErrCode, LEDErrROFF)
	}
	if pak.Records[0].CurrentG != 300 {
		t.Errorf("CurrentG = %d, 期望 300", pak.Records[0].CurrentG)
	}
}

func TestParseEventRecord_LittleEndianWireFormat(t *testing.T) {
	// 使用现场线上的原始字节序列，避免测试构造器与解析器同时改错而无法发现问题。
	data := []byte{
		0x78, 0x56, 0x34, 0x12, // ledHwID = 0x12345678
		0x04, 0x03, 0x02, 0x01, // subHwID = 0x01020304
		0x04, 0x03, 0x02, 0x01, // swVer = 0x01020304
		0x01, 0x08, 0x26, 0x26, // confVer = 0x26260801
		byte(StateY), 0xFF, // LEDErrROFF = -1
		0x34, 0x12, // currentR = 0x1234
		0xCD, 0xAB, // currentY = 0xABCD
		0x00, 0x01, // currentG = 0x0100
	}
	rec, err := ParseEventRecord(data)
	if err != nil {
		t.Fatalf("ParseEventRecord 失败: %v", err)
	}
	if rec.LedHwID != 0x12345678 || rec.SubHwID != 0x01020304 {
		t.Fatalf("硬件 ID 小端解析错误: led=%08X sub=%08X", rec.LedHwID, rec.SubHwID)
	}
	if rec.SwVer != 0x01020304 || rec.ConfVer != 0x26260801 {
		t.Fatalf("版本字段小端解析错误: sw=%08X conf=%08X", rec.SwVer, rec.ConfVer)
	}
	if rec.CurrentR != 0x1234 || rec.CurrentY != 0xABCD || rec.CurrentG != 0x0100 {
		t.Fatalf("电流字段小端解析错误: r=%04X y=%04X g=%04X", rec.CurrentR, rec.CurrentY, rec.CurrentG)
	}
}

func TestBuildCmdFrame_LittleEndianWireFormat(t *testing.T) {
	frame := BuildCmdFrame(CmdCheckin, 0x01020304, 0x1234, 0xA1B2C3D4, nil)
	if got := frame[4:8]; !equalBytes(got, []byte{0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("swVer 未按小端编码: % X", got)
	}
	if got := frame[8:10]; !equalBytes(got, []byte{0x34, 0x12}) {
		t.Fatalf("cmdSeq 未按小端编码: % X", got)
	}
	if got := frame[12:16]; !equalBytes(got, []byte{0xD4, 0xC3, 0xB2, 0xA1}) {
		t.Fatalf("userVal 未按小端编码: % X", got)
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseEventPak_DatLenMismatch(t *testing.T) {
	rec := EventRecord{LedHwID: 1}
	data := buildEventPakData(t, []EventRecord{rec})
	// 篡改 datLen
	binary.LittleEndian.PutUint16(data[2:4], 999)
	if _, err := ParseEventPak(data); err == nil {
		t.Fatal("期望 datLen 不匹配错误，实际解析成功")
	}
}

func TestParseEventPak_TooShort(t *testing.T) {
	if _, err := ParseEventPak([]byte{0x00, 0x01}); err == nil {
		t.Fatal("期望长度不足错误，实际解析成功")
	}
}

func TestFaultTypeFromErrCode(t *testing.T) {
	cases := map[int8]string{
		LEDErrROFF:       "lamp_off",
		LEDErrRYGON:      "abnormal_on",
		LEDErrGONTimeout: "timeout",
		LEDErrGDim:       "dim",
		LEDErrPowerLoss:  "power_loss",
	}
	for code, want := range cases {
		if got := FaultTypeFromErrCode(code); got != want {
			t.Errorf("FaultTypeFromErrCode(%d) = %s, 期望 %s", code, got, want)
		}
	}
}

func TestFaultLevelFromErrCode(t *testing.T) {
	if got := FaultLevelFromErrCode(LEDErrROFF); got != "critical" {
		t.Errorf("灯灭应为 critical, 实际 %s", got)
	}
	if got := FaultLevelFromErrCode(LEDErrGONTimeout); got != "normal" {
		t.Errorf("超时应为 normal, 实际 %s", got)
	}
	if got := FaultLevelFromErrCode(LEDErrPowerLoss); got != "critical" {
		t.Errorf("断电应为 critical, 实际 %s", got)
	}
}

func TestMakeAckCmd(t *testing.T) {
	if cmd := MakeAckCmd(CmdCheckin); cmd != CmdCheckin|CmdAckFlag {
		t.Errorf("ack = %02X, 期望 %02X", cmd, CmdCheckin|CmdAckFlag)
	}
	if !IsAckFrame(MakeAckCmd(CmdCheckin)) {
		t.Error("ack 帧应被识别为回应帧")
	}
}

func TestBuildTimeSyncAck(t *testing.T) {
	epoch := uint32(1787055200)
	frame := BuildTimeSyncAck(CmdCheckin, 0x01020304, 7, epoch)
	// 应标记为回应帧
	if !IsAckFrame(frame[1]) {
		t.Errorf("cmd 应为 ack, got %02X", frame[1])
	}
	// userVal = epoch seconds
	if got := binary.LittleEndian.Uint32(frame[12:16]); got != epoch {
		t.Errorf("userVal = %d, 期望 %d", got, epoch)
	}
	// 校验和正确
	var sum uint16
	for _, b := range frame {
		sum += uint16(b)
	}
	if uint8(sum) != CmdChecksumValid {
		t.Errorf("校验和错误 %02X", uint8(sum))
	}
	// swVer 与 cmdSeq
	if got := binary.LittleEndian.Uint32(frame[4:8]); got != 0x01020304 {
		t.Errorf("swVer = %08X", got)
	}
	if got := binary.LittleEndian.Uint16(frame[8:10]); got != 7 {
		t.Errorf("cmdSeq = %d, 期望 7", got)
	}
	// 无数据部分
	if got := binary.LittleEndian.Uint16(frame[10:12]); got != 0 {
		t.Errorf("datLen 应为 0, got %d", got)
	}
}

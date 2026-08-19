package mqtt

import (
	"encoding/binary"
	"fmt"
)

// CMD_FRAME 固定头部长度（16 字节）
// token(1) + cmd(1) + ver(1) + checksum(1) + swVer(4) + cmdSeq(2) + datLen(2) + userVal(4)
const CmdFrameHeaderLen = 16

// EVENT_PAK 头部长度（4 字节）
// eventRecordNum(2) + datLen(2)
const EventPakHeaderLen = 4

// EVENT_RECORD 固定长度（24 字节）
// ledHwId(4) + subHwId(4) + swVer(4) + confVer(4) + ledState(1) + errCode(1) + current[3](6) = 24
// 注意：C 结构体因 1 字节对齐，current[3] 紧跟 errCode 后，总长 24 字节
const EventRecordLen = 24

// ParseCmdFrame 解析二进制命令帧
// 验证 token=0x55、checksum=0xFF，解析各字段
// 使用小端序解析多字节字段（以现场检测器实际报文为准）
func ParseCmdFrame(data []byte) (*CmdFrame, error) {
	if len(data) < CmdFrameHeaderLen {
		return nil, fmt.Errorf("数据长度不足: 需要 %d 字节, 实际 %d 字节", CmdFrameHeaderLen, len(data))
	}

	frame := &CmdFrame{
		Token:    data[0],
		Cmd:      data[1],
		Ver:      data[2],
		Checksum: data[3],
		SwVer:    binary.LittleEndian.Uint32(data[4:8]),
		CmdSeq:   binary.LittleEndian.Uint16(data[8:10]),
		DatLen:   binary.LittleEndian.Uint16(data[10:12]),
		UserVal:  binary.LittleEndian.Uint32(data[12:16]),
	}

	// 验证魔术字
	if frame.Token != CmdToken {
		return nil, fmt.Errorf("魔术字错误: 期望 0x%02X, 实际 0x%02X", CmdToken, frame.Token)
	}

	// 验证数据长度一致性
	if int(frame.DatLen) != len(data)-CmdFrameHeaderLen {
		return nil, fmt.Errorf("数据长度不匹配: datLen=%d, 实际数据部分=%d", frame.DatLen, len(data)-CmdFrameHeaderLen)
	}

	// 验证校验和：整包所有字节按 uint8 累加必须等于 0xFF
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	if uint8(sum) != CmdChecksumValid {
		return nil, fmt.Errorf("校验和错误: 期望 0x%02X, 实际 0x%02X", CmdChecksumValid, uint8(sum))
	}

	// 提取变长数据部分
	if frame.DatLen > 0 {
		frame.Data = data[CmdFrameHeaderLen:]
	}

	return frame, nil
}

// ParseEventPak 解析事件包
// 从 CMD_FRAME 的 Data 部分解析 EVENT_PAK 结构
func ParseEventPak(data []byte) (*EventPak, error) {
	if len(data) < EventPakHeaderLen {
		return nil, fmt.Errorf("事件包数据长度不足: 需要 %d 字节, 实际 %d 字节", EventPakHeaderLen, len(data))
	}

	pak := &EventPak{
		EventRecordNum: binary.LittleEndian.Uint16(data[0:2]),
		DatLen:         binary.LittleEndian.Uint16(data[2:4]),
	}

	// 验证记录数量与数据长度一致性
	expectedLen := int(pak.EventRecordNum) * EventRecordLen
	if int(pak.DatLen) != expectedLen {
		return nil, fmt.Errorf("事件包数据长度不匹配: datLen=%d, 期望 %d (记录数=%d*%d)",
			pak.DatLen, expectedLen, pak.EventRecordNum, EventRecordLen)
	}

	// 检查实际数据是否足够
	recordData := data[EventPakHeaderLen:]
	if len(recordData) < expectedLen {
		return nil, fmt.Errorf("事件记录数据不足: 需要 %d 字节, 实际 %d 字节", expectedLen, len(recordData))
	}

	// 逐条解析事件记录
	pak.Records = make([]EventRecord, 0, pak.EventRecordNum)
	for i := 0; i < int(pak.EventRecordNum); i++ {
		offset := i * EventRecordLen
		rec, err := ParseEventRecord(recordData[offset : offset+EventRecordLen])
		if err != nil {
			return nil, fmt.Errorf("解析第 %d 条事件记录失败: %w", i+1, err)
		}
		pak.Records = append(pak.Records, *rec)
	}

	return pak, nil
}

// ParseEventRecord 解析单条事件记录
// EVENT_RECORD 结构（24 字节，1 字节对齐）：
// ledHwId(4) + subHwId(4) + swVer(4) + confVer(4) + ledState(1) + errCode(1) + current[3](6)
func ParseEventRecord(data []byte) (*EventRecord, error) {
	if len(data) < EventRecordLen {
		return nil, fmt.Errorf("事件记录数据长度不足: 需要 %d 字节, 实际 %d 字节", EventRecordLen, len(data))
	}

	rec := &EventRecord{
		LedHwID: binary.LittleEndian.Uint32(data[0:4]),
		SubHwID: binary.LittleEndian.Uint32(data[4:8]),
		SwVer:   binary.LittleEndian.Uint32(data[8:12]),
		ConfVer: binary.LittleEndian.Uint32(data[12:16]),
		// ledState 和 errCode 为有符号 int8，直接类型转换
		LedState: int8(data[16]),
		ErrCode:  int8(data[17]),
		// current[3] 为 3 个 uint16，小端序
		CurrentR: binary.LittleEndian.Uint16(data[18:20]),
		CurrentY: binary.LittleEndian.Uint16(data[20:22]),
		CurrentG: binary.LittleEndian.Uint16(data[22:24]),
	}

	return rec, nil
}

// BuildCmdFrame 构造命令帧（用于服务器→设备下发命令）
// 自动计算校验和，返回完整的二进制帧
func BuildCmdFrame(cmd uint8, swVer uint32, cmdSeq uint16, userVal uint32, data []byte) []byte {
	datLen := uint16(len(data))
	// 构造帧：头部 16 字节 + 数据部分
	frame := make([]byte, CmdFrameHeaderLen+int(datLen))

	frame[0] = CmdToken // token
	frame[1] = cmd      // cmd
	frame[2] = CmdVer   // ver
	// checksum 先填 0，最后计算
	binary.LittleEndian.PutUint32(frame[4:8], swVer)
	binary.LittleEndian.PutUint16(frame[8:10], cmdSeq)
	binary.LittleEndian.PutUint16(frame[10:12], datLen)
	binary.LittleEndian.PutUint32(frame[12:16], userVal)

	// 拷贝数据部分
	if datLen > 0 {
		copy(frame[CmdFrameHeaderLen:], data)
	}

	// 计算校验和：整包所有字节 uint8 累加，取低 8 位使其等于 0xFF
	var sum uint16
	for _, b := range frame {
		sum += uint16(b)
	}
	// 调整 checksum 字节使总和为 0xFF
	// 当前总和（checksum 位置为 0）的低 8 位
	currentSum := uint8(sum)
	// 需要补充的值使总和低 8 位 = 0xFF
	frame[3] = CmdChecksumValid - currentSum

	return frame
}

// BuildTimeSyncAck 构造时间同步回应帧
// 用于 CMD_CHECKIN 和 CMD_POWER_ON 的回应，userVal 填充当前 epoch seconds
func BuildTimeSyncAck(cmd uint8, swVer uint32, cmdSeq uint16, epochSeconds uint32) []byte {
	// 标记为回应帧
	ackCmd := MakeAckCmd(cmd)
	// 回应帧无数据部分
	return BuildCmdFrame(ackCmd, swVer, cmdSeq, epochSeconds, nil)
}

package model

import "time"

// PacketLog 报文日志表
// 记录设备上报的原始二进制报文及解析结果，用于审计和故障排查
type PacketLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID   uint32    `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	RawData      []byte    `json:"raw_data" gorm:"type:blob;comment:原始二进制报文"`
	CmdType      uint8     `json:"cmd_type" gorm:"comment:命令类型"`
	CmdSeq       uint16    `json:"cmd_seq" gorm:"comment:包序号"`
	ParsedResult string    `json:"parsed_result" gorm:"type:json;comment:解析结果JSON"`
	Valid        bool      `json:"valid" gorm:"comment:校验是否通过"`
	ReceivedAt   time.Time `json:"received_at" gorm:"comment:接收时间"`
}

// TableName 指定表名
func (PacketLog) TableName() string {
	return "packet_logs"
}

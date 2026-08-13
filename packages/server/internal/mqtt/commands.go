package mqtt

// ==================== CMD_FRAME 帧字段常量 ====================

const (
	// CmdToken 魔术字，固定 0x55
	CmdToken uint8 = 0x55
	// CmdVer 协议版本号，当前 0x10 = v1.0
	CmdVer uint8 = 0x10
	// CmdChecksumValid 校验和目标值，整包所有字节(uint8)相加等于 0xFF
	CmdChecksumValid uint8 = 0xFF
)

// ==================== 命令类型常量（COMMAND） ====================

const (
	// CmdCheckin 定时签到，设备工作正常（设备→服务器）
	CmdCheckin uint8 = 0x00
	// CmdAlarm 故障报警，信号灯异常（设备→服务器）
	CmdAlarm uint8 = 0x01
	// CmdPowerOn 上电/重启完成报告（设备→服务器）
	CmdPowerOn uint8 = 0x03
	// CmdUpdateConfig 下发配置更新（服务器→设备）
	CmdUpdateConfig uint8 = 0x20
	// CmdCheckFW 查询是否有新固件（设备→服务器）
	CmdCheckFW uint8 = 0x30
	// CmdGetFW 请求固件升级数据（设备→服务器）
	CmdGetFW uint8 = 0x31
	// CmdReboot 远程重启设备（服务器→设备）
	CmdReboot uint8 = 0x7F
	// CmdAckFlag 响应标志，bit7=1 表示回应帧
	CmdAckFlag uint8 = 0x80
)

// ==================== 错误码常量（errCode） ====================

const (
	LEDErrOK         int8 = 0   // 工作正常
	LEDErrROFF       int8 = -1  // 红灯周期全灭
	LEDErrYOFF       int8 = -2  // 黄灯周期全灭
	LEDErrGOFF       int8 = -3  // 绿灯周期全灭
	LEDErrRYON       int8 = -4  // 红黄同亮
	LEDErrRGON       int8 = -5  // 红绿同亮
	LEDErrYGON       int8 = -6  // 黄绿同亮
	LEDErrRYGON      int8 = -7  // 红黄绿同亮
	LEDErrRONTimeout int8 = -8  // 红灯亮灯超时
	LEDErrYONTimeout int8 = -9  // 黄灯亮灯超时
	LEDErrGONTimeout int8 = -10 // 绿灯亮灯超时
	LEDErrRDim       int8 = -11 // 红灯缺亮（暂未实现）
	LEDErrYDim       int8 = -12 // 黄灯缺亮（暂未实现）
	LEDErrGDim       int8 = -13 // 绿灯缺亮（暂未实现）
	LEDErrPowerLoss  int8 = -14 // 断电（超过设定时间阈值）
)

// ==================== 灯组状态常量（LED_STATE） ====================

const (
	StateR    int8 = 0  // 红灯状态
	StateY    int8 = 1  // 黄灯状态
	StateG    int8 = 2  // 绿灯状态
	StateNone int8 = -1 // 未知状态（故障无法确定）
)

// ==================== CMD_FRAME 帧结构体 ====================

// CmdFrame 命令帧结构
// 所有设备通信基于统一的二进制命令帧结构
type CmdFrame struct {
	Token    uint8  // 魔术字，固定 0x55
	Cmd      uint8  // 命令类型
	Ver      uint8  // 协议版本号，当前 0x10
	Checksum uint8  // 校验和，整包所有字节(uint8)相加等于 0xFF
	SwVer    uint32 // 设备软件版本号
	CmdSeq   uint16 // 包序号，每发送一个命令序号加一
	DatLen   uint16 // 数据部分长度（字节）
	UserVal  uint32 // 用户自定义数据（用于时间同步等）
	Data     []byte // 变长数据，根据 cmd 类型有不同结构
}

// ==================== EVENT_PAK / EVENT_RECORD 结构体 ====================

// EventPak 事件包结构
// 当 cmd 为 CMD_CHECKIN 或 CMD_ALARM 时，dat 部分按此格式解释
type EventPak struct {
	EventRecordNum uint16        // 事件记录数量
	DatLen         uint16        // eventRecord 部分总长度
	Records        []EventRecord // 变长事件记录数组
}

// EventRecord 事件记录结构
// 包含设备硬件ID、灯组状态、错误码、电流值等核心字段
type EventRecord struct {
	LedHwID  uint32 // 设备硬件 ID（出厂唯一）
	SubHwID  uint32 // 子灯组 ID
	SwVer    uint32 // 固件版本号
	ConfVer  uint32 // 配置版本号（0xYYMMDDnn 格式）
	LedState int8   // 当前灯组亮灯状态
	ErrCode  int8   // 错误码（见 LED_ERR_* 常量）
	CurrentR uint16 // 红灯电流值（0-2048）
	CurrentY uint16 // 黄灯电流值（0-2048）
	CurrentG uint16 // 绿灯电流值（0-2048）
}

// ==================== 故障分类辅助函数 ====================

// FaultTypeFromErrCode 根据错误码返回故障类型分类
func FaultTypeFromErrCode(errCode int8) string {
	switch errCode {
	case LEDErrROFF, LEDErrYOFF, LEDErrGOFF:
		return "lamp_off" // 灯珠灭灯
	case LEDErrRYON, LEDErrRGON, LEDErrYGON, LEDErrRYGON:
		return "abnormal_on" // 异常同亮
	case LEDErrRONTimeout, LEDErrYONTimeout, LEDErrGONTimeout:
		return "timeout" // 亮灯超时
	case LEDErrRDim, LEDErrYDim, LEDErrGDim:
		return "dim" // 缺亮
	case LEDErrPowerLoss:
		return "power_loss" // 断电
	default:
		return "unknown"
	}
}

// FaultLevelFromErrCode 根据错误码返回故障等级
func FaultLevelFromErrCode(errCode int8) string {
	switch errCode {
	case LEDErrROFF, LEDErrYOFF, LEDErrGOFF,
		LEDErrRYON, LEDErrRGON, LEDErrYGON, LEDErrRYGON,
		LEDErrPowerLoss:
		return "critical" // 严重
	case LEDErrRONTimeout, LEDErrYONTimeout, LEDErrGONTimeout,
		LEDErrRDim, LEDErrYDim, LEDErrGDim:
		return "normal" // 一般
	default:
		return "normal"
	}
}

// IsAckFrame 判断是否为回应帧（bit7=1）
func IsAckFrame(cmd uint8) bool {
	return cmd&CmdAckFlag != 0
}

// MakeAckCmd 将命令类型标记为回应帧
func MakeAckCmd(cmd uint8) uint8 {
	return cmd | CmdAckFlag
}

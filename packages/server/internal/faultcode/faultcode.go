// Package faultcode —— 固件故障错误码常量与分类规则基座（范围A）
//
// 从 mqtt 包抽出，供 mqtt（事件解析/落库）与 recognition（研判引擎）共同引用，
// 避免 recognition ↔ mqtt 的循环依赖。语义与既有 mqtt.LED_ERR_* / FaultTypeFromErrCode /
// FaultLevelFromErrCode 完全一致（R9：不改变判定语义）。
package faultcode

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

// FaultTypeFromErrCode 根据错误码返回故障类型分类（语义不变）
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

// FaultLevelFromErrCode 根据错误码返回故障等级（语义不变）
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

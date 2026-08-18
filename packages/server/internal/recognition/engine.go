// Package recognition —— 智能多源故障识别研判引擎（范围A）
//
// 把故障识别从"固件单一 errCode 1:1 直判"升级为多源融合研判：
//   - 第 1 层：确定性规则基座（内聚 mqtt 的 FaultTypeFromErrCode/FaultLevelFromErrCode，不改变语义）
//   - 第 2 层：多源交叉验证与置信度融合（固件为主信号，其它证据加权印证/否证）
//   - 第 3 层：判定与分流（confirmed 直判 / pending_review 待确认不派单 / filtered 误报过滤）
//
// 安全关键约束：**误报过滤绝不丢弃真故障** —— 只有"明确否证"才过滤；
// 证据冲突/孤证一律降级为 pending_review（可被证据补充后升级确认），宁多等证据不漏真故障。
//
// 判定结果通过 model.FaultRecognition 携带，业务层（mqtt.processFault）据此落库。
package recognition

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tsloms/server/internal/faultcode"
	"github.com/tsloms/server/internal/model"
)

// ---------- 判定分流阈值（第 3 层） ----------
const (
	// ConfHigh 高置信阈值：conf ≥ 此值 → 确认故障，critical 自动派单
	ConfHigh = 0.90
	// ConfLow 低置信阈值：conf < 此值 → 明确否证/孤证无支撑 → 误报过滤（仅记证据）
	ConfLow = 0.50
)

// ---------- 确定性规则基座（第 1 层） ----------
//
// 内聚既有 mqtt.FaultTypeFromErrCode / FaultLevelFromErrCode 作为规则基座（R9：不改变语义），
// 在此基础上扩展出"基础置信度"。固件 errCode 是设备主动上报的强信号，基础置信度给高分。

// errCodeBaseConf 每个 errCode 的基础置信度（证据源：固件主动上报，强相关）
var errCodeBaseConf = map[int8]float64{
	faultcode.LEDErrROFF:       0.98,
	faultcode.LEDErrYOFF:       0.98,
	faultcode.LEDErrGOFF:       0.98,
	faultcode.LEDErrRYON:       0.98,
	faultcode.LEDErrRGON:       0.98,
	faultcode.LEDErrYGON:       0.98,
	faultcode.LEDErrRYGON:      0.98,
	faultcode.LEDErrRONTimeout: 0.95,
	faultcode.LEDErrYONTimeout: 0.95,
	faultcode.LEDErrGONTimeout: 0.95,
	faultcode.LEDErrRDim:       0.92,
	faultcode.LEDErrYDim:       0.92,
	faultcode.LEDErrGDim:       0.92,
	faultcode.LEDErrPowerLoss:  0.97,
}

// baseConfidence 返回固件 errCode 的基础置信度；未知错误码降为低置信（存疑，不直接高判）。
func baseConfidence(errCode int8) float64 {
	if c, ok := errCodeBaseConf[errCode]; ok {
		return c
	}
	return 0.4
}

// RuleEvidence 归一化后的单条证据
type RuleEvidence struct {
	DeviceHwID    string    // 设备硬件ID(uuid字符串)
	SourceType    string    // model.EvSource*
	ErrCode       *int8     // firmware 类
	LedState      *int8     // firmware/led_state 类
	CurrentR      *uint16   // firmware/current 类
	CurrentY      *uint16   // firmware/current 类
	CurrentG      *uint16   // firmware/current 类
	RawData       string    // 原始内容
	CapturedAt    time.Time // 证据发生时间
	RefMediaID    *uint     // 举证/监控
	RefFeedbackID *uint     // 群众反映
	Confidence    float64   // 该证据对判定的贡献度 0-1
}

// LedUUID 协议帧设备硬件ID(uint32) → 台账 uuid 字符串（大写十六进制，8位补零）。
// 设计边界：协议层 LedHwID/Evaluator.DeviceHwID 保持 uint32 不变；
// 台账层（devices/fault_records/fault_evidence/fault_case 等）hw_id 为 uuid 字符串。
// AIITSS 设备的十六进制 uuid（如 "1114004B"）恰为该 uint32 的十六进制表示，可无缝对应。
func LedUUID(v uint32) string { return fmt.Sprintf("%08X", v) }

// Evaluator 一次研判的引擎上下文
type Evaluator struct {
	DeviceHwID uint32
	ErrCode    int8
	// 主信号原始值（固件事件记录）
	LedState int8
	CurrentR uint16
	CurrentY uint16
	CurrentG uint16
	// 证据模型（本次调用的主信号 + 注入 / 检索到的辅助证据）归一化后集合
	evidence []RuleEvidence
}

// NewEvaluator 构造研判引擎，主信号为固件 errCode + 电流/灯态
func NewEvaluator(hwID uint32, errCode int8, ledState int8, curR, curY, curG uint16) *Evaluator {
	return &Evaluator{
		DeviceHwID: hwID,
		ErrCode:    errCode,
		LedState:   ledState,
		CurrentR:   curR,
		CurrentY:   curY,
		CurrentG:   curG,
	}
}

// AddEvidence 注入一条辅助证据（群众反映/手机举证/视频监控/电流异常等）
func (e *Evaluator) AddEvidence(ev RuleEvidence) {
	e.evidence = append(e.evidence, ev)
}

// Evidence 返回已注入的辅助证据集合（只读，用于落库/案例摘要）
func (e *Evaluator) Evidence() []RuleEvidence {
	return e.evidence
}

// Validate 执行完整研判：规则基座 → 多源交叉验证 → 判定分流 → 返回研判结果。
// 结果含故障类型/等级/置信度/分流状态/来源/证据数/批次号。
func (e *Evaluator) Validate() model.FaultRecognition {
	evaluationID := NewEvaluationID(e.DeviceHwID)

	// 第 1 层：确定性规则基座（不改变既有判定语义）
	faultType := faultcode.FaultTypeFromErrCode(e.ErrCode)
	faultLevel := faultcode.FaultLevelFromErrCode(e.ErrCode)
	conf := baseConfidence(e.ErrCode)

	// 未知错误码（缺规则映射）：宁待确认也不默认误报，更不高判
	if !knownErrCode(e.ErrCode) {
		return model.FaultRecognition{
			FaultType:         faultType,
			FaultLevel:        faultLevel,
			Confidence:        conf,
			RecognitionStatus: model.RecognitionPendingReview,
			RecognitionSource: model.RecognitionSourceRule,
			EvidenceCount:     1,
			EvaluationID:      evaluationID,
		}
	}

	// 第 2 层：多源交叉验证与置信度融合
	conf = e.crossValidate(conf, faultType)

	// 第 3 层：判定与分流
	status := model.RecognitionConfirmed
	source := model.RecognitionSourceMultiSource
	if len(e.evidence) == 0 {
		source = model.RecognitionSourceRule
	}
	switch {
	case conf >= ConfHigh:
		status = model.RecognitionConfirmed
	case conf < ConfLow:
		// 明确否证 → 误报过滤（仅记证据日志，不产生故障/工单）
		status = model.RecognitionFiltered
	default:
		// 存疑/证据冲突/孤证：待确认，不自动派单；可被证据补充后升级确认
		status = model.RecognitionPendingReview
	}

	return model.FaultRecognition{
		FaultType:         faultType,
		FaultLevel:        faultLevel,
		Confidence:        round3(conf),
		RecognitionStatus: status,
		RecognitionSource: source,
		EvidenceCount:     1 + len(e.evidence),
		EvaluationID:      evaluationID,
	}
}

// crossValidate 多源交叉验证：按辅助证据调整置信度。
// 原则：主信号 errCode 强，辅助证据只做 +/- 微调；冲突时降级而非直接过滤。
func (e *Evaluator) crossValidate(base float64, faultType string) float64 {
	conf := base
	citizenCount := 0
	mediaCount := 0
	checkedCurrent := false
	contradictCurrent := false

	for _, ev := range e.evidence {
		switch ev.SourceType {
		case model.EvSourceCitizen, model.EvSourcePhotoEvidence, model.EvSourceVideoMonitor:
			// 群众/举证/监控辅助证据：印证 → 提升置信度（每人/每媒体一次印证加成，封顶）
			if ev.SourceType == model.EvSourceCitizen {
				citizenCount++
			} else {
				mediaCount++
			}
		case model.EvSourceCurrent:
			// 电流证据：与故障类型交叉校验
			checkedCurrent = true
			if e.currentRefutes(ev, faultType) {
				// 关联灯色电流存在且明确矛盾 ▶ 降级（不直接过滤）
				contradictCurrent = true
			} else if e.currentCorroborates(ev, faultType) {
				conf += 0.03
			}
		case model.EvSourceLedState:
			// 灯态证据：与 errCode 指示的故障灯色相互印证
			if ledCorroborates(ev, e.ErrCode) {
				conf += 0.02
			}
		}
	}

	// 人工/媒体印证加成（辅助证据数量越多越可信，但封顶避免无意义堆叠）
	human := citizenCount + mediaCount
	if human > 0 {
		conf += 0.05 * math.Min(float64(human), 3)
	}

	// 明确电流矛盾（如"灯灭"但电流正常高）→ 显著降级，但仅到"待确认"，不可直接判真/彻底过滤
	if checkedCurrent && contradictCurrent {
		conf -= 0.30
	}

	// 封顶在 [0,1]
	return math.Max(0, math.Min(1, conf))
}

// knownErrCode 是否已有规则映射
func knownErrCode(errCode int8) bool {
	_, ok := errCodeBaseConf[errCode]
	return ok
}

// lampColorOfErr 返回 errCode 指示的故障灯色（r/y/g/none）
func lampColorOfErr(errCode int8) string {
	switch errCode {
	case faultcode.LEDErrROFF, faultcode.LEDErrRONTimeout, faultcode.LEDErrRDim:
		return "r"
	case faultcode.LEDErrYOFF, faultcode.LEDErrYONTimeout, faultcode.LEDErrYDim:
		return "y"
	case faultcode.LEDErrGOFF, faultcode.LEDErrGONTimeout, faultcode.LEDErrGDim:
		return "g"
	default:
		return "none"
	}
}

// currentCorroborates 电流是否印证该故障类型（如 lamp_off 对应灯电流显著偏低）
// 只要求故障关联的灯色电流通道存在即可判断，其它通道可不提供。
func (e *Evaluator) currentCorroborates(ev RuleEvidence, faultType string) bool {
	color := lampColorOfErr(e.ErrCode)
	switch faultType {
	case "lamp_off":
		switch color {
		case "r":
			return ev.CurrentR != nil && float64(*ev.CurrentR) < 50
		case "y":
			return ev.CurrentY != nil && float64(*ev.CurrentY) < 50
		case "g":
			return ev.CurrentG != nil && float64(*ev.CurrentG) < 50
		}
	case "power_loss":
		if !anyCurrent(ev) {
			return false
		}
		return sumCurrent(ev) < 50
	}
	return false
}

// currentRefutes 电流是否否证该故障类型（明显矛盾：灯灭但电流正常高）
// 只要求故障关联的灯色电流通道存在即可判断，其它通道可不提供。
func (e *Evaluator) currentRefutes(ev RuleEvidence, faultType string) bool {
	color := lampColorOfErr(e.ErrCode)
	switch faultType {
	case "lamp_off":
		switch color {
		case "r":
			return ev.CurrentR != nil && float64(*ev.CurrentR) >= 200
		case "y":
			return ev.CurrentY != nil && float64(*ev.CurrentY) >= 200
		case "g":
			return ev.CurrentG != nil && float64(*ev.CurrentG) >= 200
		}
	case "power_loss":
		if !anyCurrent(ev) {
			return false
		}
		return sumCurrent(ev) >= 100
	}
	return false
}

// sumCurrent 对已提供的电流通道求和；缺失（nil）的通道按 0 计，避免单通道证据解引用 panic
func sumCurrent(ev RuleEvidence) float64 {
	var sum float64
	if ev.CurrentR != nil {
		sum += float64(*ev.CurrentR)
	}
	if ev.CurrentY != nil {
		sum += float64(*ev.CurrentY)
	}
	if ev.CurrentG != nil {
		sum += float64(*ev.CurrentG)
	}
	return sum
}

// anyCurrent 是否提供了任一电流通道
func anyCurrent(ev RuleEvidence) bool {
	return ev.CurrentR != nil || ev.CurrentY != nil || ev.CurrentG != nil
}

// ledCorroborates 灯态是否印证 errCode（如灯灭故障时灯态处于该色）
func ledCorroborates(ev RuleEvidence, errCode int8) bool {
	if ev.LedState == nil {
		return false
	}
	color := lampColorOfErr(errCode)
	if color == "none" {
		return true // 同亮/断电类灯态难单独判断，视为中性佐证
	}
	want := map[string]int8{"r": faultcode.StateR, "y": faultcode.StateY, "g": faultcode.StateG}[color]
	return *ev.LedState == want
}

// NewEvaluationID 生成研判批次号（设备 + 时间戳的短哈希，保证当批可溯源）
func NewEvaluationID(hwID uint32) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", hwID, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:4]) // 8 位 hex 足够批次区分
}

// round3 保留 3 位小数
func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}

// BuildSignature 生成证据特征指纹（案例库检索/去重用）
// 主信号 errCode + 电流 + 灯态 + 辅助证据来源类型有序串。
func BuildSignature(e *Evaluator) string {
	srcs := make([]string, 0, len(e.evidence))
	for _, ev := range e.evidence {
		srcs = append(srcs, ev.SourceType)
	}
	sortStr := strings.Join(srcs, ",")
	return fmt.Sprintf("hw%d:err%d:r%d:y%d:g%d:ls%d:aux[%s]",
		e.DeviceHwID, e.ErrCode, e.CurrentR, e.CurrentY, e.CurrentG, e.LedState, sortStr)
}

// EvidenceToModel 把归一化证据转为持久化 fault_evidence 记录
func EvidenceToModel(ev RuleEvidence, faultID *uint, evaluationID string) model.FaultEvidence {
	return model.FaultEvidence{
		FaultID:       faultID,
		EvaluationID:  evaluationID,
		DeviceHwID:    ev.DeviceHwID,
		SourceType:    ev.SourceType,
		ErrCode:       ev.ErrCode,
		LedState:      ev.LedState,
		CurrentR:      ev.CurrentR,
		CurrentY:      ev.CurrentY,
		CurrentG:      ev.CurrentG,
		RawData:       ev.RawData,
		RefMediaID:    ev.RefMediaID,
		RefFeedbackID: ev.RefFeedbackID,
		CapturedAt:    ev.CapturedAt,
		Confidence:    round3(ev.Confidence),
	}
}

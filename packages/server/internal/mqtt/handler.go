package mqtt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/tsloms/server/internal/caselib"
	"github.com/tsloms/server/internal/logger"
	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/recognition"
	"go.uber.org/zap"
)

// Handler MQTT 消息处理器
// 负责接收设备消息、解析二进制协议、校验、分发到对应命令处理器
type Handler struct {
	logger     *zap.Logger
	mqttClient *MQTTClient
}

// NewHandler 创建消息处理器实例
func NewHandler(mc *MQTTClient) *Handler {
	return &Handler{
		logger:     logger.Get(),
		mqttClient: mc,
	}
}

// HandleMessage MQTT 消息入口处理函数
// 解析 CMD_FRAME -> 校验 -> 分发到对应命令处理器
func (h *Handler) HandleMessage(client MQTT.Client, msg MQTT.Message) {
	payload := msg.Payload()
	topic := msg.Topic()

	// 记录原始报文日志
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("消息处理 panic",
				zap.String("topic", topic),
				zap.Any("panic", r),
			)
		}
	}()

	// 解析命令帧
	frame, err := ParseCmdFrame(payload)
	if err != nil {
		h.logger.Warn("命令帧解析失败",
			zap.String("topic", topic),
			zap.Error(err),
		)
		// 记录无效报文日志
		h.logPacket("", payload, 0, 0, "", false)
		return
	}

	// 解析事件包（签到和告警命令携带事件数据）
	var eventPak *EventPak
	if frame.DatLen > 0 && (frame.Cmd == CmdCheckin || frame.Cmd == CmdAlarm || frame.Cmd == CmdPowerOn) {
		eventPak, err = ParseEventPak(frame.Data)
		if err != nil {
			h.logger.Warn("事件包解析失败",
				zap.String("topic", topic),
				zap.Uint8("cmd", frame.Cmd),
				zap.Error(err),
			)
		}
	}

	// 构造解析结果 JSON
	parsedResult := h.buildParsedResult(frame, eventPak)

	// 提取设备硬件 ID（从第一条事件记录，或 0 表示无事件数据）
	var deviceHwID string
	if eventPak != nil && len(eventPak.Records) > 0 {
		deviceHwID = recognition.LedUUID(eventPak.Records[0].LedHwID)
	}

	// 记录报文日志
	h.logPacket(deviceHwID, payload, frame.Cmd, frame.CmdSeq, parsedResult, true)

	// 根据命令类型分发到对应处理器
	switch frame.Cmd {
	case CmdCheckin:
		h.HandleCheckin(frame, eventPak, topic)
	case CmdAlarm:
		h.HandleAlarm(frame, eventPak)
	case CmdPowerOn:
		h.HandlePowerOn(frame, eventPak, topic)
	case CmdCheckFW:
		h.HandleCheckFW(frame, topic)
	case CmdGetFW:
		h.HandleGetFW(frame, topic)
	default:
		h.logger.Warn("未知命令类型",
			zap.String("topic", topic),
			zap.Uint8("cmd", frame.Cmd),
		)
	}
}

// HandleCheckin 处理设备签到
// 1. 更新设备在线状态和签到时间
// 2. 检查事件记录中是否有故障，有则触发故障研判
// 3. 返回时间同步回应
func (h *Handler) HandleCheckin(frame *CmdFrame, eventPak *EventPak, uplinkTopic string) {
	now := time.Now()

	// 遍历事件记录，更新设备信息并检查故障
	if eventPak != nil {
		// 同帧内同一硬件ID只对末条记录做一次 upsert（B3：减少热路径重复查询往返；
		// 设备版本字段采用末条记录值，与原逐条覆盖语义一致）
		for _, rec := range lastRecords(eventPak.Records) {
			h.upsertDevice(rec, now)
		}
		for _, rec := range eventPak.Records {
			// 签到中如果包含故障记录，也进行故障研判（逐条执行，语义不变）
			if rec.ErrCode != LEDErrOK {
				h.processFault(&rec)
			}
		}
	}

	// 发送时间同步回应
	h.sendTimeSyncAck(frame, uplinkTopic)

	h.logger.Info("设备签到处理完成",
		zap.Uint32("swVer", frame.SwVer),
		zap.Uint16("cmdSeq", frame.CmdSeq),
	)
}

// HandleAlarm 处理设备告警
// 1. 解析故障事件记录
// 2. 执行故障研判与去重
// 3. 自动生成维修工单
func (h *Handler) HandleAlarm(frame *CmdFrame, eventPak *EventPak) {
	if eventPak == nil || len(eventPak.Records) == 0 {
		h.logger.Warn("告警消息无事件记录",
			zap.Uint16("cmdSeq", frame.CmdSeq),
		)
		return
	}

	// 更新设备信息（同帧同硬件ID取末条记录合并为一次 upsert，减少重复查询；设备版本字段取末条值）
	for _, rec := range lastRecords(eventPak.Records) {
		h.upsertDevice(rec, time.Now())
	}
	for _, rec := range eventPak.Records {
		// 故障研判与工单生成（逐条执行，语义不变）
		h.processFault(&rec)
	}

	h.logger.Info("设备告警处理完成",
		zap.Uint16("cmdSeq", frame.CmdSeq),
		zap.Int("recordCount", len(eventPak.Records)),
	)
}

// HandlePowerOn 处理设备上电报告
// 1. 更新设备信息与在线状态
// 2. 返回时间同步回应
func (h *Handler) HandlePowerOn(frame *CmdFrame, eventPak *EventPak, uplinkTopic string) {
	now := time.Now()

	if eventPak != nil {
		// 同帧同硬件ID取末条记录做一次 upsert（B3：减少重复查询；设备版本字段取末条值）
		for _, rec := range lastRecords(eventPak.Records) {
			h.upsertDevice(rec, now)
		}
	}

	// 发送时间同步回应
	h.sendTimeSyncAck(frame, uplinkTopic)

	h.logger.Info("设备上电报告处理完成",
		zap.Uint32("swVer", frame.SwVer),
		zap.Uint16("cmdSeq", frame.CmdSeq),
	)
}

// lastRecords 返回每个硬件ID（hwID）在 Records 中最后一次出现的记录。
// 用于 B3 同帧设备 upsert 合并：同一 hwID 一帧内只 upsert 一次，且取值与原逐条覆盖语义一致（末条生效）。
func lastRecords(records []EventRecord) []EventRecord {
	// 记录每个 hwID 最后一次出现的索引
	lastIdx := make(map[uint32]int, len(records))
	for i := range records {
		lastIdx[records[i].LedHwID] = i
	}
	out := make([]EventRecord, 0, len(lastIdx))
	for _, i := range lastIdx {
		out = append(out, records[i])
	}
	return out
}

// upsertDevice 新增或更新设备信息
// 根据 ledHwId 查找设备，存在则更新版本号和在线状态，不存在则创建
func (h *Handler) upsertDevice(rec EventRecord, checkinTime time.Time) {
	if model.DB == nil {
		return
	}

	var device model.Device
	result := model.DB.Where("hw_id = ?", recognition.LedUUID(rec.LedHwID)).First(&device)

	if result.Error != nil {
		// 设备不存在，创建新设备
		device = model.Device{
			HwID:          recognition.LedUUID(rec.LedHwID),
			SwVersion:     rec.SwVer,
			ConfVersion:   rec.ConfVer,
			OnlineStatus:  true,
			LastCheckinAt: &checkinTime,
		}
		if err := model.DB.Create(&device).Error; err != nil {
			h.logger.Error("创建设备记录失败",
				zap.Uint32("hwId", rec.LedHwID),
				zap.Error(err),
			)
		}
	} else {
		// 设备已存在，更新信息
		updates := map[string]interface{}{
			"sw_version":      rec.SwVer,
			"conf_version":    rec.ConfVer,
			"online_status":   true,
			"last_checkin_at": checkinTime,
		}
		if err := model.DB.Model(&device).Updates(updates).Error; err != nil {
			h.logger.Error("更新设备记录失败",
				zap.Uint32("hwId", rec.LedHwID),
				zap.Error(err),
			)
		}
	}
}

// processFault 故障研判与去重（智能多源故障识别研判引擎已接入，范围A）
// 同一设备同一 errCode 在 30 分钟内只生成一条故障记录，后续更新 lastSeen（R3）。
// 研判链路：
//
//	· 确定性规则基座（内聚 FaultTypeFromErrCode/FaultLevelFromErrCode）→ 多源交叉验证 → 判定分流；
//	· confirmed：按原逻辑落库，critical 自动工单（R6），并写 confidence/证据/案例；
//	· pending_review：低置信/存疑 → 落入故障但【不自动派单】，可被证据补充后升级确认；
//	· filtered：明确否证 → 误报过滤，仅记证据与案例，不产生故障/工单。
//
// 红线保持：R2 状态机、R3 去重窗口、R6 自动工单、R9 兼容。
func (h *Handler) processFault(rec *EventRecord) {
	if model.DB == nil {
		return
	}

	now := time.Now()
	dedupWindow := 30 * time.Minute

	// 智能研判：多源证据采集/归一化 → 规则判定 → 置信度融合 → 分流
	eval := recognition.NewEvaluator(rec.LedHwID, rec.ErrCode, rec.LedState, rec.CurrentR, rec.CurrentY, rec.CurrentG)
	// 主信号本身即含 errCode/电流/灯态；可检索注入辅助证据（群众反映/举证/监控）
	injectAuxEvidence(eval, rec.LedHwID, now)
	judge := eval.Validate()

	// 误报过滤：明确否证，仅记证据与案例，不产生故障/工单（绝不把真故障当误报丢弃）
	if judge.RecognitionStatus == model.RecognitionFiltered {
		h.persistEvidence(eval, nil, judge, rec, now)
		h.persistCase(eval, judge)
		h.logger.Info("故障研判(误报过滤)",
			zap.Uint32("hwId", rec.LedHwID),
			zap.Int8("errCode", rec.ErrCode),
			zap.Float64("confidence", judge.Confidence),
		)
		return
	}

	// 查找同一设备同一错误码的活跃故障记录
	var existing model.FaultRecord
	result := model.DB.Where(
		"device_hw_id = ? AND err_code = ? AND status IN ?",
		recognition.LedUUID(rec.LedHwID), rec.ErrCode,
		[]string{model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched},
	).First(&existing)

	if result.Error == nil {
		// 故障已存在，检查是否在去重窗口内
		if now.Sub(existing.LastSeen) <= dedupWindow {
			// 在去重窗口内：先尝试待确认升级（M2），再更新 lastSeen。
			// M2 自动升级：若 existing 为待确认(pending_review)且尚未派单，本次上报证据使其达到高置信
			// (judge==confirmed)，则升级为确认；若为 critical 则自动派单（复用 M1 原子防重，只建一条）。
			// 绝不把已 confirmed/已派单/超窗 resolved 的故障误降级或重复派单。
			dispatchIf := false
			if existing.RecognitionStatus == model.RecognitionPendingReview &&
				existing.WorkOrderID == nil &&
				judge.RecognitionStatus == model.RecognitionConfirmed {
				updGrade := map[string]interface{}{
					"recognition_status": model.RecognitionConfirmed,
					"confidence":         judge.Confidence,
					"recognition_source": judge.RecognitionSource,
					"evidence_count":     judge.EvidenceCount,
					"last_evaluation_id": judge.EvaluationID,
					"last_seen":          now,
				}
				if existing.CurrentR != rec.CurrentR || existing.CurrentY != rec.CurrentY ||
					existing.CurrentG != rec.CurrentG || existing.LedState != rec.LedState {
					updGrade["current_r"] = rec.CurrentR
					updGrade["current_y"] = rec.CurrentY
					updGrade["current_g"] = rec.CurrentG
					updGrade["led_state"] = rec.LedState
				}
				model.DB.Model(&existing).Updates(updGrade)
				// critical 自动派单（M1 原子防重，内部回填 work_order_id/confirmed）
				if existing.FaultLevel == "critical" && judge.FaultLevel == "critical" {
					model.EnsureActiveWorkOrder(model.DB, existing.ID, recognition.LedUUID(rec.LedHwID))
					dispatchIf = true
				}
				h.logger.Info("待确认故障自动升级确认",
					zap.Uint("faultId", existing.ID),
					zap.Int8("errCode", rec.ErrCode),
					zap.Float64("confidence", judge.Confidence),
					zap.Bool("dispatched", dispatchIf),
				)
				return
			}

			// 常规在窗更新 lastSeen；仅当电流/灯态有变化时才附带更新这些字段，
			// 避免高频上报时的无意义整行写（B2：恒真更新）。
			updates := map[string]interface{}{
				"last_seen": now,
			}
			if existing.CurrentR != rec.CurrentR || existing.CurrentY != rec.CurrentY ||
				existing.CurrentG != rec.CurrentG || existing.LedState != rec.LedState {
				updates["current_r"] = rec.CurrentR
				updates["current_y"] = rec.CurrentY
				updates["current_g"] = rec.CurrentG
				updates["led_state"] = rec.LedState
			}
			model.DB.Model(&existing).Updates(updates)
			return
		}
		// 超过去重窗口，将旧故障标记为已解决，创建新故障记录
		model.DB.Model(&existing).Update("status", model.FaultStatusResolved)
	}

	// 创建新故障记录（研判结果落库）
	fault := model.FaultRecord{
		DeviceHwID:        recognition.LedUUID(rec.LedHwID),
		ErrCode:           rec.ErrCode,
		FaultType:         judge.FaultType,
		FaultLevel:        judge.FaultLevel,
		LedState:          rec.LedState,
		CurrentR:          rec.CurrentR,
		CurrentY:          rec.CurrentY,
		CurrentG:          rec.CurrentG,
		FirstSeen:         now,
		LastSeen:          now,
		Status:            model.FaultStatusOccurred,
		Confidence:        &judge.Confidence,
		RecognitionSource: judge.RecognitionSource,
		RecognitionStatus: judge.RecognitionStatus,
		EvidenceCount:     judge.EvidenceCount,
		LastEvaluationID:  judge.EvaluationID,
	}

	if err := model.DB.Create(&fault).Error; err != nil {
		h.logger.Error("创建故障记录失败",
			zap.Uint32("hwId", rec.LedHwID),
			zap.Int8("errCode", rec.ErrCode),
			zap.Error(err),
		)
		return
	}

	// 多源证据落库 + 案例沉淀（本次研判批次）
	h.persistEvidence(eval, &fault, judge, rec, now)
	h.persistCase(eval, judge)

	// 预警生成：确认的故障自动产生一条预警记录（闭合数据链路：设备→故障→预警→转工单/忽略→工单）
	// 预警独立于工单，可被 转工单 或 忽略（含预警配置自动忽略），不影响既有故障-派单链路。
	if judge.RecognitionStatus == model.RecognitionConfirmed {
		h.createWarningFromFault(&fault, judge, now)
	}

	// 严重故障自动生成工单 —— 仅当研判为【确认】状态才自动派单（R6 语义保持）；
	// 待确认/存疑故障不自动派单，可被证据补充后升级确认后再派。
	if judge.FaultLevel == "critical" && judge.RecognitionStatus == model.RecognitionConfirmed {
		h.createWorkOrder(&fault)
	}

	h.logger.Info("故障研判完成",
		zap.Uint32("hwId", rec.LedHwID),
		zap.Int8("errCode", rec.ErrCode),
		zap.String("faultType", judge.FaultType),
		zap.String("faultLevel", judge.FaultLevel),
		zap.Float64("confidence", judge.Confidence),
		zap.String("recognitionStatus", judge.RecognitionStatus),
		zap.Uint("faultId", fault.ID),
	)
}

// createWarningFromFault 确认的故障 → 生成一条预警记录（闭合数据链路：设备→故障→预警→处理）。
// 预警独立于工单，可被 转工单 或 忽略（含预警配置自动忽略）处置。
// 同 fault 仅生成一条预警（幂等：已存在 source=fault+fault_id 的预警则不重复）。
func (h *Handler) createWarningFromFault(fault *model.FaultRecord, judge model.FaultRecognition, now time.Time) {
	if model.DB == nil || fault == nil {
		return
	}
	// 幂等：同一故障只产生一条预警
	var cnt int64
	model.DB.Model(&model.Warning{}).Where("source = ? AND fault_id = ?", model.WarningSourceFault, fault.ID).Count(&cnt)
	if cnt > 0 {
		return
	}

	level := model.WarningLevelWarning
	if fault.FaultLevel == "critical" {
		level = model.WarningLevelCritical
	}

	w := &model.Warning{
		DeviceHwID:   fault.DeviceHwID,
		WarningCode:  int(fault.ErrCode),
		WarningLabel: FaultTypeFromErrCode(fault.ErrCode),
		Level:        level,
		Source:       model.WarningSourceFault,
		DealState:    model.WarningDealUnhandled,
		Status:       model.WarningUntransferred,
		FaultID:      &fault.ID,
		OccurredAt:   now,
		Remark:       "自动生成：故障已确认（" + fault.FaultType + "）",
	}
	// 预警路口归属（由设备接口冗余）
	var dev model.Device
	if model.DB.Where("hw_id = ?", fault.DeviceHwID).First(&dev).Error == nil {
		w.CrossingID = dev.CrossingID
		w.EquipmentUUID = dev.Intersection
	}

	if err := model.DB.Create(w).Error; err != nil {
		h.logger.Warn("生成预警失败", zap.Uint("faultId", fault.ID), zap.Error(err))
		return
	}

	// 预警配置自动忽略检查：命中忽略规则则直接置为已忽略（不生成待处理）
	var rules []model.WarningRule
	if model.DB.Where("enabled = ?", true).Find(&rules).Error == nil {
		rt := time.Now()
		for i := range rules {
			if rules[i].Matches(w) {
				model.DB.Model(w).Updates(map[string]interface{}{
					"deal_state":    model.WarningDealIgnored,
					"ignore_reason": "自动忽略规则 [" + rules[i].Name + "]",
					"resolved_at":   &rt,
				})
				break
			}
		}
	}

	h.logger.Info("已生成预警",
		zap.Uint("warningId", w.ID),
		zap.Uint("faultId", fault.ID),
		zap.String("hwId", fault.DeviceHwID),
		zap.Int8("errCode", fault.ErrCode),
	)
}

// createWorkOrder 自动生成维修工单（M1：并发安全）
// 委托 model.EnsureActiveWorkOrder 原子式创建/复用单条活跃工单，配合 work_orders 活跃工单部分唯一索引，
// 保证同一故障无论从哪条并发入口触发都只建成一条活跃工单。
func (h *Handler) createWorkOrder(fault *model.FaultRecord) {
	if model.DB == nil {
		return
	}

	wo := model.EnsureActiveWorkOrder(model.DB, fault.ID, fault.DeviceHwID)
	if wo == nil {
		h.logger.Error("自动生成工单失败或已存在",
			zap.Uint("faultId", fault.ID),
			zap.String("hwId", fault.DeviceHwID),
		)
		return
	}

	h.logger.Info("自动生成维修工单",
		zap.String("orderNo", wo.OrderNo),
		zap.Uint("orderId", wo.ID),
		zap.Uint("faultId", fault.ID),
		zap.String("hwId", fault.DeviceHwID),
	)
}

// HandleCheckFW 处理设备固件查询（CMD_CHECK_FW 0x30）
// 设备上报当前 swVer，服务器查询是否有已发布的更高版本固件：
// 有 -> 回应含目标版本与固件信息，指导设备发起升级；无 -> 回应无新版本（目标版本号填 0）
func (h *Handler) HandleCheckFW(frame *CmdFrame, uplinkTopic string) {
	deviceHwID := topicHwID(uplinkTopic)
	h.logger.Info("固件查询请求",
		zap.Uint32("hwId", deviceHwID),
		zap.Uint32("swVer", frame.SwVer),
		zap.Uint16("cmdSeq", frame.CmdSeq),
	)

	// 查询最新已发布固件
	var fw model.FirmwarePackage
	err := model.DB.Where("published = ?", true).Order("major DESC, minor DESC, build DESC").First(&fw).Error
	if err != nil {
		// 无可用固件，回应无新版本
		h.sendFWCheckAck(frame, uplinkTopic, 0, "")
		return
	}

	// 是否可升级：比较版本（优先位域值，其次大/次版本号）
	targetSwVer := fw.SwVersion
	if targetSwVer == 0 {
		// 位域值未设置时，按解析版本号比较
		if swVerMajor(frame.SwVer) > fw.Major ||
			(swVerMajor(frame.SwVer) == fw.Major && swVerMinor(frame.SwVer) >= fw.Minor) {
			h.sendFWCheckAck(frame, uplinkTopic, 0, "")
			return
		}
	} else if frame.SwVer >= targetSwVer {
		h.sendFWCheckAck(frame, uplinkTopic, 0, "")
		return
	}

	// 有可升级固件：回应目标版本位域值
	h.logger.Info("检测到设备固件可升级",
		zap.Uint32("hwId", deviceHwID),
		zap.String("target", fw.Version),
		zap.Uint32("targetSwVer", fw.SwVersion),
	)
	h.sendFWCheckAck(frame, uplinkTopic, fw.SwVersion, fw.Version)
}

// sendFWCheckAck 发送固件查询回应帧
// targetSwVer 为目标固件位域值；0 表示无新版本
func (h *Handler) sendFWCheckAck(frame *CmdFrame, uplinkTopic string, targetSwVer uint32, targetVer string) {
	if h.mqttClient == nil || !h.mqttClient.IsConnected() {
		return
	}
	// 数据部分：目标固件位域值(4字节)。0 表示无新版本
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, targetSwVer)
	ackCmd := MakeAckCmd(CmdCheckFW)
	payload := BuildCmdFrame(ackCmd, frame.SwVer, frame.CmdSeq, targetSwVer, data)
	downTopic := buildDownTopic(uplinkTopic, frame.CmdSeq)
	if err := h.mqttClient.Publish(downTopic, 1, payload); err != nil {
		h.logger.Error("发送固件查询回应失败",
			zap.String("topic", downTopic),
			zap.Error(err),
		)
		return
	}
	h.logger.Info("已发送固件查询回应",
		zap.String("topic", downTopic),
		zap.Uint32("targetSwVer", targetSwVer),
		zap.String("targetVersion", targetVer),
	)
}

// HandleGetFW 处理设备固件数据请求（CMD_GET_FW 0x31）
// 设备请求固件升级数据，服务器回应最新已发布固件包的位域值供其校验
func (h *Handler) HandleGetFW(frame *CmdFrame, uplinkTopic string) {
	deviceHwID := topicHwID(uplinkTopic)
	h.logger.Info("固件数据请求",
		zap.Uint32("hwId", deviceHwID),
		zap.Uint32("swVer", frame.SwVer),
		zap.Uint16("cmdSeq", frame.CmdSeq),
	)

	var fw model.FirmwarePackage
	err := model.DB.Where("published = ?", true).Order("major DESC, minor DESC, build DESC").First(&fw).Error
	if err != nil {
		h.sendFWCheckAck(frame, uplinkTopic, 0, "")
		return
	}
	h.sendFWCheckAck(frame, uplinkTopic, fw.SwVersion, fw.Version)
}

// buildDownTopic 从上行 Topic 构造下行 Topic：将末尾 /U 替换为 /D
func buildDownTopic(uplinkTopic string, cmdSeq uint16) string {
	down := strings.TrimSuffix(uplinkTopic, "/U") + "/D"
	if down == "/D" {
		down = fmt.Sprintf("trafficLight/down/%d/ack", cmdSeq)
	}
	return down
}

// topicHwID 从设备上行 Topic 提取硬件 ID（协议层 uint32，仅用于日志溯源）。
// 上行 Topic 格式：{prefix}/{网络号}/{站点号}/{硬件ID}/U，硬件ID 位于倒数第 2 段。
// 无法解析时返回 0（无意义），不影响业务。
func topicHwID(uplinkTopic string) uint32 {
	segments := strings.Split(uplinkTopic, "/")
	// 需至少有 {prefix}/{net}/{station}/{hwid}/{U} 五段，hwid 在倒数第 2 段
	if len(segments) < 5 {
		return 0
	}
	n, err := strconv.ParseUint(segments[len(segments)-2], 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}

// swVerMajor / swVerMinor 提取位域版本号主/次版本（bit31:28 / bit27:24）
func swVerMajor(v uint32) uint32 { return (v >> 28) & 0xF }
func swVerMinor(v uint32) uint32 { return (v >> 24) & 0xF }

// sendTimeSyncAck 发送时间同步回应
// 对 CMD_CHECKIN 和 CMD_POWER_ON，通过 userVal 返回当前 epoch seconds（UTC+8）
// uplinkTopic 为设备上行 Topic，将末尾 /U 替换为 /D 构造下行 Topic
func (h *Handler) sendTimeSyncAck(frame *CmdFrame, uplinkTopic string) {
	if h.mqttClient == nil || !h.mqttClient.IsConnected() {
		return
	}

	// 获取当前 epoch seconds（UTC+8 时区修正）
	loc, _ := time.LoadLocation("Asia/Shanghai")
	epochSeconds := uint32(time.Now().In(loc).Unix())

	// 构造时间同步回应帧
	ackPayload := BuildTimeSyncAck(frame.Cmd, frame.SwVer, frame.CmdSeq, epochSeconds)

	// 从上行 Topic 构造下行 Topic：将末尾 /U 替换为 /D
	downTopic := buildDownTopic(uplinkTopic, frame.CmdSeq)

	if err := h.mqttClient.Publish(downTopic, 1, ackPayload); err != nil {
		h.logger.Error("发送时间同步回应失败",
			zap.String("topic", downTopic),
			zap.Uint16("cmdSeq", frame.CmdSeq),
			zap.Error(err),
		)
	}
}

// logPacket 记录报文日志到数据库
func (h *Handler) logPacket(deviceHwID string, rawData []byte, cmdType uint8, cmdSeq uint16, parsedResult string, valid bool) {
	if model.DB == nil {
		return
	}

	log := model.PacketLog{
		DeviceHwID:   deviceHwID,
		RawData:      rawData,
		CmdType:      cmdType,
		CmdSeq:       cmdSeq,
		ParsedResult: parsedResult,
		Valid:        valid,
		ReceivedAt:   time.Now(),
	}

	if err := model.DB.Create(&log).Error; err != nil {
		h.logger.Error("记录报文日志失败",
			zap.String("hwId", deviceHwID),
			zap.Error(err),
		)
	}
}

// injectAuxEvidence 检索并注入该设备在近时间窗内的辅助证据（群众反映/手机举证/视频监控/电流异常）。
// 本阶段：真实视频分析/AI 视觉不实现（P1/P2），此处从 DeviceMedia/Feedback 已落库记录中
// 汇入已归一化的佐证信号，作为多源交叉验证的输入；无记录则不注入（不阻塞规则主通道）。
// hwID 为协议帧 uint32，查询台账时转 uuid 字符串。
func injectAuxEvidence(eval *recognition.Evaluator, hwID uint32, now time.Time) {
	hwUUID := recognition.LedUUID(hwID)
	windowStart := now.Add(-24 * time.Hour) // 检索窗口：近 24h 的辅助证据

	// 群众反映（Feedback）—— 辅助证据
	var feedbacks []model.Feedback
	model.DB.Where("device_hw_id = ? AND created_at >= ?", hwUUID, windowStart).Find(&feedbacks)
	for i := range feedbacks {
		fb := &feedbacks[i]
		fid := fb.ID
		eval.AddEvidence(recognition.RuleEvidence{
			DeviceHwID:    hwUUID,
			SourceType:    model.EvSourceCitizen,
			RawData:       "反馈#" + fb.Title + " | " + fb.Content,
			CapturedAt:    fb.CreatedAt,
			RefFeedbackID: &fid,
			Confidence:    0.9,
		})
	}

	// 手机举证 / 视频监控媒体（DeviceMedia）—— 辅助证据
	var medias []model.DeviceMedia
	model.DB.Where("device_hw_id = ? AND media_type IN ? AND created_at >= ?",
		hwUUID, []string{model.MediaEvidence, model.MediaMonitoring, model.MediaTimelapse}, windowStart).Find(&medias)
	for i := range medias {
		md := &medias[i]
		mid := md.ID
		src := model.EvSourcePhotoEvidence
		if md.MediaType == model.MediaMonitoring || md.MediaType == model.MediaTimelapse {
			src = model.EvSourceVideoMonitor
		}
		eval.AddEvidence(recognition.RuleEvidence{
			DeviceHwID: hwUUID,
			SourceType: src,
			RawData:    "媒体#" + md.Title + " | " + md.URL,
			CapturedAt: md.CreatedAt,
			RefMediaID: &mid,
			Confidence: 0.9,
		})
	}
}

// persistEvidence 把本次研判的多源证据落库（fault_evidence）。
// 主信号（firmware：errCode/电流/灯态）+ 注入的辅助证据统一写库，可溯源。
// faultID 为 nil 时表示被过滤/未落故障的证据（仍保留，保证 100% 可溯源）。
func (h *Handler) persistEvidence(eval *recognition.Evaluator, fault *model.FaultRecord, judge model.FaultRecognition, rec *EventRecord, now time.Time) {
	var faultID *uint
	if fault != nil {
		faultID = &fault.ID
	}

	// 主信号固件证据
	errCode := rec.ErrCode
	led := rec.LedState
	curR := rec.CurrentR
	curY := rec.CurrentY
	curG := rec.CurrentG
	primaryEv := recognition.RuleEvidence{
		DeviceHwID:    recognition.LedUUID(rec.LedHwID),
		SourceType:    model.EvSourceFirmware,
		ErrCode:       &errCode,
		LedState:      &led,
		CurrentR:      &curR,
		CurrentY:      &curY,
		CurrentG:      &curG,
		RawData:       convertInt8(errCode),
		CapturedAt:    now,
		RefMediaID:    nil,
		RefFeedbackID: nil,
		Confidence:    judge.Confidence,
	}
	recs := []*recognition.RuleEvidence{&primaryEv}

	// 注入的辅助证据
	for i := range eval.Evidence() {
		ev := eval.Evidence()[i]
		if ev.DeviceHwID == "" {
			ev.DeviceHwID = recognition.LedUUID(rec.LedHwID)
		}
		recs = append(recs, &ev)
	}

	for _, r := range recs {
		m := recognition.EvidenceToModel(*r, faultID, judge.EvaluationID)
		if err := model.DB.Create(&m).Error; err != nil {
			h.logger.Error("写入多源证据失败",
				zap.Uint32("hwId", rec.LedHwID),
				zap.String("source", r.SourceType),
				zap.Error(err),
			)
		}
	}
}

// persistCase 把本次研判沉淀为案例库样本（fault_case），供后续训练/100% 识别达标。
func (h *Handler) persistCase(eval *recognition.Evaluator, judge model.FaultRecognition) {
	cr := caselib.NewCaseRecorder(model.DB)
	if _, err := cr.SeedRecord(eval, judge, judge.EvaluationID); err != nil {
		h.logger.Error("写入案例库失败",
			zap.Uint32("hwId", eval.DeviceHwID),
			zap.Error(err),
		)
	}
}

// convertInt8 把 errCode 转为可读文本（证据 raw_data 用）
func convertInt8(v int8) string {
	return strconv.Itoa(int(v))
}
func (h *Handler) buildParsedResult(frame *CmdFrame, eventPak *EventPak) string {
	result := map[string]interface{}{
		"cmd":     frame.Cmd,
		"ver":     frame.Ver,
		"swVer":   frame.SwVer,
		"cmdSeq":  frame.CmdSeq,
		"datLen":  frame.DatLen,
		"userVal": frame.UserVal,
	}

	if eventPak != nil {
		records := make([]map[string]interface{}, 0, len(eventPak.Records))
		for _, rec := range eventPak.Records {
			records = append(records, map[string]interface{}{
				"ledHwId":  rec.LedHwID,
				"subHwId":  rec.SubHwID,
				"swVer":    rec.SwVer,
				"confVer":  rec.ConfVer,
				"ledState": rec.LedState,
				"errCode":  rec.ErrCode,
				"currentR": rec.CurrentR,
				"currentY": rec.CurrentY,
				"currentG": rec.CurrentG,
			})
		}
		result["eventRecords"] = records
	}

	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

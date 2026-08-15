package mqtt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/tsloms/server/internal/model"
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
	logger, _ := zap.NewProduction()
	return &Handler{
		logger:     logger,
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
		h.logPacket(0, payload, 0, 0, "", false)
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
	var deviceHwID uint32
	if eventPak != nil && len(eventPak.Records) > 0 {
		deviceHwID = eventPak.Records[0].LedHwID
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
		for _, rec := range eventPak.Records {
			h.upsertDevice(rec, now)
			// 签到中如果包含故障记录，也进行故障研判
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

	for _, rec := range eventPak.Records {
		// 更新设备信息
		h.upsertDevice(rec, time.Now())
		// 故障研判与工单生成
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
		for _, rec := range eventPak.Records {
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

// upsertDevice 新增或更新设备信息
// 根据 ledHwId 查找设备，存在则更新版本号和在线状态，不存在则创建
func (h *Handler) upsertDevice(rec EventRecord, checkinTime time.Time) {
	if model.DB == nil {
		return
	}

	var device model.Device
	result := model.DB.Where("hw_id = ?", rec.LedHwID).First(&device)

	if result.Error != nil {
		// 设备不存在，创建新设备
		device = model.Device{
			HwID:          rec.LedHwID,
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

// processFault 故障研判与去重
// 同一设备同一 errCode 在 30 分钟内只生成一条故障记录，后续更新 lastSeen
// 严重故障自动生成维修工单
func (h *Handler) processFault(rec *EventRecord) {
	if model.DB == nil {
		return
	}

	now := time.Now()
	dedupWindow := 30 * time.Minute

	// 查找同一设备同一错误码的活跃故障记录
	var existing model.FaultRecord
	result := model.DB.Where(
		"device_hw_id = ? AND err_code = ? AND status IN ?",
		rec.LedHwID, rec.ErrCode,
		[]string{model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched},
	).First(&existing)

	if result.Error == nil {
		// 故障已存在，检查是否在去重窗口内
		if now.Sub(existing.LastSeen) <= dedupWindow {
			// 在去重窗口内，仅更新 lastSeen 和电流值
			updates := map[string]interface{}{
				"last_seen": now,
				"current_r": rec.CurrentR,
				"current_y": rec.CurrentY,
				"current_g": rec.CurrentG,
				"led_state": rec.LedState,
			}
			model.DB.Model(&existing).Updates(updates)
			return
		}
		// 超过去重窗口，将旧故障标记为已解决，创建新故障记录
		model.DB.Model(&existing).Update("status", model.FaultStatusResolved)
	}

	// 创建新故障记录
	faultType := FaultTypeFromErrCode(rec.ErrCode)
	faultLevel := FaultLevelFromErrCode(rec.ErrCode)

	fault := model.FaultRecord{
		DeviceHwID: rec.LedHwID,
		ErrCode:    rec.ErrCode,
		FaultType:  faultType,
		FaultLevel: faultLevel,
		LedState:   rec.LedState,
		CurrentR:   rec.CurrentR,
		CurrentY:   rec.CurrentY,
		CurrentG:   rec.CurrentG,
		FirstSeen:  now,
		LastSeen:   now,
		Status:     model.FaultStatusOccurred,
	}

	if err := model.DB.Create(&fault).Error; err != nil {
		h.logger.Error("创建故障记录失败",
			zap.Uint32("hwId", rec.LedHwID),
			zap.Int8("errCode", rec.ErrCode),
			zap.Error(err),
		)
		return
	}

	// 严重故障自动生成工单
	if faultLevel == "critical" {
		h.createWorkOrder(&fault)
	}

	h.logger.Info("故障研判完成",
		zap.Uint32("hwId", rec.LedHwID),
		zap.Int8("errCode", rec.ErrCode),
		zap.String("faultType", faultType),
		zap.String("faultLevel", faultLevel),
		zap.Uint("faultId", fault.ID),
	)
}

// createWorkOrder 自动生成维修工单
func (h *Handler) createWorkOrder(fault *model.FaultRecord) {
	if model.DB == nil {
		return
	}

	// 生成工单编号：WO{yyyyMMdd}{4位自增序号}
	orderNo := model.NextOrderNo(model.DB)

	wo := model.WorkOrder{
		OrderNo:    orderNo,
		FaultID:    fault.ID,
		DeviceHwID: fault.DeviceHwID,
		Status:     model.WorkOrderStatusPending,
	}

	if err := model.DB.Create(&wo).Error; err != nil {
		h.logger.Error("创建工单失败",
			zap.Uint("faultId", fault.ID),
			zap.Error(err),
		)
		return
	}

	// 关联故障记录的工单 ID，并将故障状态推进到“已确认”
	now := time.Now()
	model.DB.Model(fault).Updates(map[string]interface{}{
		"work_order_id": wo.ID,
		"status":        model.FaultStatusConfirmed,
		"confirmed_at":  &now,
	})

	h.logger.Info("自动生成维修工单",
		zap.String("orderNo", orderNo),
		zap.Uint("faultId", fault.ID),
		zap.Uint32("hwId", fault.DeviceHwID),
	)
}

// HandleCheckFW 处理设备固件查询（CMD_CHECK_FW 0x30）
// 设备上报当前 swVer，服务器查询是否有已发布的更高版本固件：
// 有 -> 回应含目标版本与固件信息，指导设备发起升级；无 -> 回应无新版本（目标版本号填 0）
func (h *Handler) HandleCheckFW(frame *CmdFrame, uplinkTopic string) {
	deviceHwID := frameHwID(frame)
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
	deviceHwID := frameHwID(frame)
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

// frameHwID 从命令帧中提取设备硬件 ID
// 固件命令帧不含事件记录，无法直接取 LedHwID；此处返回 0（无事件数据时）
func frameHwID(frame *CmdFrame) uint32 {
	return 0
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
func (h *Handler) logPacket(deviceHwID uint32, rawData []byte, cmdType uint8, cmdSeq uint16, parsedResult string, valid bool) {
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
			zap.Uint32("hwId", deviceHwID),
			zap.Error(err),
		)
	}
}

// buildParsedResult 构造解析结果 JSON 字符串
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

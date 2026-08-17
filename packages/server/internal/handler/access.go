package handler

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/mqtt"
)

// mqttGlobal 保存服务器持有的 MQTT 客户端（用于状态上报；由 cmd/server 启动期注册）。
var (
	mqttGlobalMu sync.RWMutex
	mqttGlobal   interface {
		IsConnected() bool
	}
)

// SetMQTTClient 注册全局 MQTT 客户端（由 cmd/server 启动期调用）。
func SetMQTTClient(mc interface {
	IsConnected() bool
}) {
	mqttGlobalMu.Lock()
	defer mqttGlobalMu.Unlock()
	mqttGlobal = mc
}

func mqttGlobalClient() interface {
	IsConnected() bool
} {
	mqttGlobalMu.RLock()
	defer mqttGlobalMu.RUnlock()
	return mqttGlobal
}

// ---------------------------------------------------------------------------
// 检测器接入（三种接入方式：真实硬件 / CSV 导入 / Mock 模拟）
// ---------------------------------------------------------------------------

// DetectorAccessStatus 回传检测器接入总体状态。
// 真实硬件依赖 MQTT Broker 连接；Mock 与 CSV 走本地模拟分派（无需 Broker）。
func DetectorAccessStatus(c *gin.Context) {
	mc := mqttGlobalClient()
	connected := mc != nil && mc.IsConnected()

	// 订阅配置（读配置项；未能取到则给默认说明）
	topic := mqtt.SubscribeTopic()
	if topic == "" {
		topic = "{prefix}/{网络号}/{站点号}/{硬件ID}/U"
	}

	// 近 N 分钟有上报的在线设备数
	var devices, activeDetectors int64
	hasDB := model.DB != nil
	if hasDB {
		model.DB.Model(&model.Device{}).Where("online_status = ?", true).Count(&devices)
		// 近 30 分钟收到上行的设备数（活跃检测器）
		cut := time.Now().Add(-30 * time.Minute)
		model.DB.Model(&model.Device{}).Where("last_checkin_at >= ?", cut).Count(&activeDetectors)
	}

	ok(c, gin.H{
		"mqtt": gin.H{
			"connected":    connected,
			"subscribe":    topic,
			"topic_prefix": mqtt.SubscribeTopic(),
		},
		"real_hardware": gin.H{
			"mode":             "real",
			"enabled":          connected,
			"online_devices":   devices,
			"active_detectors": activeDetectors,
		},
		"mock_enabled": true,
		"csv_enabled":  true,
		"server_time":  time.Now().Format(time.RFC3339),
	})
}

// mockSendReq Mock 模拟发送请求体
type mockSendReq struct {
	HwID     uint32 `json:"hw_id"`     // 硬件 ID（必需）
	Cmd      string `json:"cmd"`       // checkin / alarm / power_on（默认 alarm）
	ErrCode  int8   `json:"err_code"`  // 错误码（默认 LED_ERR_OK）
	LedState int8   `json:"led_state"` // 灯态（0/1/2/-1）
	CurR     uint16 `json:"current_r"`
	CurY     uint16 `json:"current_y"`
	CurG     uint16 `json:"current_g"`
}

// MockSend 发送一条模拟协议帧（走真实研判链路，无硬件/无 Broker 也可用）。
// POST /access/mock/send
func MockSend(c *gin.Context) {
	var req mockSendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if req.HwID == 0 {
		badRequest(c, "硬件ID必填")
		return
	}
	cmd := strings.ToLower(req.Cmd)
	var payload []byte
	var cmdType uint8
	seq := uint16(time.Now().Unix() & 0xFFFF)
	switch cmd {
	case "checkin", "签到":
		cmdType = mqtt.CmdCheckin
		payload = mqtt.BuildCheckinPayload(req.HwID, seq, req.ErrCode, req.LedState, req.CurR, req.CurY, req.CurG)
	case "power_on", "上电":
		cmdType = mqtt.CmdPowerOn
		payload = mqtt.BuildPowerOnPayload(req.HwID, seq)
	default: // alarm
		cmdType = mqtt.CmdAlarm
		payload = mqtt.BuildAlarmPayload(req.HwID, seq, req.ErrCode, req.LedState, req.CurR, req.CurY, req.CurG)
	}

	trace, err := mqtt.DispatchFrame(req.HwID, cmdType, payload)
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"sent": true, "cmd": trace.Cmd, "topic": trace.Topic, "hw_id": req.HwID, "message": "模拟帧已投递"})
}

// csvImportReq CSV 导入请求体：rows 为二维字符串数组（首行表头可选）。
// 表头示例：hw_id,cmd,err_code,led_state,current_r,current_y,current_g
// 或：硬件ID,命令,错误码,灯态,红灯电流,黄灯电流,绿灯电流
type csvImportReq struct {
	Rows []csvRow `json:"rows"`
}

type csvRow struct {
	HwID     uint32 `json:"hw_id"`
	Cmd      string `json:"cmd"` // checkin/alarm/power_on（默认 alarm）
	ErrCode  int8   `json:"err_code"`
	LedState int8   `json:"led_state"`
	CurR     uint16 `json:"current_r"`
	CurY     uint16 `json:"current_y"`
	CurG     uint16 `json:"current_g"`
}

// CSVImport 导入 CSV 并逐行回放（构造协议帧走真实研判链路）。
// POST /access/csv/import   body: {"content":"<csv文本>"} 或 {"rows":[...]}
func CSVImport(c *gin.Context) {
	var body struct {
		Content string   `json:"content"`
		Rows    []csvRow `json:"rows"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "参数错误")
		return
	}

	var rows []csvRow
	if len(body.Rows) > 0 {
		rows = body.Rows
	} else if body.Content != "" {
		var parseErr error
		rows, parseErr = parseCSVRows(body.Content)
		if parseErr != nil {
			badRequest(c, "CSV 解析失败: "+parseErr.Error())
			return
		}
	} else {
		badRequest(c, "请提供 CSV 内容或 rows")
		return
	}
	if len(rows) == 0 {
		badRequest(c, "无有效行")
		return
	}

	okCount, errRows := replayRows(rows)
	ok(c, gin.H{
		"imported": okCount, "failed": len(errRows), "total": len(rows),
		"errors": errRows, "message": "已导入并回放 " + strconv.Itoa(okCount) + " 条",
	})
}

// replayRows 逐行构造帧并投递，返回成功数与错误明细。
func replayRows(rows []csvRow) (int, []gin.H) {
	okN := 0
	var failed []gin.H
	seq := uint16(1)
	for i, r := range rows {
		if r.HwID == 0 {
			failed = append(failed, gin.H{"row": i + 1, "error": "hw_id 必填"})
			continue
		}
		cmd := strings.ToLower(r.Cmd)
		var payload []byte
		var cmdType uint8
		switch cmd {
		case "checkin":
			cmdType = mqtt.CmdCheckin
			payload = mqtt.BuildCheckinPayload(r.HwID, seq, r.ErrCode, r.LedState, r.CurR, r.CurY, r.CurG)
		case "power_on":
			cmdType = mqtt.CmdPowerOn
			payload = mqtt.BuildPowerOnPayload(r.HwID, seq)
		default:
			cmdType = mqtt.CmdAlarm
			payload = mqtt.BuildAlarmPayload(r.HwID, seq, r.ErrCode, r.LedState, r.CurR, r.CurY, r.CurG)
		}
		seq++
		if _, err := mqtt.DispatchFrame(r.HwID, cmdType, payload); err != nil {
			failed = append(failed, gin.H{"row": i + 1, "error": err.Error()})
			continue
		}
		okN++
	}
	return okN, failed
}

// parseCSVRows 解析 CSV 文本（支持中文表头）。
func parseCSVRows(content string) ([]csvRow, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true
	var rows []csvRow
	first := true
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if first {
			first = false
			// 若首行为表头（含“硬件ID/hw_id/命令”等关键字）则跳过
			if len(rec) > 0 && (strings.Contains(rec[0], "hw") || strings.Contains(rec[0], "硬件ID") || strings.EqualFold(rec[0], "id")) {
				continue
			}
		}
		row := csvRow{}
		if len(rec) > 0 {
			v, _ := strconv.ParseUint(strings.TrimSpace(rec[0]), 10, 32)
			row.HwID = uint32(v)
		}
		if len(rec) > 1 {
			row.Cmd = strings.TrimSpace(rec[1])
		}
		if len(rec) > 2 {
			v, _ := strconv.ParseInt(strings.TrimSpace(rec[2]), 10, 8)
			row.ErrCode = int8(v)
		}
		if len(rec) > 3 {
			v, _ := strconv.ParseInt(strings.TrimSpace(rec[3]), 10, 8)
			row.LedState = int8(v)
		}
		if len(rec) > 4 {
			v, _ := strconv.ParseUint(strings.TrimSpace(rec[4]), 10, 16)
			row.CurR = uint16(v)
		}
		if len(rec) > 5 {
			v, _ := strconv.ParseUint(strings.TrimSpace(rec[5]), 10, 16)
			row.CurY = uint16(v)
		}
		if len(rec) > 6 {
			v, _ := strconv.ParseUint(strings.TrimSpace(rec[6]), 10, 16)
			row.CurG = uint16(v)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

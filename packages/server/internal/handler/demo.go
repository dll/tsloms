package handler

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tsloms/server/internal/model"
)

// ---------------------------------------------------------------------------
// 系统演示：生成随机演示数据 / 一键清理回滚（不影响生产数据）
// ---------------------------------------------------------------------------
// 演示设备使用独立硬件ID段（uuid 前缀 DEMO，编号 900001-909999），且路口名/名称打 [演示] 标记；
// 清理仅按本段前缀+标记删除，绝不动生产设备/路口/故障/工单。

const (
	demoHWPrefix = "DEMO" // 演示设备硬件ID uuid 前缀
	demoHwStart  = 900001 // 演示设备硬件ID段起始
	demoHwEnd    = 909999 // 演示设备硬件ID段结束
	demoMark     = "[演示]"
)

// demoHWID 生成演示设备 uuid 硬件ID：DEMO900001 形式
func demoHWID(i int) string {
	return fmt.Sprintf("%s%d", demoHWPrefix, demoHwStart+i)
}

// demoDevices 随机演示交叉口名（打标记便于识别/清理）
var demoIntersections = []string{"长江中路", "黄河大道", "人民路", "解放路", "中山北路", "文化路", "建设东路", "和平南路", "环城西路", "站前广场"}

// DemoStatus 返回当前是否存在演示数据及数量。
// GET /demo/status
func DemoStatus(c *gin.Context) {
	devCnt := demoDeviceCount()
	ok(c, gin.H{
		"running":      devCnt > 0,
		"devices":      devCnt,
		"intersection": demoIntersectionCount(),
		"hw_range":     fmt.Sprintf("%d-%d", demoHwStart, demoHwEnd),
	})
}

// DemoStart 生成一批随机演示数据（设备/路口/故障/工单）。
// POST /demo/start   body: {n: 5}
func DemoStart(c *gin.Context) {
	var body struct {
		N int `json:"n"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.N <= 0 || body.N > 20 {
		body.N = 5
	}
	now := time.Now()

	// 1) 演示路口（含坐标，供地图显示）
	createdIntersections := 0
	for i := 0; i < body.N; i++ {
		name := demoMark + demoIntersections[rand.Intn(len(demoIntersections))] + strconv.Itoa(rand.Intn(900)+100)
		x := model.Crossing{Name: name, Type: "1", Lat: f64ptr(30.6 + rand.Float64()*1.5), Lng: f64ptr(117.0 + rand.Float64()*1.6), Status: model.CrossingStatusNormal}
		if err := model.DB.Create(&x).Error; err == nil {
			createdIntersections++
		}
	}

	// 2) 演示设备
	createdDevices := 0
	for i := 0; i < body.N*2; i++ {
		hw := demoHWID(i)
		dev := model.Device{
			HwID:         hw,
			Intersection: demoMark + demoIntersections[rand.Intn(len(demoIntersections))],
			NetworkCode:  0,
			StationCode:  0,
			SwVersion:    uint32(0x01020304),
			ConfVersion:  uint32(0x26080101),
			OnlineStatus: rand.Intn(2) == 0,
			Lat:          f64ptr(30.6 + rand.Float64()*1.5),
			Lng:          f64ptr(117.0 + rand.Float64()*1.6),
		}
		if err := model.DB.Create(&dev).Error; err == nil {
			createdDevices++
		}
	}

	// 3) 演示故障（关联演示设备）+ 4) 部分演示工单
	var demoErrs = []int8{-1, -2, -3, -4, -5, -6, -7, -8, -9, -10, -14}
	createdFaults := 0
	createdOrders := 0
	for i := 0; i < body.N*2; i++ {
		hw := demoHWID(i)
		errCode := demoErrs[rand.Intn(len(demoErrs))]
		fault := model.FaultRecord{
			DeviceHwID:        hw,
			ErrCode:           errCode,
			FaultType:         mqttFaultType(errCode),
			FaultLevel:        "critical",
			LedState:          0,
			CurrentR:          uint16(rand.Intn(1200) + 100),
			CurrentY:          uint16(rand.Intn(300)),
			CurrentG:          uint16(rand.Intn(300)),
			FirstSeen:         now.Add(-time.Duration(rand.Intn(60)) * time.Minute),
			LastSeen:          now,
			Status:            model.FaultStatusConfirmed,
			RecognitionStatus: model.RecognitionConfirmed,
		}
		conf := 0.9 + rand.Float64()*0.09
		fault.Confidence = &conf
		RecognitionSource := "demo"
		fault.RecognitionSource = RecognitionSource
		if err := model.DB.Create(&fault).Error; err != nil {
			continue
		}
		createdFaults++
		// 每条故障生成一条已完成/处理中的演示工单
		wo := model.WorkOrder{
			OrderNo:    "WO-DEMO" + now.Format("20060102") + strconv.Itoa(rand.Intn(9000)+1000),
			FaultID:    fault.ID,
			DeviceHwID: hw,
			Status:     []string{"completed", "processing", "pending"}[rand.Intn(3)],
		}
		if err := model.DB.Create(&wo).Error; err == nil {
			createdOrders++
		}
	}

	ok(c, gin.H{
		"message":       "演示数据已生成（不影响生产）",
		"intersections": createdIntersections,
		"devices":       createdDevices,
		"faults":        createdFaults,
		"work_orders":   createdOrders,
	})
}

// DemoEnd 一键清理演示数据并回滚（仅删演示段/标记数据，不动生产）。
// POST /demo/end
func DemoEnd(c *gin.Context) {
	// 1) 删除演示段设备的工单（uuid 前缀 DEMO 匹配）
	orderDeleted := model.DB.Where("device_hw_id LIKE ?", demoHWPrefix+"%").
		Delete(&model.WorkOrder{}).RowsAffected
	// 2) 删除演示段设备的故障
	faultDeleted := model.DB.Where("device_hw_id LIKE ?", demoHWPrefix+"%").
		Delete(&model.FaultRecord{}).RowsAffected
	// 3) 删除演示段设备
	deviceDeleted := model.DB.Where("hw_id LIKE ?", demoHWPrefix+"%").
		Delete(&model.Device{}).RowsAffected
	// 4) 删除演示路口（名称带 [演示]）
	intersectionDeleted := model.DB.Where("name LIKE ?", "%"+demoMark+"%").
		Delete(&model.Crossing{}).RowsAffected

	ok(c, gin.H{
		"message":       "演示数据已清理（生产数据未受影响）",
		"intersections": intersectionDeleted,
		"devices":       deviceDeleted,
		"faults":        faultDeleted,
		"work_orders":   orderDeleted,
	})
}

// demoDeviceCount 统计演示段设备数
func demoDeviceCount() int64 {
	var n int64
	model.DB.Model(&model.Device{}).Where("hw_id LIKE ?", demoHWPrefix+"%").Count(&n)
	return n
}

func demoIntersectionCount() int64 {
	var n int64
	model.DB.Model(&model.Crossing{}).Where("name LIKE ?", "%"+demoMark+"%").Count(&n)
	return n
}

func f64ptr(v float64) *float64 { return &v }

// mqttFaultType 演示用：errCode→故障类型（复用 handler 内可用的定义）
func mqttFaultType(errCode int8) string {
	switch errCode {
	case -1, -2, -3:
		return "lamp_off"
	case -4, -5, -6, -7:
		return "abnormal_on"
	case -8, -9, -10:
		return "timeout"
	case -11, -12, -13:
		return "dim"
	case -14:
		return "power_loss"
	default:
		return "unknown"
	}
}

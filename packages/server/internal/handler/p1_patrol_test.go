package handler

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P1 自动巡检：任务 CRUD/执行、巡检记录、排行、信号灯自检（REST）
// ============================================================================

func p1PatrolEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/patrol/tasks", ListPatrolTasks)
		api.POST("/patrol/tasks", CreatePatrolTask)
		api.GET("/patrol/tasks/:id", GetPatrolTask)
		api.PUT("/patrol/tasks/:id", UpdatePatrolTask)
		api.DELETE("/patrol/tasks/:id", DeletePatrolTask)
		api.POST("/patrol/tasks/:id/run", RunPatrolTask)
		api.GET("/patrol/records", ListPatrolRecords)
		api.GET("/patrol/ranking", GetPatrolRanking)
		api.POST("/patrol/selfcheck", PostPatrolSelfCheck)
	}
	return r
}

// seedPatrolDevices 构造测试设备：2 台正常 + 1 台离线 + 1 台带活跃故障
func seedPatrolDevices(t *testing.T) (normal, offline, faulty model.Device) {
	t.Helper()
	normal = model.Device{HwID: 1001, Intersection: "路口A", OnlineStatus: true}
	offline = model.Device{HwID: 1002, Intersection: "路口B", OnlineStatus: false}
	lat, lng := 31.86, 117.28
	normal.Lat, normal.Lng = &lat, &lng
	offline.Lat, offline.Lng = &lat, &lng
	model.DB.Create(&normal)
	model.DB.Create(&offline)

	// 带活跃故障设备
	model.DB.Create(&model.Device{HwID: 1003, Intersection: "路口C", OnlineStatus: true})
	var faultyD model.Device
	model.DB.Where("hw_id = ?", 1003).First(&faultyD)
	model.DB.Create(&model.FaultRecord{
		DeviceHwID: 1003, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusOccurred, FirstSeen: time.Now(), LastSeen: time.Now(),
	})
	return normal, offline, faultyD
}

func TestPatrolTask_CreateListGet(t *testing.T) {
	r := p1PatrolEngine(t)
	// 创建（random 抽检）
	code, body := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"name":"随机抽检-早班","mode":"random","target_count":2}`)
	mustOK(t, code, body, "创建任务")
	taskID := uint(body["data"].(map[string]interface{})["task"].(map[string]interface{})["id"].(float64))
	if body["data"].(map[string]interface{})["task"].(map[string]interface{})["status"] != model.PatrolStatusPlanned {
		t.Errorf("新建任务状态应为 planned, 实际 %v", body["data"].(map[string]interface{})["task"].(map[string]interface{})["status"])
	}

	// 列表
	_, lb := doReq(t, r, "GET", "/api/v1/patrol/tasks", "")
	if lb["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("任务列表 total 期望 1, 实际 %v", lb["data"].(map[string]interface{})["total"])
	}
	// 详情
	_, gb := doReq(t, r, "GET", "/api/v1/patrol/tasks/"+uid(taskID), "")
	if gb["data"].(map[string]interface{})["task"].(map[string]interface{})["name"] != "随机抽检-早班" {
		t.Errorf("任务详情 name 不符")
	}
}

func TestPatrolTask_InvalidMode(t *testing.T) {
	r := p1PatrolEngine(t)
	_, body := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"name":"坏模式","mode":"bogus"}`)
	if body["code"].(float64) == 0 {
		t.Errorf("不支持的模式应报错")
	}
	// 空 name
	_, body2 := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"mode":"random"}`)
	if body2["code"].(float64) == 0 {
		t.Errorf("缺 name 应报错")
	}
}

func TestPatrolTask_UpdateDelete(t *testing.T) {
	r := p1PatrolEngine(t)
	_, body := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"name":"任务","mode":"street"}`)
	taskID := uint(body["data"].(map[string]interface{})["task"].(map[string]interface{})["id"].(float64))

	_, ub := doReq(t, r, "PUT", "/api/v1/patrol/tasks/"+uid(taskID), `{"name":"改名","target_count":5}`)
	if ub["code"].(float64) != 0 {
		t.Fatalf("更新失败: %v", ub)
	}
	_, gb := doReq(t, r, "GET", "/api/v1/patrol/tasks/"+uid(taskID), "")
	if gb["data"].(map[string]interface{})["task"].(map[string]interface{})["name"] != "改名" {
		t.Errorf("更新后 name 不符")
	}
	if gb["data"].(map[string]interface{})["task"].(map[string]interface{})["target_count"].(float64) != 5 {
		t.Errorf("更新后 target_count 不符")
	}
	_, db := doReq(t, r, "DELETE", "/api/v1/patrol/tasks/"+uid(taskID), "")
	if db["code"].(float64) != 0 {
		t.Fatalf("删除失败: %v", db)
	}
	_, gb2 := doReq(t, r, "GET", "/api/v1/patrol/tasks/"+uid(taskID), "")
	if gb2["code"].(float64) == 0 {
		t.Errorf("删除后应 404/找不到")
	}
}

func TestPatrolTask_RunRandomAndRecords(t *testing.T) {
	r := p1PatrolEngine(t)
	seedPatrolDevices(t)
	_, body := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"name":"随机抽检","mode":"random","target_count":2}`)
	taskID := uint(body["data"].(map[string]interface{})["task"].(map[string]interface{})["id"].(float64))

	// 触发执行
	_, rb := doReq(t, r, "POST", "/api/v1/patrol/tasks/"+uid(taskID)+"/run", "")
	if rb["code"].(float64) != 0 {
		t.Fatalf("执行失败: %v", rb)
	}
	if rb["data"].(map[string]interface{})["created"].(float64) != 2 {
		t.Errorf("随机抽检应产生 2 条记录, 实际 %v", rb["data"].(map[string]interface{})["created"])
	}

	// 记录列表
	_, recb := doReq(t, r, "GET", "/api/v1/patrol/records", "")
	if recb["data"].(map[string]interface{})["total"].(float64) != 2 {
		t.Errorf("记录 total 期望 2, 实际 %v", recb["data"].(map[string]interface{})["total"])
	}

	// 任务状态应 done、run_count=1
	_, gb := doReq(t, r, "GET", "/api/v1/patrol/tasks/"+uid(taskID), "")
	td := gb["data"].(map[string]interface{})["task"].(map[string]interface{})
	if td["status"] != model.PatrolStatusDone {
		t.Errorf("执行后任务状态应为 done, 实际 %v", td["status"])
	}
	if td["run_count"].(float64) != 1 {
		t.Errorf("run_count 期望 1, 实际 %v", td["run_count"])
	}
}

func TestPatrolTask_RunSelfCheck(t *testing.T) {
	r := p1PatrolEngine(t)
	seedPatrolDevices(t)
	// selfcheck 任务（无特定范围 => 全量设备，含离线与故障）
	_, body := doReq(t, r, "POST", "/api/v1/patrol/tasks", `{"name":"信号灯自检","mode":"selfcheck"}`)
	taskID := uint(body["data"].(map[string]interface{})["task"].(map[string]interface{})["id"].(float64))

	_, rb := doReq(t, r, "POST", "/api/v1/patrol/tasks/"+uid(taskID)+"/run", "")
	if rb["code"].(float64) != 0 {
		t.Fatalf("自检执行失败: %v", rb)
	}
	// 3 台设备 → 3 条记录；离线 1 + 故障 1 = 2 异常
	if rb["data"].(map[string]interface{})["created"].(float64) != 3 {
		t.Errorf("自检应产生 3 条记录, 实际 %v", rb["data"].(map[string]interface{})["created"])
	}
	if rb["data"].(map[string]interface{})["abnormal"].(float64) != 2 {
		t.Errorf("应判定 2 异常(离线+故障), 实际 %v", rb["data"].(map[string]interface{})["abnormal"])
	}
}

func TestPatrolRanking_ByPatroler(t *testing.T) {
	r := p1PatrolEngine(t)
	model.InitTestDB()
	now := time.Now()
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 1, PatrolType: "random", CheckResult: model.PatrolResultNormal, PatrolBy: "张三", PatrolAt: now})
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 2, PatrolType: "selfcheck", CheckResult: model.PatrolResultAbnormal, PatrolBy: "张三", PatrolAt: now})
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 3, PatrolType: "random", CheckResult: model.PatrolResultAbnormal, PatrolBy: "李四", PatrolAt: now})

	_, body := doReq(t, r, "GET", "/api/v1/patrol/ranking", "")
	if body["code"].(float64) != 0 {
		t.Fatalf("排行失败: %v", body)
	}
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("按巡检人聚合应有 2 条(张三/李四), 实际 %d", len(list))
	}
	for _, it := range list {
		item := it.(map[string]interface{})
		if item["key"] == "张三" {
			if item["patrol_count"].(float64) != 2 {
				t.Errorf("张三 巡检人次应为 2, 实际 %v", item["patrol_count"])
			}
			if item["abnormal_count"].(float64) != 1 {
				t.Errorf("张三 异常数应为 1, 实际 %v", item["abnormal_count"])
			}
		}
		if item["key"] == "李四" {
			if item["patrol_count"].(float64) != 1 {
				t.Errorf("李四 巡检人次应为 1, 实际 %v", item["patrol_count"])
			}
		}
	}
}

func TestPatrolRanking_ByDevice(t *testing.T) {
	model.InitTestDB()
	now := time.Now()
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 777, PatrolType: "random", CheckResult: model.PatrolResultNormal, PatrolBy: "a", PatrolAt: now})
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 777, PatrolType: "random", CheckResult: model.PatrolResultAbnormal, PatrolBy: "b", PatrolAt: now})
	svc := patrolSvc()
	items, err := svc.Ranking("device", 10)
	if err != nil {
		t.Fatalf("按设备排行失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("应按设备聚合为 1 条, 实际 %d", len(items))
	}
	if items[0].Key != "777" || items[0].PatrolCount != 2 || items[0].AbnormalCnt != 1 {
		t.Errorf("设备排行内容不符: %+v", items[0])
	}
}

func TestPatrolSelfCheck_Post(t *testing.T) {
	r := p1PatrolEngine(t)
	seedPatrolDevices(t)
	// 用 device_ids 直接自检（含离线 + 故障设备）—— 取设备表真实自增 ID
	var devs []model.Device
	model.DB.Where("hw_id IN ?", []uint32{1001, 1002, 1003}).Find(&devs)
	if len(devs) != 3 {
		t.Fatalf("应 seed 3 台设备, 实际 %d", len(devs))
	}
	ids := []uint{devs[0].ID, devs[1].ID, devs[2].ID}
	_, body := doReq(t, r, "POST", "/api/v1/patrol/selfcheck", `{"device_ids":[`+
		uid(ids[0])+`,`+uid(ids[1])+`,`+uid(ids[2])+`]}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("自检接口失败: %v", body)
	}
	if body["data"].(map[string]interface{})["created"].(float64) != 3 {
		t.Errorf("自检应创建 3 条记录, 实际 %v", body["data"].(map[string]interface{})["created"])
	}
	if body["data"].(map[string]interface{})["abnormal"].(float64) != 2 {
		t.Errorf("自检应判定 2 异常, 实际 %v", body["data"].(map[string]interface{})["abnormal"])
	}
	// 记录已落库
	var cnt int64
	model.DB.Model(&model.PatrolRecord{}).Where("patrol_type = ?", model.PatrolModeSelfCheck).Count(&cnt)
	if cnt != 3 {
		t.Errorf("自检记录应落库 3 条, 实际 %d", cnt)
	}

	// 缺参数应报错
	_, body2 := doReq(t, r, "POST", "/api/v1/patrol/selfcheck", `{}`)
	if body2["code"].(float64) == 0 {
		t.Errorf("缺 device 参数应报错")
	}
}

func TestPatrolSelfCheck_ByHwIDs(t *testing.T) {
	r := p1PatrolEngine(t)
	seedPatrolDevices(t)
	_, body := doReq(t, r, "POST", "/api/v1/patrol/selfcheck", `{"device_hw_ids":[1001,1003]}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("按 hw 自检失败: %v", body)
	}
	if body["data"].(map[string]interface{})["created"].(float64) != 2 {
		t.Errorf("按 hw 自检应创建 2 条, 实际 %v", body["data"].(map[string]interface{})["created"])
	}
	if body["data"].(map[string]interface{})["abnormal"].(float64) != 1 {
		t.Errorf("按 hw 自检应判定 1 异常(1003 故障), 实际 %v", body["data"].(map[string]interface{})["abnormal"])
	}
}

func TestPatrolRecords_Filter(t *testing.T) {
	r := p1PatrolEngine(t)
	now := time.Now()
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 1, PatrolType: "random", CheckResult: model.PatrolResultNormal, PatrolBy: "张三", PatrolAt: now})
	model.DB.Create(&model.PatrolRecord{DeviceHwID: 2, PatrolType: "selfcheck", CheckResult: model.PatrolResultAbnormal, PatrolBy: "张三", PatrolAt: now})

	_, body := doReq(t, r, "GET", "/api/v1/patrol/records?check_result=abnormal", "")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("按 check_result=abnormal 过滤 total 期望 1, 实际 %v", body["data"].(map[string]interface{})["total"])
	}
	_, body2 := doReq(t, r, "GET", "/api/v1/patrol/records?patrol_type=random", "")
	if body2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("按 patrol_type=random 过滤 total 期望 1, 实际 %v", body2["data"].(map[string]interface{})["total"])
	}
}

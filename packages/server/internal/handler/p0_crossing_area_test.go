package handler

import (
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-4 路口/行政区划：crossings CRUD + areas 树 + device 挂接区划/路口
// ============================================================================

func p0CrossingEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/crossings", ListCrossings)
		api.POST("/crossings", CreateCrossing)
		api.GET("/crossings/:id", GetCrossing)
		api.PUT("/crossings/:id", UpdateCrossing)
		api.DELETE("/crossings/:id", DeleteCrossing)
		api.GET("/crossings/:id/devices", GetCrossingDevices)
		api.GET("/areas/tree", ListAreasTree)
		api.POST("/areas", CreateArea)
		api.PUT("/areas/:id", UpdateArea)
		api.DELETE("/areas/:id", DeleteArea)
		api.POST("/devices", CreateDevice)
		api.PUT("/devices/:id", UpdateDevice)
	}
	return r
}

func TestAreas_Tree_Seeded(t *testing.T) {
	r := p0CrossingEngine(t)
	_, body := doReq(t, r, "GET", "/api/v1/areas/tree", "")
	if body["code"].(float64) != 0 {
		t.Fatalf("区划树失败: %v", body)
	}
	if body["data"].(map[string]interface{})["total"].(float64) != 8 {
		t.Errorf("区划种子树应含 8 个节点, 实际 %v", body["data"].(map[string]interface{})["total"])
	}
	roots := body["data"].(map[string]interface{})["tree"].([]interface{})
	if len(roots) != 1 {
		t.Errorf("应只有 1 个根(安徽省), 实际 %d", len(roots))
	}
}

func TestAreas_CRUD(t *testing.T) {
	r := p0CrossingEngine(t)
	// 新增区划
	_, body := doReq(t, r, "POST", "/api/v1/areas", `{"name":"新街道","code":"X001","area_type":"street"}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("新增区划失败: %v", body)
	}
	areaID := uint(body["data"].(map[string]interface{})["area"].(map[string]interface{})["id"].(float64))

	// 编辑
	_, body2 := doReq(t, r, "PUT", "/api/v1/areas/"+uid(areaID), `{"name":"改名街道"}`)
	if body2["code"].(float64) != 0 {
		t.Fatalf("编辑区划失败: %v", body2)
	}
	// 删除
	_, body3 := doReq(t, r, "DELETE", "/api/v1/areas/"+uid(areaID), "")
	if body3["code"].(float64) != 0 {
		t.Fatalf("删除区划失败: %v", body3)
	}
}

func TestAreas_DeleteProtectedWithChildren(t *testing.T) {
	r := p0CrossingEngine(t)
	// 种子中「安徽省」有下级，删除应被拒绝
	prov := model.Area{}
	model.DB.Where("area_type = ?", model.AreaProvince).First(&prov)
	code, body := doReq(t, r, "DELETE", "/api/v1/areas/"+uid(prov.ID), "")
	if code == 200 || body["code"].(float64) == 0 {
		t.Errorf("删除含下级的区划应被拒绝")
	}
}

func TestCrossing_CRUD_AndDeviceBound(t *testing.T) {
	r := p0CrossingEngine(t)
	// 准备区划：街道 + 道路
	street := model.Area{Name: "测试街道", Code: "S001", AreaType: model.AreaStreet}
	model.DB.Create(&street)
	road := model.Area{Name: "测试路", Code: "R001", AreaType: model.AreaRoad}
	model.DB.Create(&road)

	// 新增路口（含区划/经纬度）
	_, body := doReq(t, r, "POST", "/api/v1/crossings", `{"name":"测试路口","point_no":"P001","type":"1","street_id":`+
		strconv.FormatUint(uint64(street.ID), 10)+`,"road_id":`+strconv.FormatUint(uint64(road.ID), 10)+
		`,"road_name":"测试路","lat":31.2,"lng":121.4}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("新增路口失败: %v", body)
	}
	cid := uint(body["data"].(map[string]interface{})["crossing"].(map[string]interface{})["id"].(float64))

	// 列表
	_, body2 := doReq(t, r, "GET", "/api/v1/crossings", "")
	if body2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("路口列表 total 期望 1, 实际 %v", body2["data"].(map[string]interface{})["total"])
	}

	// 新增设备并挂接该路口
	_, body3 := doReq(t, r, "POST", "/api/v1/devices", `{"hw_id":"1001","crossing_id":`+strconv.FormatUint(uint64(cid), 10)+`,"road_name":"测试路"}`)
	if body3["code"].(float64) != 0 {
		t.Fatalf("新增设备失败: %v", body3)
	}
	devID := uint(body3["data"].(map[string]interface{})["device"].(map[string]interface{})["id"].(float64))

	// 路口下设备
	_, body4 := doReq(t, r, "GET", "/api/v1/crossings/"+strconv.FormatUint(uint64(cid), 10)+"/devices", "")
	if body4["code"].(float64) != 0 {
		t.Fatalf("路口设备列表失败: %v", body4)
	}
	devs := body4["data"].(map[string]interface{})["devices"].([]interface{})
	if len(devs) != 1 {
		t.Errorf("路口设备数期望 1, 实际 %d", len(devs))
	}

	// 编辑路口（改名）
	_, body5 := doReq(t, r, "PUT", "/api/v1/crossings/"+strconv.FormatUint(uint64(cid), 10), `{"name":"改名路口"}`)
	if body5["code"].(float64) != 0 {
		t.Fatalf("编辑路口失败: %v", body5)
	}

	// 删除路口：应成功并解除设备挂接
	_, body6 := doReq(t, r, "DELETE", "/api/v1/crossings/"+strconv.FormatUint(uint64(cid), 10), "")
	if body6["code"].(float64) != 0 {
		t.Fatalf("删除路口失败: %v", body6)
	}
	var d model.Device
	model.DB.First(&d, devID)
	if d.CrossingID != nil {
		t.Errorf("删除路口后设备 crossing_id 应置空")
	}
}

func TestUpdateDevice_LocationPick_Fields(t *testing.T) {
	r := p0CrossingEngine(t)
	d := model.Device{HwID: "2001", Intersection: "原路口"}
	model.DB.Create(&d)

	// 地图拾取：更新经纬度 + 挂接路口/道路
	var roadID uint
	model.DB.Model(&model.Area{}).Where("area_type = ?", model.AreaRoad).First(&model.Area{})
	// 取种子道路
	road := model.Area{}
	model.DB.Where("area_type = ?", model.AreaRoad).First(&road)
	roadID = road.ID

	_, body := doReq(t, r, "PUT", "/api/v1/devices/"+uid(d.ID),
		`{"lat":30.5,"lng":120.8,"crossing_id":0,"road_id":`+strconv.FormatUint(uint64(roadID), 10)+`,"road_name":"宿州路"}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("设备地图拾取更新失败: %v", body)
	}
	var d2 model.Device
	model.DB.First(&d2, d.ID)
	if d2.Lat == nil || *d2.Lat != 30.5 || d2.Lng == nil || *d2.Lng != 120.8 {
		t.Errorf("经纬度拾取未生效: %+v", d2.Lat)
	}
	// crossing_id=0 表示置空
	if d2.CrossingID != nil {
		t.Errorf("crossing_id=0 应置空, 实际 %v", *d2.CrossingID)
	}
	if d2.RoadID == nil || *d2.RoadID != roadID {
		t.Errorf("road_id 挂接未生效")
	}
	if d2.RoadName != "宿州路" {
		t.Errorf("road_name 未生效")
	}
}

func TestUpdateDevice_NoArea_KeepsOld(t *testing.T) {
	r := p0CrossingEngine(t)
	cid := uint(7)
	d := model.Device{HwID: "3001", CrossingID: &cid}
	model.DB.Create(&d)
	// 只更新经纬度，不传区划字段（nil → 不改动）
	_, body := doReq(t, r, "PUT", "/api/v1/devices/"+uid(d.ID), `{"lat":31.1,"lng":121.1}`)
	if body["code"].(float64) != 0 {
		t.Fatalf("更新失败: %v", body)
	}
	var d2 model.Device
	model.DB.First(&d2, d.ID)
	if d2.CrossingID == nil || *d2.CrossingID != 7 {
		t.Errorf("未传区划字段应保持原挂接, 实际 %v", d2.CrossingID)
	}
}

// 占位避免 time 未使用（部分用例用到）
var _ = time.Now

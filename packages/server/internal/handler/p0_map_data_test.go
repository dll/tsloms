package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// mapRoutes 注册地图聚合接口（P0-5）
func mapRoutes(r *gin.Engine) {
	g := r.Group("/api/v1")
	g.GET("/map/crossing-data", GetCrossingMapData)
	g.GET("/map/road-data", GetRoadMapData)
}

// TestDeriveColorLevel 分级着色档位：green / yellow_low / yellow / orange / red
func TestDeriveColorLevel(t *testing.T) {
	cases := []struct {
		ratio float64
		want  string
	}{
		{0, "green"}, {0.1, "yellow_low"}, {0.5, "yellow"}, {0.8, "orange"}, {1, "red"}, {1.5, "red"},
	}
	for _, c := range cases {
		if got := deriveColorLevel(c.ratio); got != c.want {
			t.Errorf("deriveColorLevel(%v)=%s, want %s", c.ratio, got, c.want)
		}
	}
}

// TestComputCrossingPoly_Healthy_Fault_Offline 路口聚合：正常/故障/离线 设备 → 比例与等级
func TestComputCrossingPoly_Healthy_Fault_Offline(t *testing.T) {
	model.InitTestDB()
	area := model.Area{Code: "A1", Name: "长江中路", AreaType: model.AreaRoad, FullName: "安徽省合肥市长江中路"}
	model.DB.Create(&area)

	x := model.Crossing{Name: "长江路-宿州路口", RoadName: "长江中路", RoadID: &area.ID}
	model.DB.Create(&x)

	// 4 台设备：2 正常在线，1 有活跃故障，1 离线
	for i := 0; i < 4; i++ {
		model.DB.Create(&model.Device{HwID: uint32(7000 + i), CrossingID: &x.ID, OnlineStatus: i < 3})
	}
	// 一台设备带活跃故障
	model.DB.Create(&model.FaultRecord{
		DeviceHwID: 7000, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical",
		Status: model.FaultStatusOccurred,
	})

	p := computeCrossingPoly(&x)
	if p.DeviceTotal != 4 {
		t.Errorf("device_total=%d, want 4", p.DeviceTotal)
	}
	// 离线(3号) + 故障(0号) = 2 故障；在线且无故障 = 2 台（1号、2号）
	if p.FaultCount != 2 {
		t.Errorf("fault_count=%d, want 2", p.FaultCount)
	}
	if p.GreenRatio != 0.5 {
		t.Errorf("green_ratio=%v, want 0.5", p.GreenRatio)
	}
	if p.FaultRatio != 0.5 {
		t.Errorf("fault_ratio=%v, want 0.5", p.FaultRatio)
	}
	if p.Level != "yellow" {
		t.Errorf("level=%s, want yellow", p.Level)
	}
}

// TestGetCrossingMapData 地图路口聚合接口返回 list/ratio/level
func TestGetCrossingMapData(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	mapRoutes(r)

	x := model.Crossing{Name: "路口A", RoadName: "长江中路"}
	model.DB.Create(&x)
	model.DB.Create(&model.Device{HwID: 8001, CrossingID: &x.ID, OnlineStatus: true})

	code, body := doReq(t, r, "GET", "/api/v1/map/crossing-data", "")
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("map/crossing-data 失败 code=%d body=%v", code, body)
	}
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) == 0 {
		t.Fatal("应返回路口聚合")
	}
	first := list[0].(map[string]interface{})
	if first["level"] != "green" || first["fault_ratio"].(float64) != 0 {
		t.Errorf("正常路口应 green，got level=%v ratio=%v", first["level"], first["fault_ratio"])
	}
}

// TestGetRoadMapData 道路级聚合：多路口上卷
func TestGetRoadMapData(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	mapRoutes(r)

	// 同一道路两个路口，各 1 台在线设备
	var ids []uint
	for i := 0; i < 2; i++ {
		x := model.Crossing{Name: "路口", RoadName: "长江中路"}
		model.DB.Create(&x)
		ids = append(ids, x.ID)
	}
	for _, id := range ids {
		model.DB.Create(&model.Device{HwID: uint32(9000 + id), CrossingID: &id, OnlineStatus: true})
	}
	code, body := doReq(t, r, "GET", "/api/v1/map/road-data", "")
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("map/road-data 失败 code=%d", code)
	}
	list := body["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) == 0 {
		t.Fatal("应返回道路聚合")
	}
	first := list[0].(map[string]interface{})
	if first["crossing_count"].(float64) < 1 || first["device_total"].(float64) < 1 {
		t.Errorf("道路聚合应含路口与设备, got %v", first)
	}
}

// TestMapData_NotFound 过滤条件：road_id/street_id 空语义（不报错）
func TestMapData_EmptyFilters(t *testing.T) {
	r := gin.New()
	model.InitTestDB()
	mapRoutes(r)
	code, _ := doReq(t, r, "GET", "/api/v1/map/crossing-data?road_id=999", "")
	if code != http.StatusOK {
		t.Errorf("road_id 过滤应 200, got %d", code)
	}
}

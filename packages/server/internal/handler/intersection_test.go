package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func setupIntersectionEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/intersections", ListIntersections)
	return r
}

func TestListIntersections_Aggregates(t *testing.T) {
	r := setupIntersectionEngine(t)
	lat1 := 31.2304
	lng1 := 121.4737
	// 路口A 两台设备，一台在线；路口B 一台设备
	model.DB.Create(&model.Device{HwID: 1, Intersection: "人民路口", OnlineStatus: true, Lat: &lat1, Lng: &lng1})
	model.DB.Create(&model.Device{HwID: 2, Intersection: "人民路口", OnlineStatus: false, Lat: &lat1, Lng: &lng1})
	model.DB.Create(&model.Device{HwID: 3, Intersection: "中山路口", OnlineStatus: true})

	// 设备1 有活跃故障 -> 路口A 故障数 1
	model.DB.Create(&model.FaultRecord{DeviceHwID: 1, ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/intersections", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code float64 `json:"code"`
		Data struct {
			List []struct {
				Intersection string `json:"intersection"`
				DeviceTotal  int    `json:"device_total"`
				Online       int    `json:"online"`
				Offline      int    `json:"offline"`
				Fault        int    `json:"fault"`
				Lat          *float64 `json:"lat"`
				Lng          *float64 `json:"lng"`
			} `json:"list"`
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 || resp.Data.Total != 2 {
		t.Errorf("total = %d, 期望 2（两个路口）", resp.Data.Total)
	}

	// 找到人民路口
	for _, row := range resp.Data.List {
		if row.Intersection == "人民路口" {
			if row.DeviceTotal != 2 || row.Online != 1 || row.Fault != 1 {
				t.Errorf("人民路口统计 = total=%d online=%d fault=%d, 期望 2/1/1",
					row.DeviceTotal, row.Online, row.Fault)
			}
			if row.Lat == nil || *row.Lat != lat1 {
				t.Error("人民路口应带经纬度")
			}
		}
	}
}

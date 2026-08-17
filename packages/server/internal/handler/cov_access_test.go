package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tsloms/server/internal/mqtt"
)

// TestAccess_MockSendAndCSV 检测器接入：Mock 模拟发送 + CSV 导入回放
func TestAccess_MockSendAndCSV(t *testing.T) {
	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/access/status", DetectorAccessStatus)
	api.POST("/access/mock/send", MockSend)
	api.POST("/access/csv/import", CSVImport)

	// 注册一个真实处理器实例（复用研判链路；DB 为 nil 时链路安全跳过写库）
	mc := mqtt.NewMQTTClient()
	h := mqtt.NewHandler(mc)
	mqtt.RegisterActiveHandler(h)

	// 1) Mock 发送：缺 hw_id → 400
	code, _ := doReq(t, r, "POST", "/api/v1/access/mock/send", `{"cmd":"alarm"}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺 hw_id 应 400, got %d", code)
	}

	// 2) Mock 发送：合法 → 200 sent true
	code, body := doReq(t, r, "POST", "/api/v1/access/mock/send",
		`{"hw_id":9001,"cmd":"alarm","err_code":-5,"led_state":0,"current_r":600,"current_y":50,"current_g":40}`)
	if code != http.StatusOK {
		t.Fatalf("Mock 发送应 200, got %d: %v", code, body)
	}
	if data, _ := body["data"].(map[string]interface{}); data == nil || data["sent"] != true {
		t.Errorf("应返回 sent=true, got %v", body["data"])
	}

	// 3) CSV 导入：content 模式
	code, body = doReq(t, r, "POST", "/api/v1/access/csv/import", `{"content":"hw_id,cmd,err_code,led_state,current_r,current_y,current_g\n9002,alarm,-4,0,500,300,20\n9003,checkin,-1,0,0,0,0\n"}`)
	if code != http.StatusOK {
		t.Fatalf("CSV 导入应 200, got %d: %v", code, body)
	}
	d := body["data"].(map[string]interface{})
	if d["total"].(float64) != 2 {
		t.Errorf("CSV total 应 2, got %v", d["total"])
	}
	if d["failed"].(float64) != 0 {
		t.Errorf("CSV failed 应 0, got %v, errs=%v", d["failed"], d["errors"])
	}

	// 4) CSV 导入：rows 模式 + 错行
	code, body = doReq(t, r, "POST", "/api/v1/access/csv/import",
		`{"rows":[{"hw_id":9005,"cmd":"alarm","err_code":-8},{"hw_id":0,"err_code":-1},{"hw_id":9006,"cmd":"power_on"}]}`)
	if code != http.StatusOK {
		t.Fatalf("CSV rows 应 200, got %d", code)
	}
	d = body["data"].(map[string]interface{})
	if d["imported"].(float64) != 2 {
		t.Errorf("rows imported 应 2, got %v", d["imported"])
	}
	if d["failed"].(float64) != 1 {
		t.Errorf("rows failed 应 1, got %v", d["failed"])
	}

	// 5) Mock 发送：未注册活跃处理器时 → 仍应返回（幂等），此处已注册，直接验证 status
	code, _ = doReq(t, r, "GET", "/api/v1/access/status", "")
	if code != http.StatusOK {
		t.Errorf("status 应 200, got %d", code)
	}
}

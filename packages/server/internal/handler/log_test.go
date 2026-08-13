package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func TestListOperationLogs_ReturnsAudit(t *testing.T) {
	db := model.InitTestDB()
	db.Create(&model.OperationLog{
		UserID: 1, Username: "admin", Action: model.OpLogin,
		Target: "auth/login", Detail: "用户登录", IP: "127.0.0.1",
	})
	db.Create(&model.OperationLog{
		UserID: 1, Username: "admin", Action: model.OpUpdate,
		Target: "device/1", Detail: "更新设备", IP: "127.0.0.1",
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/logs/operations", ListOperationLogs)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/logs/operations", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			List  []model.OperationLog `json:"list"`
			Total int64                `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("code = %d, 期望 0", resp.Code)
	}
	if resp.Data.Total != 2 {
		t.Errorf("total = %d, 期望 2", resp.Data.Total)
	}
	if len(resp.Data.List) != 2 {
		t.Errorf("list 长度 = %d, 期望 2", len(resp.Data.List))
	}
}

func TestListPacketLogs_FilterByValid(t *testing.T) {
	db := model.InitTestDB()
	db.Create(&model.PacketLog{DeviceHwID: 1, CmdType: 0x00, CmdSeq: 1, Valid: true})
	db.Create(&model.PacketLog{DeviceHwID: 2, CmdType: 0x01, CmdSeq: 2, Valid: false})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/logs/packets", ListPacketLogs)

	// 只筛有效报文
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/logs/packets?valid=true", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			List []model.PacketLog `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.List) != 1 || !resp.Data.List[0].Valid {
		t.Errorf("valid=true 应返回 1 条有效报文, 实际 %d 条", len(resp.Data.List))
	}
}

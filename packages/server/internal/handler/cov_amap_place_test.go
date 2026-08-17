package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tsloms/server/internal/config"
)

// TestAmapPlaceSearch_NoKey 未配置 AMAP_WEB_KEY 时返回 fallback，前端降级本地搜索
func TestAmapPlaceSearch_NoKey(t *testing.T) {
	config.ResetCache()
	_ = config.Load // 确保默认无 key（测试环境 amap key 默认空）

	r := gin.New()
	api := r.Group("/api/v1")
	api.GET("/proxy/amap/place", AmapPlaceSearch)

	// 缺关键字 → 400
	code, _ := doReq(t, r, "GET", "/api/v1/proxy/amap/place", "")
	if code != http.StatusBadRequest {
		t.Errorf("缺 kw 应 400, got %d", code)
	}

	// 无 key → 200 fallback:true，pois 为空
	code, body := doReq(t, r, "GET", "/api/v1/proxy/amap/place?kw=人民路", "")
	if code != http.StatusOK {
		t.Errorf("无key应200 fallback, got %d", code)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺 data: %v", body)
	}
	if data["fallback"] != true {
		t.Errorf("应标记 fallback, got %v", data["fallback"])
	}
	if pois, _ := data["pois"].([]interface{}); pois == nil || len(pois) != 0 {
		t.Errorf("无key时 pois 应为空")
	}
	msg, _ := data["message"].(string)
	if !strings.Contains(msg, "AMAP_WEB_KEY") {
		t.Errorf("message 应提示未配置key")
	}
}

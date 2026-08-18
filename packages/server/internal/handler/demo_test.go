package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func demoEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/demo/status", DemoStatus)
		g.POST("/demo/start", DemoStart)
		g.POST("/demo/end", DemoEnd)
	}
	return r
}

// TestDemoStart_CreatesWarnings 系统演示生成数据时应同步生成预警记录（闭环：设备→故障→预警→处理），
// 且清理时一并删除，避免演示数据残留。
func TestDemoStart_CreatesWarnings(t *testing.T) {
	r := demoEngine(t)

	// 开始演示
	code, _ := doReq(t, r, "POST", "/api/v1/demo/start?n=3", "")
	if code != 200 {
		t.Fatalf("demo/start code=%d", code)
	}

	// 生成的预警数应>0，且均为演示段设备（DEMO 前缀）
	var warnings []model.Warning
	model.DB.Where("device_hw_id LIKE ?", "DEMO%").Find(&warnings)
	if len(warnings) == 0 {
		t.Fatalf("演示后未生成任何预警记录，期望>0")
	}
	// 每条预警 source=fault 且关联 fault_id（闭合链路）
	for _, w := range warnings {
		if w.Source != model.WarningSourceFault {
			t.Errorf("预警 %d source=%q 期望 %q", w.ID, w.Source, model.WarningSourceFault)
		}
		if w.FaultID == nil {
			t.Errorf("预警 %d 未关联 fault_id（链路未闭合）", w.ID)
		}
		if w.DealState != model.WarningDealUnhandled {
			t.Errorf("预警 %d deal_state=%q", w.ID, w.DealState)
		}
	}

	// DemoStatus 应统计到预警数
	code, body := doReq(t, r, "GET", "/api/v1/demo/status", "")
	if code != 200 {
		t.Fatalf("demo/status code=%d", code)
	}
	data, _ := body["data"].(map[string]interface{})
	wn, _ := data["warnings"].(float64)
	if wn <= 0 {
		t.Errorf("demo/status 期望 warnings>0，实际=%v", data["warnings"])
	}

	// 结束演示：预警应一并清理
	code, _ = doReq(t, r, "POST", "/api/v1/demo/end", "")
	if code != 200 {
		t.Fatalf("demo/end code=%d", code)
	}
	var remain int64
	model.DB.Model(&model.Warning{}).Where("device_hw_id LIKE ?", "DEMO%").Count(&remain)
	if remain != 0 {
		t.Fatalf("demo/end 后演示预警残留 %d 条", remain)
	}
}

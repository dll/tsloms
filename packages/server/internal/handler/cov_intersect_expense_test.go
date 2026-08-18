package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func intersectExpenseEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/intersections", ListIntersections)
		g.PUT("/intersections/rename", RenameIntersection)
		g.PUT("/intersections/location", SetIntersectionLocation)
		g.DELETE("/intersections/clear", ClearIntersection)
		g.GET("/expenses", ListRepairExpenses)
		g.GET("/expenses/stats", ExpenseStats)
		g.POST("/expenses", SaveRepairExpense)
		g.PUT("/expenses/:id", SaveRepairExpense)
		g.PUT("/expenses/:id/confirm", ConfirmRepairExpense)
		g.DELETE("/expenses/:id", DeleteRepairExpense)
	}
	return r
}

func TestIntersection_Rename(t *testing.T) {
	r := intersectExpenseEngine(t)
	model.DB.Create(&model.Device{HwID: "1", Intersection: "旧路口", OnlineStatus: true})
	model.DB.Create(&model.Device{HwID: "2", Intersection: "旧路口", OnlineStatus: false})
	// 成功重命名
	code, body := doReq(t, r, "PUT", "/api/v1/intersections/rename", `{"old":"旧路口","new":"新路口"}`)
	mustOK(t, code, body, "重命名")
	if body["data"].(map[string]interface{})["affected"].(float64) != 2 {
		t.Errorf("应影响 2 台设备")
	}
	// 缺参数
	code, _ = doReq(t, r, "PUT", "/api/v1/intersections/rename", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 新旧相同
	code, _ = doReq(t, r, "PUT", "/api/v1/intersections/rename", `{"old":"新路口","new":"新路口"}`)
	if code != http.StatusBadRequest {
		t.Errorf("新旧相同应 400, got %d", code)
	}
}

func TestIntersection_LocationAndClear(t *testing.T) {
	r := intersectExpenseEngine(t)
	model.DB.Create(&model.Device{HwID: "3", Intersection: "定位路口"})
	// 设经纬度
	code, _ := doReq(t, r, "PUT", "/api/v1/intersections/location", `{"intersection":"定位路口","lat":31.2,"lng":121.5}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "设经纬度")
	// 非法经纬度
	code, _ = doReq(t, r, "PUT", "/api/v1/intersections/location", `{"intersection":"定位路口","lat":200,"lng":121}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法经纬度应 400, got %d", code)
	}
	// 缺参数
	code, _ = doReq(t, r, "PUT", "/api/v1/intersections/location", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 清空路口
	code, _ = doReq(t, r, "DELETE", "/api/v1/intersections/clear?intersection=定位路口", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "清空路口")
	// 缺 intersection 参数
	code, _ = doReq(t, r, "DELETE", "/api/v1/intersections/clear", "")
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
}

func TestExpense_ListAndStats(t *testing.T) {
	r := intersectExpenseEngine(t)
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE1", Type: model.ExpenseTypeMaterial, Amount: 100, DeviceHwID: "1", CreatedAt: now()})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE2", Type: model.ExpenseTypeLabor, Amount: 200, DeviceHwID: "1", Confirmed: true, CreatedAt: now()})
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE3", Type: model.ExpenseTypeTraffic, Amount: 50, DeviceHwID: "2", CreatedAt: now()})

	// 列表 + 筛选
	for _, q := range []string{"", "?device_hw_id=1", "?work_order_id=1", "?type=material", "?from=2026-08-01", "?from=2026-08-01&to=2026-08-30", "?confirmed=true"} {
		code, body := doReq(t, r, "GET", "/api/v1/expenses"+q, "")
		mustOK(t, code, body, "费用列表 "+q)
	}
	// 统计
	code, body := doReq(t, r, "GET", "/api/v1/expenses/stats", "")
	mustOK(t, code, body, "费用统计")
	if body["data"].(map[string]interface{})["total_amount"].(float64) != 350 {
		t.Errorf("total_amount 期望 350")
	}
	if body["data"].(map[string]interface{})["total_count"].(float64) != 3 {
		t.Errorf("total_count 期望 3")
	}
}

func TestExpense_Save(t *testing.T) {
	r := intersectExpenseEngine(t)
	wo := model.WorkOrder{OrderNo: "WOexp", DeviceHwID: "7", Status: model.WorkOrderStatusPending}
	model.DB.Create(&wo)
	// 新增
	code, body := doReq(t, r, "POST", "/api/v1/expenses", `{"type":"material","amount":150,"work_order_id":`+uid(wo.ID)+`,"work_date":"2026-08-01"}`)
	mustOK(t, code, body, "新增费用")
	eid := uint(body["data"].(map[string]interface{})["expense"].(map[string]interface{})["id"].(float64))
	// 新增应自动带出工单设备ID
	// 更新
	code, _ = doReq(t, r, "PUT", "/api/v1/expenses/"+uid(eid), `{"type":"labor","amount":300,"description":"改"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新费用")
	// 缺 type/amount
	code, _ = doReq(t, r, "POST", "/api/v1/expenses", `{"type":"material"}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺amount应 400, got %d", code)
	}
	// 非法类型
	code, _ = doReq(t, r, "POST", "/api/v1/expenses", `{"type":"bogus","amount":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法类型应 400, got %d", code)
	}
	// 负金额
	code, _ = doReq(t, r, "POST", "/api/v1/expenses", `{"type":"material","amount":-1}`)
	if code != http.StatusBadRequest {
		t.Errorf("负金额应 400, got %d", code)
	}
	// 非法日期
	code, _ = doReq(t, r, "POST", "/api/v1/expenses", `{"type":"material","amount":1,"work_date":"bad-date"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法日期应 400, got %d", code)
	}
	// 关联工单不存在
	code, _ = doReq(t, r, "POST", "/api/v1/expenses", `{"type":"material","amount":1,"work_order_id":99999}`)
	if code != http.StatusBadRequest {
		t.Errorf("工单不存在应 400, got %d", code)
	}
	// 更新不存在（需在 body 中带 id 触发更新分支）
	code, _ = doReq(t, r, "PUT", "/api/v1/expenses/99999", `{"id":99999,"type":"material","amount":1}`)
	if code != http.StatusNotFound {
		t.Errorf("更新不存在应 404, got %d", code)
	}
}

func TestExpense_ConfirmDelete(t *testing.T) {
	r := intersectExpenseEngine(t)
	model.DB.Create(&model.RepairExpense{ExpenseNo: "FE9", Type: model.ExpenseTypeOther, Amount: 88, CreatedAt: now()})
	var e model.RepairExpense
	model.DB.First(&e)
	// 确认
	code, _ := doReq(t, r, "PUT", "/api/v1/expenses/"+uid(e.ID)+"/confirm", `{"confirmed":true}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "确认费用")
	// 确认不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/expenses/99999/confirm", `{"confirmed":true}`)
	if code != http.StatusNotFound {
		t.Errorf("确认不存在应 404, got %d", code)
	}
	// 非法ID确认
	code, _ = doReq(t, r, "PUT", "/api/v1/expenses/abc/confirm", `{"confirmed":true}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法确认ID应 400, got %d", code)
	}
	// 删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/expenses/"+uid(e.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除费用")
	// 删除不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/expenses/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法删除ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/expenses/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
}

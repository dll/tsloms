package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func invEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/inv/materials", ListMaterialsV2)
		g.GET("/inv/materials/stats", MaterialStats)
		g.POST("/inv/materials", SaveMaterial)
		g.PUT("/inv/materials/:id", SaveMaterial)
		g.DELETE("/inv/materials/:id", DeleteMaterialV2)
		g.GET("/inv/stocks", ListMaterialStocks)
		g.POST("/inv/stocks/adjust", AdjustMaterialStock)
		g.POST("/inv/stocks/use", UseMaterialStock)
	}
	return r
}

func invSeed(t *testing.T) model.Material {
	t.Helper()
	m := model.Material{Code: "LAMP001", Name: "红灯灯珠", Category: "灯泡", Unit: "支", UnitPrice: 5, Stock: 3, Threshold: 5, Status: "active"}
	model.DB.Create(&m)
	return m
}

func TestInv_ListAndStats(t *testing.T) {
	r := invEngine(t)
	invSeed(t)
	for _, q := range []string{"", "?keyword=红灯", "?category=灯泡", "?status=active", "?low_stock=1", "?device_hw_id=1"} {
		code, body := doReq(t, r, "GET", "/api/v1/inv/materials"+q, "")
		mustOK(t, code, body, "物料列表 "+q)
	}
	// stats
	code, body := doReq(t, r, "GET", "/api/v1/inv/materials/stats", "")
	mustOK(t, code, body, "物料统计")
	if body["data"].(map[string]interface{})["material_count"].(float64) != 1 {
		t.Errorf("material_count 期望 1")
	}
	if body["data"].(map[string]interface{})["low_stock_count"].(float64) != 1 {
		t.Errorf("low_stock_count 期望 1(stock3<=threshold5)")
	}
}

func TestInv_Delete(t *testing.T) {
	r := invEngine(t)
	m := invSeed(t)
	// 无库存且合法删除
	code, _ := doReq(t, r, "DELETE", "/api/v1/inv/materials/"+uid(m.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除物料")
	// 不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/inv/materials/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/inv/materials/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
}

func TestInv_Stocks(t *testing.T) {
	r := invEngine(t)
	m := invSeed(t)
	// 入库流水
	model.DB.Create(&model.MaterialStock{MaterialID: m.ID, MaterialName: m.Name, Type: model.StockTypeIn, Quantity: 10})
	// 列表
	code, body := doReq(t, r, "GET", "/api/v1/inv/stocks?material_id="+uid(m.ID)+"&type=in", "")
	mustOK(t, code, body, "库存流水")
	// adjust 库存（需 type+quantity）
	code, body = doReq(t, r, "POST", "/api/v1/inv/stocks/adjust", `{"material_id":`+uid(m.ID)+`,"type":"adjust","quantity":5,"note":"盘点"}`)
	mustOK(t, code, body, "调整库存")
	// 非法 type
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/adjust", `{"material_id":`+uid(m.ID)+`,"type":"bogus","quantity":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法type应 400, got %d", code)
	}
	// 缺参数
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/adjust", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 调整不存在物料
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/adjust", `{"material_id":99999,"type":"in","quantity":1}`)
	if code != http.StatusNotFound {
		t.Errorf("调整不存在物料应 404, got %d", code)
	}
	// use 库存（需存在工单）
	wo := model.WorkOrder{OrderNo: "WOSTK", Status: model.WorkOrderStatusPending}
	model.DB.Create(&wo)
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/use", `{"material_id":`+uid(m.ID)+`,"quantity":2,"work_order_id":`+uid(wo.ID)+`}`)
	if code != http.StatusOK {
		t.Errorf("领用 code=%d", code)
	}
	// use 超库存 → 400
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/use", `{"material_id":`+uid(m.ID)+`,"quantity":999,"work_order_id":`+uid(wo.ID)+`}`)
	if code != http.StatusBadRequest {
		t.Errorf("超库存领用应 400, got %d", code)
	}
	// use 工单不存在 → 404
	code, _ = doReq(t, r, "POST", "/api/v1/inv/stocks/use", `{"material_id":`+uid(m.ID)+`,"quantity":1,"work_order_id":99999}`)
	if code != http.StatusNotFound {
		t.Errorf("工单不存在应 404, got %d", code)
	}
}

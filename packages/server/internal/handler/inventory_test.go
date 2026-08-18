package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// newInvEngine 构造库存/采购/费用测试引擎（全部路由，共享同一测试库）
func newInvEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	api := r.Group("/inv")
	{
		api.GET("/materials", ListMaterialsV2)
		api.POST("/materials", SaveMaterial)
		api.PUT("/materials/:id", SaveMaterial)
		api.DELETE("/materials/:id", DeleteMaterialV2)
		api.GET("/materials/stats", MaterialStats)
		api.GET("/stocks", ListMaterialStocks)
		api.POST("/stocks/adjust", AdjustMaterialStock)
		api.POST("/stocks/use", UseMaterialStock)
	}
	pr := r.Group("/purchases")
	{
		pr.GET("", ListPurchaseOrders)
		pr.POST("", CreatePurchaseOrder)
		pr.GET("/:id", GetPurchaseOrder)
		pr.POST("/:id/receive", ReceivePurchase)
		pr.POST("/:id/cancel", CancelPurchase)
		pr.DELETE("/:id", DeletePurchase)
	}
	ex := r.Group("/expenses")
	{
		ex.GET("", ListRepairExpenses)
		ex.GET("/stats", ExpenseStats)
		ex.POST("", SaveRepairExpense)
		ex.PUT("/:id", SaveRepairExpense)
		ex.PUT("/:id/confirm", ConfirmRepairExpense)
	}
	sup := r.Group("/suppliers")
	{
		sup.GET("", ListSuppliers)
		sup.POST("", SaveSupplier)
		sup.PUT("/:id", SaveSupplier)
		sup.DELETE("/:id", DeleteSupplier)
	}
	return r
}

func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return body
}

func TestMaterialCreateAndStockAdjust(t *testing.T) {
	r := newInvEngine(t)

	// 新增物料
	w := doJSON(r, "POST", "/inv/materials", map[string]interface{}{
		"code": "LED-RED-01", "name": "红灯灯珠", "category": "灯泡",
		"unit": "个", "unit_price": 25.5, "stock": 10, "threshold": 5,
	})
	body := parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("新增物料失败: %v", body["message"])
	}
	var m model.Material
	if err := model.DB.First(&m, "code = ?", "LED-RED-01").Error; err != nil {
		t.Fatalf("物料未写入: %v", err)
	}
	if m.Stock != 10 {
		t.Fatalf("初始库存应为 10, got %d", m.Stock)
	}

	// 盘亏 3 个
	w = doJSON(r, "POST", "/inv/stocks/adjust", map[string]interface{}{
		"material_id": m.ID, "type": "loss", "quantity": 3, "note": "报损",
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("盘亏失败: %v", body["message"])
	}
	model.DB.First(&m, m.ID)
	if m.Stock != 7 {
		t.Fatalf("盘亏后库存应为 7, got %d", m.Stock)
	}

	// 超额盘亏应被拒绝（库存不足）
	w = doJSON(r, "POST", "/inv/stocks/adjust", map[string]interface{}{
		"material_id": m.ID, "type": "loss", "quantity": 100,
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("超额盘亏应失败, got %v", body["code"])
	}

	// 流水数量校验
	var stocks []model.MaterialStock
	model.DB.Where("material_id = ?", m.ID).Order("id ASC").Find(&stocks)
	if len(stocks) != 2 {
		t.Fatalf("应有 2 条流水(初始+盘亏), got %d", len(stocks))
	}
	if stocks[1].Quantity != -3 {
		t.Fatalf("盘亏流水应为 -3, got %d", stocks[1].Quantity)
	}
}

func TestMaterialDuplicateCodeRejected(t *testing.T) {
	r := newInvEngine(t)
	doJSON(r, "POST", "/inv/materials", map[string]interface{}{"code": "C1", "name": "A"})
	w := doJSON(r, "POST", "/inv/materials", map[string]interface{}{"code": "C1", "name": "B"})
	body := parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("重复编码应被拒绝, got %v", body["code"])
	}
}

func TestPurchaseReceiveUpdatesStock(t *testing.T) {
	r := newInvEngine(t)

	// 物料 + 供应商
	doJSON(r, "POST", "/inv/materials", map[string]interface{}{"code": "PW-01", "name": "电源", "unit": "个", "unit_price": 120, "stock": 0})
	var mat model.Material
	model.DB.First(&mat, "code = ?", "PW-01")
	supplier := model.Supplier{Name: "测试供应商"}
	model.DB.Create(&supplier)

	// 创建采购单
	w := doJSON(r, "POST", "/purchases", map[string]interface{}{
		"supplier_id": supplier.ID,
		"items": []map[string]interface{}{
			{"material_id": mat.ID, "quantity": 20, "price": 110},
		},
	})
	body := parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("创建采购单失败: %v", body)
	}
	po := model.PurchaseOrder{}
	model.DB.Last(&po)
	if po.Status != model.PurchaseStatusDraft {
		t.Fatalf("初始状态应为 draft, got %s", po.Status)
	}
	if po.TotalAmount != 2200 {
		t.Fatalf("采购总额应为 2200, got %v", po.TotalAmount)
	}

	// 部分入库 8 个
	var item model.PurchaseOrderItem
	model.DB.First(&item, "order_id = ?", po.ID)
	w = doJSON(r, "POST", "/purchases/1/receive", map[string]interface{}{
		"items": []map[string]interface{}{{"item_id": item.ID, "quantity": 8}},
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("部分入库失败: %v", body)
	}
	model.DB.First(&mat, mat.ID)
	if mat.Stock != 8 {
		t.Fatalf("部分入库后库存应为 8, got %d", mat.Stock)
	}
	model.DB.First(&po, po.ID)
	if po.Status != model.PurchaseStatusPartial {
		t.Fatalf("应转为 partial, got %s", po.Status)
	}

	// 剩余 12 个入库 → 完成
	w = doJSON(r, "POST", "/purchases/1/receive", map[string]interface{}{
		"items": []map[string]interface{}{{"item_id": item.ID, "quantity": 12}},
	})
	body = parseBody(t, w)
	model.DB.First(&po, po.ID)
	if po.Status != model.PurchaseStatusCompleted {
		t.Fatalf("入库完毕应转为 completed, got %s", po.Status)
	}
	model.DB.First(&mat, mat.ID)
	if mat.Stock != 20 {
		t.Fatalf("全部入库后库存应为 20, got %d", mat.Stock)
	}

	// 超量入库应被拒
	w = doJSON(r, "POST", "/purchases/1/receive", map[string]interface{}{
		"items": []map[string]interface{}{{"item_id": item.ID, "quantity": 1}},
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("已完成采购单再入库应被拒, got %v", body["code"])
	}
}

func TestExpenseTypesAndConfirm(t *testing.T) {
	r := newInvEngine(t)

	// 四种费用类型
	for i, typ := range []string{model.ExpenseTypeMaterial, model.ExpenseTypeLabor, model.ExpenseTypeTraffic, model.ExpenseTypeOther} {
		w := doJSON(r, "POST", "/expenses", map[string]interface{}{
			"type": typ, "amount": 100 + i, "device_hw_id": "9", "work_date": "2026-08-15",
		})
		body := parseBody(t, w)
		if code, _ := body["code"].(float64); code != 0 {
			t.Fatalf("新增费用 %s 失败: %v", typ, body)
		}
	}

	// 统计
	w := doJSON(r, "GET", "/expenses/stats", nil)
	body := parseBody(t, w)
	data, _ := body["data"].(map[string]interface{})
	if data == nil {
		t.Fatal("统计返回缺 data")
	}
	if total, _ := data["total_amount"].(float64); total != 406 {
		t.Fatalf("费用总额应为 406, got %v", total)
	}

	// 非法类型拒绝
	w = doJSON(r, "POST", "/expenses", map[string]interface{}{"type": "bogus", "amount": 1})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("非法费用类型应被拒, got %v", body["code"])
	}

	// 确认入账
	var e model.RepairExpense
	model.DB.First(&e)
	w = doJSON(r, "PUT", "/expenses/1/confirm", map[string]interface{}{"confirmed": true})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("确认失败: %v", body["message"])
	}
	model.DB.First(&e, e.ID)
	if !e.Confirmed {
		t.Fatal("确认未生效")
	}
}

// TestWorkOrderMaterialUse 工单领料出库：扣减库存 + 写 type=use 流水并关联工单
func TestWorkOrderMaterialUse(t *testing.T) {
	r := newInvEngine(t)

	// 物料（初始库存 10）
	doJSON(r, "POST", "/inv/materials", map[string]interface{}{
		"code": "LED-BULB", "name": "灯珠", "unit": "个", "stock": 10, "unit_price": 5,
	})
	var m model.Material
	model.DB.First(&m, "code = ?", "LED-BULB")

	// 构造一张工单
	wo := model.WorkOrder{
		OrderNo:    model.NextOrderNo(model.DB),
		DeviceHwID: "7",
		Status:     model.WorkOrderStatusProcessing,
	}
	model.DB.Create(&wo)

	// 领料出库 3 个
	w := doJSON(r, "POST", "/inv/stocks/use", map[string]interface{}{
		"material_id":   m.ID,
		"quantity":      3,
		"work_order_id": wo.ID,
		"note":          "换灯珠",
	})
	body := parseBody(t, w)
	if code, _ := body["code"].(float64); code != 0 {
		t.Fatalf("工单领料失败: %v", body["message"])
	}
	model.DB.First(&m, m.ID)
	if m.Stock != 7 {
		t.Fatalf("领料后库存应为 7, got %d", m.Stock)
	}

	// 流水：type=use、数量为负、关联工单与设备
	var s model.MaterialStock
	model.DB.Where("material_id = ?", m.ID).Order("id DESC").First(&s)
	if s.Type != model.StockTypeUse {
		t.Fatalf("流水类型应为 use, got %s", s.Type)
	}
	if s.Quantity != -3 {
		t.Fatalf("领料流水数量应为 -3, got %d", s.Quantity)
	}
	if s.WorkOrderID == nil || *s.WorkOrderID != wo.ID {
		t.Fatalf("流水未关联工单, work_order_id=%v", s.WorkOrderID)
	}
	if s.RefType != "repair" {
		t.Fatalf("流水引用类型应为 repair, got %s", s.RefType)
	}

	// 领料应自动生成耗材费用单（关联工单/设备，金额=数量*单价）
	var e model.RepairExpense
	model.DB.Where("work_order_id = ?", wo.ID).Where("type = ?", model.ExpenseTypeMaterial).First(&e)
	if e.ID == 0 {
		t.Fatalf("领料未自动生成耗材费用单")
	}
	if e.DeviceHwID != wo.DeviceHwID {
		t.Fatalf("费用设备ID应为 %s, got %s", wo.DeviceHwID, e.DeviceHwID)
	}
	if e.Amount != float64(3*m.UnitPrice) {
		t.Fatalf("费用金额应为 %.2f, got %.2f", float64(3*m.UnitPrice), e.Amount)
	}
	if e.ExpenseNo == "" {
		t.Fatalf("费用单号为空")
	}

	// 超额领料应被拒（库存不足）
	w = doJSON(r, "POST", "/inv/stocks/use", map[string]interface{}{
		"material_id": m.ID, "quantity": 100, "work_order_id": wo.ID,
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("超额领料应被拒, got %v", body["code"])
	}

	// 不存在工单应被拒
	w = doJSON(r, "POST", "/inv/stocks/use", map[string]interface{}{
		"material_id": m.ID, "quantity": 1, "work_order_id": 99999,
	})
	body = parseBody(t, w)
	if code, _ := body["code"].(float64); code != -1 {
		t.Fatalf("不存在工单应被拒, got %v", body["code"])
	}
}

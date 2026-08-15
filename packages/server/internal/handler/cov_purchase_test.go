package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

func purchaseEngine(t *testing.T) *gin.Engine {
	t.Helper()
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/purchases", ListPurchaseOrders)
		g.GET("/purchases/:id", GetPurchaseOrder)
		g.POST("/purchases", CreatePurchaseOrder)
		g.POST("/purchases/:id/cancel", CancelPurchase)
		g.DELETE("/purchases/:id", DeletePurchase)
	}
	return r
}

func TestPurchase_ListGet(t *testing.T) {
	r := purchaseEngine(t)
	sup := model.Supplier{Name: "供货商X", Status: "active"}
	model.DB.Create(&sup)
	po := model.PurchaseOrder{OrderNo: "PO1", SupplierID: sup.ID, Status: model.PurchaseStatusDraft, TotalAmount: 100}
	model.DB.Create(&po)
	model.DB.Create(&model.PurchaseOrderItem{OrderID: po.ID, MaterialID: 1, MaterialName: "灯珠", Quantity: 10, Price: 10, Amount: 100})

	// 列表 + 筛选
	for _, q := range []string{"", "?order_no=PO", "?supplier_id=" + uid(sup.ID), "?status=draft"} {
		code, body := doReq(t, r, "GET", "/api/v1/purchases"+q, "")
		mustOK(t, code, body, "采购列表 "+q)
	}
	// 详情
	code, body := doReq(t, r, "GET", "/api/v1/purchases/"+uid(po.ID), "")
	mustOK(t, code, body, "采购详情")
	if body["data"].(map[string]interface{})["purchase"].(map[string]interface{})["supplier_name"] != "供货商X" {
		t.Errorf("supplier_name 应带出")
	}
	// 详情不存在
	code, _ = doReq(t, r, "GET", "/api/v1/purchases/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("详情不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "GET", "/api/v1/purchases/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 无供应商名（供应商不存在）
	r2 := purchaseEngine(t)
	po2 := model.PurchaseOrder{OrderNo: "PO2", SupplierID: 999, Status: model.PurchaseStatusDraft}
	model.DB.Create(&po2)
	code, body = doReq(t, r2, "GET", "/api/v1/purchases/"+uid(po2.ID), "")
	mustOK(t, code, body, "无供应商详情")
	if body["data"].(map[string]interface{})["purchase"].(map[string]interface{})["supplier_name"] != "" {
		t.Errorf("供应商不存在 supplier_name 应空")
	}
}

func TestPurchase_CancelDelete(t *testing.T) {
	r := purchaseEngine(t)
	sup := model.Supplier{Name: "供货商Y", Status: "active"}
	model.DB.Create(&sup)
	// 草稿 → 取消
	draft := model.PurchaseOrder{OrderNo: "POdraft", SupplierID: sup.ID, Status: model.PurchaseStatusDraft}
	model.DB.Create(&draft)
	code, _ := doReq(t, r, "POST", "/api/v1/purchases/"+uid(draft.ID)+"/cancel", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "取消草稿")
	// 已取消再次取消 → 400
	code, _ = doReq(t, r, "POST", "/api/v1/purchases/"+uid(draft.ID)+"/cancel", "")
	if code != http.StatusBadRequest {
		t.Errorf("已取消再取消应 400, got %d", code)
	}
	// 取消不存在
	code, _ = doReq(t, r, "POST", "/api/v1/purchases/99999/cancel", "")
	if code != http.StatusNotFound {
		t.Errorf("取消不存在应 404, got %d", code)
	}
	// 非法ID取消
	code, _ = doReq(t, r, "POST", "/api/v1/purchases/abc/cancel", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法取消ID应 400, got %d", code)
	}

	// 删除：非草稿（已取消）不可删
	code, _ = doReq(t, r, "DELETE", "/api/v1/purchases/"+uid(draft.ID), "")
	if code != http.StatusBadRequest {
		t.Errorf("非草稿删除应 400, got %d", code)
	}
	// 删除草稿成功
	draft2 := model.PurchaseOrder{OrderNo: "POd2", SupplierID: sup.ID, Status: model.PurchaseStatusDraft}
	model.DB.Create(&draft2)
	code, _ = doReq(t, r, "DELETE", "/api/v1/purchases/"+uid(draft2.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除草稿")
	// 删除不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/purchases/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法删除ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/purchases/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
}

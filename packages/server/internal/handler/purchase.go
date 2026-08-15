package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
)

// PurchaseView 采购单视图（含供应商名）
func purchaseView(po model.PurchaseOrder, supplierName string, items []model.PurchaseOrderItem) gin.H {
	itemViews := make([]gin.H, 0, len(items))
	for _, it := range items {
		itemViews = append(itemViews, gin.H{
			"id": it.ID, "material_id": it.MaterialID, "material_name": it.MaterialName,
			"quantity": it.Quantity, "price": it.Price, "amount": it.Amount, "received_qty": it.ReceivedQty,
		})
	}
	return gin.H{
		"id": po.ID, "order_no": po.OrderNo, "supplier_id": po.SupplierID, "supplier_name": supplierName,
		"status": po.Status, "total_amount": po.TotalAmount, "received_at": po.ReceivedAt,
		"operator": po.Operator, "note": po.Note, "created_at": po.CreatedAt,
		"items": itemViews,
	}
}

// ListPurchaseOrders 采购单列表（分页，可按单号/供应商/状态筛选）
func ListPurchaseOrders(c *gin.Context) {
	page, pageSize := paginate(c)
	query := model.DB.Model(&model.PurchaseOrder{})
	if no := c.Query("order_no"); no != "" {
		query = query.Where("order_no LIKE ?", "%"+no+"%")
	}
	if sid := c.Query("supplier_id"); sid != "" {
		query = query.Where("supplier_id = ?", sid)
	}
	if st := c.Query("status"); st != "" {
		query = query.Where("status = ?", st)
	}
	var total int64
	query.Count(&total)
	var list []model.PurchaseOrder
	query.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	items := make([]gin.H, 0, len(list))
	for _, po := range list {
		var s model.Supplier
		sName := ""
		if err := model.DB.First(&s, po.SupplierID).Error; err == nil {
			sName = s.Name
		}
		items = append(items, purchaseView(po, sName, nil))
	}
	ok(c, gin.H{"list": items, "total": total, "page": page, "page_size": pageSize})
}

// GetPurchaseOrder 采购单详情（含明细）
func GetPurchaseOrder(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "采购单ID无效")
		return
	}
	var po model.PurchaseOrder
	if err := model.DB.First(&po, id).Error; err != nil {
		notFound(c, "采购单不存在")
		return
	}
	var s model.Supplier
	sName := ""
	if err := model.DB.First(&s, po.SupplierID).Error; err == nil {
		sName = s.Name
	}
	var items []model.PurchaseOrderItem
	model.DB.Where("order_id = ?", po.ID).Order("id ASC").Find(&items)
	ok(c, gin.H{"purchase": purchaseView(po, sName, items)})
}

// CreatePurchaseOrder 创建采购单
// body: supplier_id, items[{material_id, quantity, price}], note
func CreatePurchaseOrder(c *gin.Context) {
	var req struct {
		SupplierID uint   `json:"supplier_id" binding:"required"`
		Note       string `json:"note"`
		Items      []struct {
			MaterialID uint    `json:"material_id" binding:"required"`
			Quantity   int     `json:"quantity" binding:"required"`
			Price      float64 `json:"price"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（supplier_id、items 必填）")
		return
	}
	if len(req.Items) == 0 {
		badRequest(c, "请至少添加一条采购明细")
		return
	}
	var s model.Supplier
	if err := model.DB.First(&s, req.SupplierID).Error; err != nil {
		badRequest(c, "供应商不存在")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	var totalAmount float64
	items := make([]model.PurchaseOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			badRequest(c, "采购数量必须大于0")
			return
		}
		var m model.Material
		if err := model.DB.First(&m, it.MaterialID).Error; err != nil {
			badRequest(c, fmt.Sprintf("物料 #%d 不存在", it.MaterialID))
			return
		}
		price := it.Price
		if price <= 0 {
			price = m.UnitPrice
		}
		amt := float64(it.Quantity) * price
		totalAmount += amt
		items = append(items, model.PurchaseOrderItem{
			MaterialID: m.ID, MaterialName: m.Name, Quantity: it.Quantity,
			Price: price, Amount: amt,
		})
	}

	var po model.PurchaseOrder
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		orderNo := model.NextBizNo(tx, "purchase_orders", "PO")
		po = model.PurchaseOrder{
			OrderNo: orderNo, SupplierID: req.SupplierID, Status: model.PurchaseStatusDraft,
			TotalAmount: totalAmount, Operator: operator, Note: req.Note,
		}
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = po.ID
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("purchase/%d", po.ID), "创建采购单 "+po.OrderNo)
	ok(c, gin.H{"purchase": po, "message": "采购单已创建"})
}

// ReceivePurchase 采购入库（支持部分入库）
// body: items[{item_id, quantity}] —— quantity 为本批本次入库数量
func ReceivePurchase(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "采购单ID无效")
		return
	}
	var req struct {
		Items []struct {
			ItemID   uint `json:"item_id" binding:"required"`
			Quantity int  `json:"quantity" binding:"required"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var po model.PurchaseOrder
	if err := model.DB.First(&po, id).Error; err != nil {
		notFound(c, "采购单不存在")
		return
	}
	if po.Status == model.PurchaseStatusCompleted || po.Status == model.PurchaseStatusCancelled {
		badRequest(c, "采购单已完成或已取消，无法入库")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	// 校验明细属于该采购单
	var items []model.PurchaseOrderItem
	model.DB.Where("order_id = ?", po.ID).Find(&items)
	itemMap := map[uint]*model.PurchaseOrderItem{}
	for i := range items {
		itemMap[items[i].ID] = &items[i]
	}
	for _, r := range req.Items {
		it, ok := itemMap[r.ItemID]
		if !ok {
			badRequest(c, fmt.Sprintf("明细 #%d 不属于该采购单", r.ItemID))
			return
		}
		if r.Quantity <= 0 {
			badRequest(c, "入库数量必须大于0")
			return
		}
		if it.ReceivedQty+r.Quantity > it.Quantity {
			badRequest(c, fmt.Sprintf("物料「%s」入库数量超过采购数量", it.MaterialName))
			return
		}
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		allComplete := true
		for _, r := range req.Items {
			it := itemMap[r.ItemID]
			newRecv := it.ReceivedQty + r.Quantity
			if err := tx.Model(&model.PurchaseOrderItem{}).Where("id = ?", it.ID).Update("received_qty", newRecv).Error; err != nil {
				return err
			}
			// 增加物料库存 + 写入库流水
			var m model.Material
			if err := tx.First(&m, it.MaterialID).Error; err != nil {
				return err
			}
			if err := tx.Model(&m).Update("stock", m.Stock+r.Quantity).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.MaterialStock{
				MaterialID: it.MaterialID, MaterialName: it.MaterialName, Type: model.StockTypeIn,
				Quantity: r.Quantity, Price: it.Price, Amount: float64(r.Quantity) * it.Price,
				RefType: "purchase", RefID: po.ID, Operator: operator, Note: "采购入库 " + po.OrderNo,
			}).Error; err != nil {
				return err
			}
			if newRecv < it.Quantity {
				allComplete = false
			}
		}

		// 重新检查是否全部明细已入库完毕
		var all []model.PurchaseOrderItem
		if err := tx.Where("order_id = ?", po.ID).Find(&all).Error; err != nil {
			return err
		}
		for _, a := range all {
			if a.ReceivedQty < a.Quantity {
				allComplete = false
				break
			}
		}
		status := model.PurchaseStatusPartial
		if allComplete {
			status = model.PurchaseStatusCompleted
		}
		updates := map[string]interface{}{"status": status}
		if status == model.PurchaseStatusCompleted {
			now := time.Now()
			updates["received_at"] = &now
		}
		return tx.Model(&po).Updates(updates).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	opText := "采购入库"
	if po.Status == model.PurchaseStatusDraft {
		opText = "采购单部分入库"
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("purchase/%d", po.ID), opText+" "+po.OrderNo)
	ok(c, gin.H{"message": "入库成功"})
}

// CancelPurchase 取消采购单（仅草稿/部分入库可取消）
func CancelPurchase(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "采购单ID无效")
		return
	}
	var po model.PurchaseOrder
	if err := model.DB.First(&po, id).Error; err != nil {
		notFound(c, "采购单不存在")
		return
	}
	if po.Status == model.PurchaseStatusCompleted || po.Status == model.PurchaseStatusCancelled {
		badRequest(c, "采购单已完成或已取消")
		return
	}
	if err := model.DB.Model(&po).Update("status", model.PurchaseStatusCancelled).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("purchase/%d", po.ID), "取消采购单 "+po.OrderNo)
	ok(c, gin.H{"message": "采购单已取消"})
}

// DeletePurchase 删除采购单（仅草稿可删除）
func DeletePurchase(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "采购单ID无效")
		return
	}
	var po model.PurchaseOrder
	if err := model.DB.First(&po, id).Error; err != nil {
		notFound(c, "采购单不存在")
		return
	}
	if po.Status != model.PurchaseStatusDraft {
		badRequest(c, "仅草稿状态的采购单可删除")
		return
	}
	if err := model.DB.Delete(&po).Error; err != nil {
		serverError(c, err)
		return
	}
	model.DB.Where("order_id = ?", po.ID).Delete(&model.PurchaseOrderItem{})
	recordOperation(c, model.OpDelete, fmt.Sprintf("purchase/%d", po.ID), "删除采购单 "+po.OrderNo)
	ok(c, gin.H{"message": "删除成功"})
}

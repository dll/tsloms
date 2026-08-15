package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"gorm.io/gorm"
)

// ===== 物料档案 =====

// ListMaterialsV2 物料列表（分页，可按编码/名称/分类/状态/低库存筛选）
func ListMaterialsV2(c *gin.Context) {
	page, pageSize := paginate(c)
	query := model.DB.Model(&model.Material{})

	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	if cat := c.Query("category"); cat != "" {
		query = query.Where("category = ?", cat)
	}
	if st := c.Query("status"); st != "" {
		query = query.Where("status = ?", st)
	}
	if low := c.Query("low_stock"); low == "1" || low == "true" {
		query = query.Where("stock <= threshold AND threshold > 0")
	}
	if dev := c.Query("device_hw_id"); dev != "" {
		query = query.Where("device_hw_id = ?", dev)
	}

	var total int64
	query.Count(&total)

	var list []model.Material
	query.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	items := make([]gin.H, 0, len(list))
	for _, m := range list {
		items = append(items, materialView(m))
	}
	ok(c, gin.H{"list": items, "total": total, "page": page, "page_size": pageSize})
}

func materialView(m model.Material) gin.H {
	return gin.H{
		"id": m.ID, "code": m.Code, "name": m.Name, "category": m.Category,
		"spec": m.Spec, "unit": m.Unit, "unit_price": m.UnitPrice, "stock": m.Stock,
		"threshold": m.Threshold, "supplier_id": m.SupplierID, "note": m.Note,
		"device_hw_id": m.DeviceHwID,
		"status":       m.Status, "low_stock": m.Threshold > 0 && m.Stock <= m.Threshold,
		"created_at": m.CreatedAt, "updated_at": m.UpdatedAt,
	}
}

// MaterialStats 库存概览统计
func MaterialStats(c *gin.Context) {
	var mCount, lowCount, stockClass int64
	model.DB.Model(&model.Material{}).Where("status = ?", "active").Count(&mCount)
	model.DB.Model(&model.Material{}).Where("status = ? AND threshold > 0 AND stock <= threshold", "active").Count(&lowCount)
	model.DB.Model(&model.MaterialStock{}).Where("type IN ?", []string{model.StockTypeIn, model.StockTypeUse}).Count(&stockClass)

	// 库存总金额（按当前库存 * 单价）
	var totalValue struct{ Sum float64 }
	model.DB.Model(&model.Material{}).Select("COALESCE(SUM(stock * unit_price),0) AS sum").Scan(&totalValue)

	ok(c, gin.H{
		"material_count": mCount, "low_stock_count": lowCount,
		"stock_record_count": stockClass, "total_value": totalValue.Sum,
	})
}

// SaveMaterial 新增/更新物料
func SaveMaterial(c *gin.Context) {
	var req struct {
		ID         *uint   `json:"id"`
		Code       string  `json:"code"`
		Name       string  `json:"name" binding:"required"`
		Category   string  `json:"category"`
		Spec       string  `json:"spec"`
		Unit       string  `json:"unit"`
		UnitPrice  float64 `json:"unit_price"`
		Stock      int     `json:"stock"`
		Threshold  int     `json:"threshold"`
		DeviceHwID *uint32 `json:"device_hw_id"`
		SupplierID *uint   `json:"supplier_id"`
		Note       string  `json:"note"`
		Status     string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（name 必填）")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	// 编码唯一（新增时才校验）
	if req.ID == nil || *req.ID == 0 {
		if req.Code == "" {
			badRequest(c, "请填写物料编码")
			return
		}
		var dup model.Material
		if err := model.DB.Where("code = ?", req.Code).First(&dup).Error; err == nil {
			badRequest(c, "物料编码已存在")
			return
		}
	}

	status := req.Status
	if status == "" {
		status = "active"
	}

	if req.ID != nil && *req.ID > 0 {
		var m model.Material
		if err := model.DB.First(&m, *req.ID).Error; err != nil {
			notFound(c, "物料不存在")
			return
		}
		updates := map[string]interface{}{
			"code": req.Code, "name": req.Name, "category": req.Category, "spec": req.Spec,
			"unit": req.Unit, "unit_price": req.UnitPrice, "threshold": req.Threshold,
			"device_hw_id": req.DeviceHwID, "supplier_id": req.SupplierID, "note": req.Note, "status": status,
		}
		if err := model.DB.Model(&m).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, fmt.Sprintf("material/%d", m.ID), "更新物料 "+m.Name)
		ok(c, gin.H{"message": "物料已更新"})
		return
	}

	m := model.Material{
		Code: req.Code, Name: req.Name, Category: req.Category, Spec: req.Spec,
		Unit: req.Unit, UnitPrice: req.UnitPrice, Stock: req.Stock, Threshold: req.Threshold,
		DeviceHwID: req.DeviceHwID, SupplierID: req.SupplierID, Note: req.Note, Status: status,
	}
	if err := model.DB.Create(&m).Error; err != nil {
		serverError(c, err)
		return
	}
	// 初始库存>0 时写入一条入库流水
	if m.Stock > 0 {
		model.DB.Create(&model.MaterialStock{
			MaterialID: m.ID, MaterialName: m.Name, Type: model.StockTypeIn,
			Quantity: m.Stock, Price: m.UnitPrice, Amount: float64(m.Stock) * m.UnitPrice,
			RefType: "adjust", Operator: operator, Note: "初始库存",
		})
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("material/%d", m.ID), "新增物料 "+m.Name)
	ok(c, gin.H{"material": materialView(m), "message": "物料已新增"})
}

// DeleteMaterialV2 删除物料
func DeleteMaterialV2(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "物料ID无效")
		return
	}
	var m model.Material
	if err := model.DB.First(&m, id).Error; err != nil {
		notFound(c, "物料不存在")
		return
	}
	if err := model.DB.Delete(&m).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("material/%d", id), "删除物料 "+m.Name)
	ok(c, gin.H{"message": "删除成功"})
}

// ===== 出入库流水 =====

// ListMaterialStocks 出入库流水（分页，可按物料/类型/日期筛选）
func ListMaterialStocks(c *gin.Context) {
	page, pageSize := paginate(c)
	query := model.DB.Model(&model.MaterialStock{})

	if mid := c.Query("material_id"); mid != "" {
		query = query.Where("material_id = ?", mid)
	}
	if typ := c.Query("type"); typ != "" {
		query = query.Where("type = ?", typ)
	}
	if from := c.Query("from"); from != "" {
		query = query.Where("created_at >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		query = query.Where("created_at <= ?", to+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var list []model.MaterialStock
	query.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// AdjustMaterialStock 手动调整库存（入库/盘盈/盘亏/报废）
// body: material_id, type(in/return/gain/loss/adjust), quantity(正数), note
func AdjustMaterialStock(c *gin.Context) {
	var req struct {
		MaterialID uint   `json:"material_id" binding:"required"`
		Type       string `json:"type" binding:"required"`
		Quantity   int    `json:"quantity" binding:"required"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（material_id、type、quantity 必填）")
		return
	}
	switch req.Type {
	case model.StockTypeIn, model.StockTypeReturn, model.StockTypeGain, model.StockTypeLoss, model.StockTypeAdjust:
	default:
		badRequest(c, "库存变动类型不合法")
		return
	}
	if req.Quantity == 0 {
		badRequest(c, "变动数量不能为0")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	var m model.Material
	if err := model.DB.First(&m, req.MaterialID).Error; err != nil {
		notFound(c, "物料不存在")
		return
	}

	// 出库类(盘亏/报废)用负数
	sign := 1
	if req.Type == model.StockTypeLoss {
		sign = -1
	}
	delta := req.Quantity * sign
	if req.Type == model.StockTypeAdjust {
		delta = req.Quantity
	}
	newStock := m.Stock + delta
	if newStock < 0 {
		badRequest(c, "库存不足，无法执行该变动")
		return
	}

	price := m.UnitPrice
	amount := float64(delta) * price

	// 更新库存 + 写流水（同一事务保证一致）
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&m).Update("stock", newStock).Error; err != nil {
			return err
		}
		return tx.Create(&model.MaterialStock{
			MaterialID: m.ID, MaterialName: m.Name, Type: req.Type,
			Quantity: delta, Price: price, Amount: amount,
			RefType: "adjust", Operator: operator, Note: req.Note,
		}).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("material-stock/%d", m.ID), fmt.Sprintf("调整库存 %s %d", m.Name, delta))
	ok(c, gin.H{"message": "库存已调整", "stock": newStock})
}

// UseMaterialStock 工单领料出库
// 维修/工单处理时从库存领用物料：扣减库存并写入 type=use 出库流水，关联工单与设备
// body: material_id(必填), quantity(必填,正数=领用数量), work_order_id(必填), note
func UseMaterialStock(c *gin.Context) {
	var req struct {
		MaterialID  uint   `json:"material_id" binding:"required"`
		Quantity    int    `json:"quantity" binding:"required"`
		WorkOrderID uint   `json:"work_order_id" binding:"required"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（material_id、quantity、work_order_id 必填）")
		return
	}
	if req.Quantity <= 0 {
		badRequest(c, "领用数量必须大于0")
		return
	}
	operator := c.GetString("op_username")
	if operator == "" {
		operator = "system"
	}

	// 校验工单存在（并带出设备ID用于出库流水）
	var wo model.WorkOrder
	if err := model.DB.First(&wo, req.WorkOrderID).Error; err != nil {
		notFound(c, "工单不存在")
		return
	}

	// 校验物料存在
	var m model.Material
	if err := model.DB.First(&m, req.MaterialID).Error; err != nil {
		notFound(c, "物料不存在")
		return
	}

	// 领用出库：库存扣减 quantity，流水 quantity 为负数
	if m.Stock < req.Quantity {
		badRequest(c, "库存不足，无法领用")
		return
	}
	newStock := m.Stock - req.Quantity
	price := m.UnitPrice
	amount := float64(-req.Quantity) * price

	// 更新库存 + 写 type=use 出库流水 + 自动生成耗材费用（同一事务保证一致）
	// 领料成本自动归集到关联工单/设备的维修成本，费用统计(material)包含领料支出
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&m).Update("stock", newStock).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.MaterialStock{
			MaterialID:   m.ID,
			MaterialName: m.Name,
			Type:         model.StockTypeUse,
			Quantity:     -req.Quantity,
			Price:        price,
			Amount:       amount,
			RefType:      "repair",
			RefID:        req.WorkOrderID,
			WorkOrderID:  &req.WorkOrderID,
			Operator:     operator,
			Note:         req.Note,
		}).Error; err != nil {
			return err
		}
		// 自动生成耗材费用单(关联工单+设备)，供维修成本归集
		expenseNo := model.NextBizNoCol(tx, "repair_expenses", "expense_no", "FE")
		return tx.Create(&model.RepairExpense{
			ExpenseNo:   expenseNo,
			WorkOrderID: &req.WorkOrderID,
			DeviceHwID:  wo.DeviceHwID,
			Type:        model.ExpenseTypeMaterial,
			Amount:      float64(req.Quantity) * price,
			Description: "工单领料: " + m.Name + " x" + fmt.Sprint(req.Quantity),
			Operator:    operator,
			Confirmed:   false,
			Note:        req.Note,
		}).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("material-stock/use/%d", m.ID),
		fmt.Sprintf("工单#%s 领用 %s x%d", wo.OrderNo, m.Name, req.Quantity))
	ok(c, gin.H{"message": "领料出库成功", "stock": newStock, "material_id": m.ID})
}

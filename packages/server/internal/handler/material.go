package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListMaterials 耗材列表查询（可按设备）
func ListMaterials(c *gin.Context) {
	query := model.DB.Model(&model.DeviceMaterial{})
	if hwID := c.Query("device_hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}
	var list []model.DeviceMaterial
	query.Order("device_hw_id ASC, id ASC").Find(&list)
	ok(c, gin.H{"list": list, "total": len(list)})
}

// UpsertMaterial 新增或更新耗材
func UpsertMaterial(c *gin.Context) {
	var req struct {
		ID         *uint    `json:"id"`
		DeviceHwID uint32   `json:"device_hw_id" binding:"required"`
		Name       string   `json:"name" binding:"required"`
		PartNo     string   `json:"part_no"`
		Spec       string   `json:"spec"`
		Quantity   int      `json:"quantity"`
		Unit       string   `json:"unit"`
		Threshold  int      `json:"threshold"`
		Note       string   `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id、name 必填）")
		return
	}
	now := time.Now()

	if req.ID != nil && *req.ID > 0 {
		// 更新
		var m model.DeviceMaterial
		if err := model.DB.First(&m, *req.ID).Error; err != nil {
			notFound(c, "耗材不存在")
			return
		}
		updates := map[string]interface{}{
			"name": req.Name, "part_no": req.PartNo, "spec": req.Spec,
			"quantity": req.Quantity, "unit": req.Unit, "threshold": req.Threshold,
			"note": req.Note, "last_changed_at": now,
		}
		if err := model.DB.Model(&m).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, fmt.Sprintf("material/%d", m.ID), "更新耗材")
		ok(c, gin.H{"material": m, "message": "耗材已更新"})
		return
	}

	// 新增
	m := model.DeviceMaterial{
		DeviceHwID: req.DeviceHwID, Name: req.Name, PartNo: req.PartNo, Spec: req.Spec,
		Quantity: req.Quantity, Unit: req.Unit, Threshold: req.Threshold, Note: req.Note,
		LastChangedAt: &now,
	}
	if err := model.DB.Create(&m).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("material/%d", m.ID), "新增耗材")
	ok(c, gin.H{"material": m, "message": "耗材已新增"})
}

// DeleteMaterial 删除耗材
func DeleteMaterial(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "耗材ID无效")
		return
	}
	if err := model.DB.Delete(&model.DeviceMaterial{}, id).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("material/%d", id), "删除耗材")
	ok(c, gin.H{"message": "删除成功"})
}

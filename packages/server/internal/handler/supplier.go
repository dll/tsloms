package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListSuppliers 供应商列表（分页或全量）
func ListSuppliers(c *gin.Context) {
	// 支持 all=1 全量（下拉用）
	if c.Query("all") == "1" || c.Query("all") == "true" {
		var list []model.Supplier
		if err := model.DB.Order("created_at DESC").Find(&list).Error; err != nil {
			serverError(c, err)
			return
		}
		ok(c, gin.H{"list": list, "total": len(list)})
		return
	}

	page, pageSize := paginate(c)
	query := model.DB.Model(&model.Supplier{})
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("name LIKE ? OR contact LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	query.Count(&total)
	var list []model.Supplier
	query.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// SaveSupplier 新增/更新供应商
func SaveSupplier(c *gin.Context) {
	var req struct {
		ID      *uint  `json:"id"`
		Name    string `json:"name" binding:"required"`
		Contact string `json:"contact"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
		Email   string `json:"email"`
		Status  string `json:"status"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（name 必填）")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}

	if req.ID != nil && *req.ID > 0 {
		var s model.Supplier
		if err := model.DB.First(&s, *req.ID).Error; err != nil {
			notFound(c, "供应商不存在")
			return
		}
		updates := map[string]interface{}{
			"name": req.Name, "contact": req.Contact, "phone": req.Phone,
			"address": req.Address, "email": req.Email, "status": status, "note": req.Note,
		}
		if err := model.DB.Model(&s).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
		recordOperation(c, model.OpUpdate, fmt.Sprintf("supplier/%d", s.ID), "更新供应商 "+s.Name)
		ok(c, gin.H{"message": "供应商已更新"})
		return
	}

	s := model.Supplier{
		Name: req.Name, Contact: req.Contact, Phone: req.Phone,
		Address: req.Address, Email: req.Email, Status: status, Note: req.Note,
	}
	if err := model.DB.Create(&s).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("supplier/%d", s.ID), "新增供应商 "+s.Name)
	ok(c, gin.H{"supplier": s, "message": "供应商已新增"})
}

// DeleteSupplier 删除供应商
func DeleteSupplier(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "供应商ID无效")
		return
	}
	var s model.Supplier
	if err := model.DB.First(&s, id).Error; err != nil {
		notFound(c, "供应商不存在")
		return
	}
	if err := model.DB.Delete(&s).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("supplier/%d", id), "删除供应商 "+s.Name)
	ok(c, gin.H{"message": "删除成功"})
}

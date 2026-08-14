package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListDepartments 部门列表（含各部门人数、子部门层级展开）。管理员可增删改，其余角色可读。
func ListDepartments(c *gin.Context) {
	var depts []model.Department
	model.DB.Order("id ASC").Find(&depts)

	// 各部门人数
	type cnt struct{ Count int64 }
	memberCount := map[uint]int64{}
	var rows []struct {
		DepartmentID *uint
		Count        int64
	}
	model.DB.Model(&model.User{}).
		Select("department_id, COUNT(*) AS count").
		Where("department_id IS NOT NULL").
		Group("department_id").Scan(&rows)
	for _, r := range rows {
		if r.DepartmentID != nil {
			memberCount[*r.DepartmentID] = r.Count
		}
	}

	out := make([]gin.H, 0, len(depts))
	for _, d := range depts {
		out = append(out, gin.H{
			"id": d.ID, "name": d.Name, "parent_id": d.ParentID,
			"leader": d.Leader, "description": d.Description,
			"member_count": memberCount[d.ID],
			"created_at":   d.CreatedAt,
		})
	}
	ok(c, gin.H{"list": out, "total": len(out)})
}

// CreateDepartment 新增部门（管理员）
func CreateDepartment(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		ParentID    *uint  `json:"parent_id"`
		Leader      string `json:"leader"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		badRequest(c, "部门名称必填")
		return
	}
	var count int64
	model.DB.Model(&model.Department{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		badRequest(c, "部门名称已存在")
		return
	}
	dept := model.Department{Name: req.Name, ParentID: req.ParentID, Leader: req.Leader, Description: req.Description}
	// 上级部门必须存在
	if req.ParentID != nil {
		var p model.Department
		if err := model.DB.First(&p, *req.ParentID).Error; err != nil {
			badRequest(c, "上级部门不存在")
			return
		}
	}
	if err := model.DB.Create(&dept).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, "department/"+req.Name, "新增部门")
	ok(c, gin.H{"id": dept.ID, "name": dept.Name, "message": "部门创建成功"})
}

// UpdateDepartment 更新部门（管理员）
func UpdateDepartment(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "部门ID无效")
		return
	}
	var dept model.Department
	if err := model.DB.First(&dept, id).Error; err != nil {
		notFound(c, "部门不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		ParentID    *uint   `json:"parent_id"`
		Leader      *string `json:"leader"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil && *req.Name != "" {
		var cnt int64
		model.DB.Model(&model.Department{}).Where("name = ? AND id <> ?", *req.Name, id).Count(&cnt)
		if cnt > 0 {
			badRequest(c, "部门名称已存在")
			return
		}
		updates["name"] = *req.Name
	}
	if req.Leader != nil {
		updates["leader"] = *req.Leader
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ParentID != nil {
		if *req.ParentID == id {
			badRequest(c, "上级部门不能是自身")
			return
		}
		var p model.Department
		if err := model.DB.First(&p, *req.ParentID).Error; err != nil {
			badRequest(c, "上级部门不存在")
			return
		}
		updates["parent_id"] = *req.ParentID
	}
	if len(updates) > 0 {
		if err := model.DB.Model(&dept).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
	}
	recordOperation(c, model.OpUpdate, "department/"+dept.Name, "更新部门")
	ok(c, gin.H{"message": "部门更新成功"})
}

// DeleteDepartment 删除部门（管理员）。若仍有成员则拒绝删除。
func DeleteDepartment(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "部门ID无效")
		return
	}
	var dept model.Department
	if err := model.DB.First(&dept, id).Error; err != nil {
		notFound(c, "部门不存在")
		return
	}
	// 存在成员则拒绝
	var memberCount int64
	model.DB.Model(&model.User{}).Where("department_id = ?", id).Count(&memberCount)
	if memberCount > 0 {
		badRequest(c, "该部门下仍有用户，无法删除")
		return
	}
	if err := model.DB.Delete(&dept).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, "department/"+dept.Name, "删除部门")
	ok(c, gin.H{"message": "部门删除成功"})
}

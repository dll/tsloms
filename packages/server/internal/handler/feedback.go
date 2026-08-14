package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListFeedbacks 反馈列表查询
func ListFeedbacks(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := model.DB.Model(&model.Feedback{})
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if hwID := c.Query("device_hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)
	var list []model.Feedback
	query.Order("created_at DESC").Offset(int((page-1)*pageSize)).Limit(int(pageSize)).Find(&list)
	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// CreateFeedback 提交问题反馈（地图/移动/后台）
func CreateFeedback(c *gin.Context) {
	var req struct {
		DeviceHwID  *uint32 `json:"device_hw_id"`
		Intersection string `json:"intersection"`
		Title       string  `json:"title" binding:"required"`
		Content     string  `json:"content"`
		Reporter    string  `json:"reporter"`
		Contact     string  `json:"contact"`
		ImageURL    string  `json:"image_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "反馈标题必填")
		return
	}
	fb := model.Feedback{
		DeviceHwID:   req.DeviceHwID,
		Intersection: req.Intersection,
		Title:        req.Title,
		Content:      req.Content,
		Reporter:     req.Reporter,
		Contact:      req.Contact,
		ImageURL:     req.ImageURL,
		Status:       model.FeedbackOpen,
	}
	if err := model.DB.Create(&fb).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("feedback/%d", fb.ID), "提交问题反馈")
	ok(c, gin.H{"feedback": fb, "message": "反馈提交成功"})
}

// UpdateFeedbackStatus 更新反馈状态 / 关联工单
func UpdateFeedbackStatus(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "反馈ID无效")
		return
	}
	var req struct {
		Status       string  `json:"status"`
		WorkOrderID  *uint   `json:"work_order_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var fb model.Feedback
	if err := model.DB.First(&fb, id).Error; err != nil {
		notFound(c, "反馈不存在")
		return
	}
	updates := map[string]interface{}{}
	if req.Status != "" {
		valid := map[string]bool{model.FeedbackOpen: true, model.FeedbackProcessing: true, model.FeedbackResolved: true, model.FeedbackClosed: true}
		if !valid[req.Status] {
			badRequest(c, "无效状态")
			return
		}
		updates["status"] = req.Status
	}
	if req.WorkOrderID != nil {
		updates["work_order_id"] = *req.WorkOrderID
	}
	if len(updates) > 0 {
		if err := model.DB.Model(&fb).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("feedback/%d", id), "更新反馈")
	ok(c, gin.H{"feedback": fb, "message": "更新成功"})
}

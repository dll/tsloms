package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListPacketLogs 报文日志查询
// 支持按设备、命令类型、有效性筛选，分页查询
func ListPacketLogs(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := model.DB.Model(&model.PacketLog{})

	// 按设备硬件 ID 筛选
	if hwID := c.Query("device_hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}
	// 按命令类型筛选
	if cmdType := c.Query("cmd_type"); cmdType != "" {
		query = query.Where("cmd_type = ?", cmdType)
	}
	// 按有效性筛选
	if valid := c.Query("valid"); valid != "" {
		query = query.Where("valid = ?", valid == "true")
	}
	// 按时间范围筛选
	if start := c.Query("start_time"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			query = query.Where("received_at >= ?", t)
		}
	}
	if end := c.Query("end_time"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			query = query.Where("received_at <= ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	query.Count(&total)

	var logs []model.PacketLog
	query.Order("received_at DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&logs)

	ok(c, gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListOperationLogs 系统操作日志查询
// 支持按操作人、操作类型、时间范围筛选，分页查询
func ListOperationLogs(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := model.DB.Model(&model.OperationLog{})

	// 按操作人筛选
	if username := c.Query("username"); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	// 按操作类型筛选
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	// 按时间范围筛选
	if start := c.Query("start_time"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if end := c.Query("end_time"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	var total int64
	query.Count(&total)

	var logs []model.OperationLog
	query.Order("created_at DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&logs)

	ok(c, gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// recordOperation 记录一条操作日志（内部使用，写入上下文标记，由中间件统一落库）
func recordOperation(c *gin.Context, action, target, detail string) {
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	// 未登录用户（如登录动作本身）使用 c 中的用户名占位
	if username == "" {
		username = c.GetString("op_username")
	}
	log := model.OperationLog{
		UserID:   userID,
		Username: username,
		Action:   action,
		Target:   target,
		Detail:   detail,
		IP:       c.ClientIP(),
	}
	if err := model.DB.Create(&log).Error; err != nil {
		c.Error(fmt.Errorf("记录操作日志失败: %w", err))
	}
}

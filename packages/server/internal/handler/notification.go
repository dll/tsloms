package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================
// 站内通知：AI 主动巡检推送（报告提醒 / 异常预警）等
// ============================================================

// ListNotificationsAPI 当前用户通知列表
func ListNotificationsAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	limit := 30
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	list := model.ListUserNotifications(uid, limit)
	count := model.UnreadNotificationCount(uid)
	ok(c, gin.H{"list": list, "unread": count, "total": len(list)})
}

// UnreadCountAPI 当前用户未读通知数（顶部铃铛角标）
func UnreadCountAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	ok(c, gin.H{"unread": model.UnreadNotificationCount(uid)})
}

// ReadNotificationAPI 标记单条已读（用户级隔离：广播通知写入 notification_reads）
func ReadNotificationAPI(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "通知ID无效")
		return
	}
	uid := userIDFromCtx(c)
	if _, err := model.MarkNotificationRead(uid, id); err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}

// ReadAllNotificationsAPI 全部标记已读（用户级隔离）
func ReadAllNotificationsAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	if err := model.MarkAllNotificationsRead(uid); err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

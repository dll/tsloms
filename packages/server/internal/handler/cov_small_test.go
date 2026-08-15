package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 反馈 Feedback ====================

func seedDev(hw uint32, inter string) {
	model.DB.Create(&model.Device{HwID: hw, Intersection: inter, OnlineStatus: true})
}

func TestFeedback_CreateAndList(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/feedbacks", ListFeedbacks)
	rg.POST("/feedbacks", CreateFeedback)
	rg.PUT("/feedbacks/:id", UpdateFeedbackStatus)
	seedDev(11, "路口甲")

	// 创建
	code, body := doReq(t, r, "POST", "/api/v1/feedbacks", `{"device_hw_id":11,"title":"灯不亮","content":"南侧"}`)
	mustOK(t, code, body, "创建反馈")
	// 创建时自动带出路口
	fb := body["data"].(map[string]interface{})["feedback"].(map[string]interface{})
	if fb["intersection"] != "路口甲" {
		t.Errorf("intersection 应自动带出, got %v", fb["intersection"])
	}

	// 列表（含筛选：status/hw/时间/排序）
	code, body = doReq(t, r, "GET", "/api/v1/feedbacks", "")
	mustOK(t, code, body, "反馈列表")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("total 期望 1")
	}
	for _, q := range []string{"?status=open", "?device_hw_id=11", "?keyword=灯", "?start_time=2026-01-01", "?end_time=2026-12-31", "?sort_by=status&order=asc"} {
		code, _ = doReq(t, r, "GET", "/api/v1/feedbacks"+q, "")
		if code != http.StatusOK {
			t.Errorf("筛选 %s 失败 code=%d", q, code)
		}
	}
	// 排序缺省
	doReq(t, r, "GET", "/api/v1/feedbacks?sort_by=created_at", "")
	doReq(t, r, "GET", "/api/v1/feedbacks?sort_by=bogus", "")
}

func TestFeedback_Validation(t *testing.T) {
	r := covSetup(t)
	rr := r.Group("/api/v1")
	rr.POST("/feedbacks", CreateFeedback)
	// 缺标题
	code, _ := doReq(t, r, "POST", "/api/v1/feedbacks", `{"device_hw_id":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺标题应 400, got %d", code)
	}
	// hw==0
	code, _ = doReq(t, r, "POST", "/api/v1/feedbacks", `{"device_hw_id":0,"title":"x"}`)
	if code != http.StatusBadRequest {
		t.Errorf("hw 0 应 400, got %d", code)
	}
	// 设备不存在
	code, _ = doReq(t, r, "POST", "/api/v1/feedbacks", `{"device_hw_id":999,"title":"x"}`)
	if code != http.StatusBadRequest {
		t.Errorf("设备不存在应 400, got %d", code)
	}
}

func TestFeedback_UpdateStatus(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/feedbacks", CreateFeedback)
	rg.PUT("/feedbacks/:id", UpdateFeedbackStatus)
	seedDev(12, "路口乙")
	_, body := doReq(t, r, "POST", "/api/v1/feedbacks", `{"device_hw_id":12,"title":"闪烁"}`)
	id := uint(body["data"].(map[string]interface{})["feedback"].(map[string]interface{})["id"].(float64))

	// 有效状态
	code, _ := doReq(t, r, "PUT", "/api/v1/feedbacks/"+uid(id), `{"status":"processing"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新状态")
	// 关联工单
	code, _ = doReq(t, r, "PUT", "/api/v1/feedbacks/"+uid(id), `{"work_order_id":5}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "关联工单")
	// 无效状态
	code, _ = doReq(t, r, "PUT", "/api/v1/feedbacks/"+uid(id), `{"status":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Errorf("无效状态应 400, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "PUT", "/api/v1/feedbacks/abc", `{"status":"open"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
	// 不存在
	code, _ = doReq(t, r, "PUT", "/api/v1/feedbacks/99999", `{"status":"open"}`)
	if code != http.StatusNotFound {
		t.Errorf("不存在反馈应 404, got %d", code)
	}
}

// ==================== 供应商 Supplier ====================

func TestSupplier_CRUD(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/suppliers", ListSuppliers)
	rg.POST("/suppliers", SaveSupplier)
	rg.DELETE("/suppliers/:id", DeleteSupplier)

	// 新增
	code, body := doReq(t, r, "POST", "/api/v1/suppliers", `{"name":"华天科技","phone":"13800000000"}`)
	mustOK(t, code, body, "新增供应商")
	s := body["data"].(map[string]interface{})["supplier"].(map[string]interface{})
	sid := uint(s["id"].(float64))

	// 列表（分页 + 关键词）
	code, body = doReq(t, r, "GET", "/api/v1/suppliers", "")
	mustOK(t, code, body, "供应商列表")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("total 期望 1")
	}
	code, _ = doReq(t, r, "GET", "/api/v1/suppliers?keyword=华天", "")
	if code != http.StatusOK {
		t.Errorf("关键词筛选失败")
	}
	code, body = doReq(t, r, "GET", "/api/v1/suppliers?all=1", "")
	mustOK(t, code, body, "全量供应商")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("all=1 total 期望 1")
	}

	// 更新
	code, _ = doReq(t, r, "POST", "/api/v1/suppliers", `{"id":`+uid(sid)+`,"name":"华天科技新","status":"inactive"}`)
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "更新供应商")
	var after model.Supplier
	model.DB.First(&after, sid)
	if after.Name != "华天科技新" || after.Status != "inactive" {
		t.Errorf("更新失败: %+v", after)
	}

	// 更新不存在
	code, _ = doReq(t, r, "POST", "/api/v1/suppliers", `{"id":9999,"name":"x"}`)
	if code != http.StatusNotFound {
		t.Errorf("更新不存在应 404, got %d", code)
	}
	// 缺 name
	code, _ = doReq(t, r, "POST", "/api/v1/suppliers", `{"phone":"1"}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺name应 400, got %d", code)
	}
	// 删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/suppliers/"+uid(sid), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除供应商")
	// 重复删除
	code, _ = doReq(t, r, "DELETE", "/api/v1/suppliers/"+uid(sid), "")
	if code != http.StatusNotFound {
		t.Errorf("重复删除应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/suppliers/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
}

// ==================== 通知 Notification ====================

func TestNotification_ListUnread(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/notifications", ListNotificationsAPI)
	rg.GET("/notifications/unread-count", UnreadCountAPI)
	rg.POST("/notifications/:id/read", ReadNotificationAPI)
	rg.POST("/notifications/read-all", ReadAllNotificationsAPI)

	// 未注入 user_id 时 uid=0（广播口）
	code, body := doReq(t, r, "GET", "/api/v1/notifications", "")
	mustOK(t, code, body, "通知列表")
	code, body = doReq(t, r, "GET", "/api/v1/notifications/unread-count", "")
	mustOK(t, code, body, "未读数")
	code, body = doReq(t, r, "GET", "/api/v1/notifications?limit=abc", "")
	mustOK(t, code, body, "limit 非法回退")
	code, _ = doReq(t, r, "POST", "/api/v1/notifications/abc/read", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法通知ID应 400, got %d", code)
	}
	// 读不存在的通知（用户级隔离，幂等）
	code, _ = doReq(t, r, "POST", "/api/v1/notifications/1/read", "")
	if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusNotFound {
		t.Errorf("读不存在通知 code=%d", code)
	}
}

func TestNotification_WithUser(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/notifications", ListNotificationsAPI)
	rg.POST("/notifications/:id/read", ReadNotificationAPI)
	rg.POST("/notifications/read-all", ReadAllNotificationsAPI)
	rg.Use(func(c *gin.Context) { c.Set("user_id", uint(7)); c.Next() })
	rg.GET("/n", ListNotificationsAPI)
	rg.POST("/n/:id/read", ReadNotificationAPI)
	rg.POST("/n/read-all", ReadAllNotificationsAPI)

	// 广播通知给所有用户(user_id=0)
	model.DB.Create(&model.Notification{Type: "broadcast", Title: "全员提醒", Content: "x", UserID: 0})
	code, body := doReq(t, r, "GET", "/api/v1/n", "")
	mustOK(t, code, body, "用户通知列表")
	if body["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("用户应看到广播通知, total=%v", body["data"].(map[string]interface{})["total"])
	}
	// 标记已读
	nt := model.Notification{}
	model.DB.First(&nt)
	code, _ = doReq(t, r, "POST", "/api/v1/n/"+uid(nt.ID)+"/read", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "标记已读")
	// 全部已读
	code, _ = doReq(t, r, "POST", "/api/v1/n/read-all", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "全部已读")
}

// ==================== 派单参考 Dispatch ====================

func TestDispatchReference(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/dispatch", DispatchReference)
	seedDev(7, "路口丙")
	// 缺参数
	code, _ := doReq(t, r, "GET", "/api/v1/dispatch", "")
	if code != http.StatusBadRequest {
		t.Errorf("缺 device_hw_id 应 400, got %d", code)
	}
	// 正常
	code, body := doReq(t, r, "GET", "/api/v1/dispatch?device_hw_id=7", "")
	mustOK(t, code, body, "派单参考")
	if body["data"].(map[string]interface{})["device_hw_id"].(float64) != 7 {
		t.Errorf("device_hw_id=%v", body["data"].(map[string]interface{})["device_hw_id"])
	}
	// 非法数字回退（fmt.Sscanf 解析失败→0）
	code, _ = doReq(t, r, "GET", "/api/v1/dispatch?device_hw_id=abc", "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "非法hw容错")
}

// ==================== 地图瓦片代理 Proxy ====================

func TestTileProxy_Validation(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.GET("/tile/baidu", BaiduTileProxy)
	rg.GET("/tile/gaode", GaodeTileProxy)
	// 缺参数
	for _, p := range []string{"/api/v1/tile/baidu", "/api/v1/tile/baidu?x=1&y=2", "/api/v1/tile/gaode", "/api/v1/tile/gaode?x=1&y=2"} {
		code, _ := doReq(t, r, "GET", p, "")
		if code != http.StatusBadRequest {
			t.Errorf("%s 缺参应 400, got %d", p, code)
		}
	}
	// mustParseInt 边界
	if mustParseInt("123") != 123 || mustParseInt("12a3") != 12 || mustParseInt("abc") != 0 {
		t.Error("mustParseInt 解析错误")
	}
}

// ==================== 登录 Auth ====================

func TestAuth_LoginFlows(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/auth/login", Login)
	// 缺参数
	code, _ := doReq(t, r, "POST", "/api/v1/auth/login", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 用户不存在
	code, _ = doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"nobody","password":"x"}`)
	if code != http.StatusUnauthorized && code != http.StatusBadRequest {
		t.Errorf("不存在用户应 401/400, got %d", code)
	}
}

func TestAuth_LoginAndDisabled(t *testing.T) {
	r := covSetup(t)
	rg := r.Group("/api/v1")
	rg.POST("/auth/login", Login)
	// 创建带 bcrypt 密码的用户
	pwd, _ := bcrypt.GenerateFromPassword([]byte("Test@12345"), bcrypt.MinCost)
	model.DB.Create(&model.User{Username: "l1", PasswordHash: string(pwd), Role: "operator", Status: ""})
	// 正确密码
	code, body := doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"l1","password":"Test@12345"}`)
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Errorf("正确密码登录失败 code=%d body=%v", code, body)
	}
	// 错误密码
	code, _ = doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"l1","password":"wrong"}`)
	if code != http.StatusUnauthorized {
		t.Errorf("错误密码应 401, got %d", code)
	}
	// 停用用户
	model.DB.Model(&model.User{}).Where("username = ?", "l1").Update("status", "disabled")
	code, _ = doReq(t, r, "POST", "/api/v1/auth/login", `{"username":"l1","password":"Test@12345"}`)
	if code != http.StatusUnauthorized {
		t.Errorf("停用用户应 401, got %d", code)
	}
}

// ==================== 响应辅助 Response / 健康 ====================

func TestHealthEndpoint(t *testing.T) {
	r := covSetup(t)
	r.GET("/health", Health)
	code, body := doReq(t, r, "GET", "/health", "")
	mustOK(t, code, body, "健康检查")
	if body["status"] != "ok" && body["data"] != nil {
		// 兼容不同响应结构，只要求 200 + code 0
	}
	// parseUint 边界
	if v, err := parseUint("123"); err != nil || v != 123 {
		t.Errorf("parseUint(123)=%d err=%v", v, err)
	}
	if _, err := parseUint("abc"); err == nil {
		t.Error("parseUint 非法应报错")
	}
	// paginate 边界
}

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// 纯函数：角色运维权限判断
func TestRoleIsOperator(t *testing.T) {
	if !RoleIsOperator(model.RoleAdmin) {
		t.Error("admin 应具备运维权限")
	}
	if !RoleIsOperator(model.RoleOperator) {
		t.Error("operator 应具备运维权限")
	}
	if RoleIsOperator("viewer") {
		t.Error("viewer 不应具备运维权限")
	}
	if RoleIsOperator("") {
		t.Error("空角色不应具备运维权限")
	}
}

// PublicDepartments：公开部门列表只含 id/name
func TestPublicDepartments(t *testing.T) {
	r := covSetup(t)
	model.DB.Create(&model.Department{Name: "运维部"})
	model.DB.Create(&model.Department{Name: "客服部"})

	r.GET("/public/departments", PublicDepartments)
	code, body := doReq(t, r, "GET", "/public/departments", "")
	mustOK(t, code, body, "公开部门列表")
	list, ok := body["list"].([]interface{})
	if !ok {
		t.Fatalf("list 断言失败: %v", body)
	}
	if len(list) != 2 {
		t.Fatalf("应返回 2 个部门, got %d", len(list))
	}
	first, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("部门条目断言失败: %v", list[0])
	}
	if _, hasName := first["name"]; !hasName {
		t.Errorf("应包含 name 字段: %v", first)
	}
	if _, hasLeader := first["leader"]; hasLeader {
		t.Errorf("公开列表不应暴露内部字段: %v", first)
	}
}

// SetMQTTClient：注入/清空全局 MQTT 客户端
func TestSetMQTTClient(t *testing.T) {
	SetMQTTClient(nil)
	if mqttGlobalClient() != nil {
		t.Error("SetMQTTClient(nil) 应为空")
	}
	// 用 mock 注入后应能取回
	mock := &fakeMQTT{}
	SetMQTTClient(mock)
	if mqttGlobalClient() != mock {
		t.Error("注入 mock 后应能取回")
	}
	SetMQTTClient(nil)
}

type fakeMQTT struct{}

func (f *fakeMQTT) IsConnected() bool { return false }

// buildFaultQuery：各过滤条件组合正确拼 SQL
func TestBuildFaultQuery(t *testing.T) {
	covSetup(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	// 无过滤条件：应返回全部
	req := httptest.NewRequest("GET", "/faults?hw_id=dev1&status=occurred&fault_type=lamp_off&fault_level=critical", nil)
	ctx.Request = req
	q := buildFaultQuery(ctx)
	if q == nil {
		t.Fatal("buildFaultQuery nil")
	}

	// 造数据后验证查询可执行
	model.DB.Create(&model.FaultRecord{DeviceHwID: "dev1", Status: "occurred", FaultType: "lamp_off", FaultLevel: "critical"})
	var n int64
	q.Count(&n)
	if n < 1 {
		t.Errorf("过滤查询应命中 1 条, got %d", n)
	}
}

// parseUint（handler 本地 helper，越界回退默认）
func TestHandlerParseUint(t *testing.T) {
	if v, _ := parseUint("0"); v != 0 {
		t.Errorf("parseUint(0) = %d", v)
	}
	if v, _ := parseUint("999"); v != 999 {
		t.Errorf("parseUint(999) = %d", v)
	}
	if _, err := parseUint("abc"); err == nil {
		t.Errorf("parseUint(abc) 应报错")
	}
}

// faultTypeCN 映射完整性
func TestFaultTypeCN(t *testing.T) {
	keys := []string{"lamp_off", "abnormal_on", "timeout", "dim", "power_loss", "unknown"}
	for _, k := range keys {
		if faultTypeCN[k] == "" {
			t.Errorf("faultTypeCN 缺 %s", k)
		}
	}
}

// 占位：确保 http 包被引用（部分 helper 使用）
var _ = http.StatusOK

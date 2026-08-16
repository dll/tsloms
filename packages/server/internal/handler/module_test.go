package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 辅助：按给定 ENABLED_MODULES 内容重建已启模块集合（测试环境也按真实生产语义判定）
func loadModulesForTest(t *testing.T, enabled string) {
	t.Helper()
	// 保存并临时关闭 runningUnderTest，使可选模块严格按配置启用（模拟生产判定）
	orig := runningUnderTest
	runningUnderTest = false
	parseEnabledModulesFrom(enabled)
	runningUnderTest = orig
}

// TestModule_CoreAlwaysEnabled 核心模块恒启，不受配置影响
func TestModule_CoreAlwaysEnabled(t *testing.T) {
	loadModulesForTest(t, "") // 空配置 → 仅核心
	for _, k := range coreModuleKeys {
		if !ModuleEnabled(k) {
			t.Errorf("核心模块 %s 应在空配置下恒启", k)
		}
	}
}

// TestModule_OptionalDefaultOff 可选模块默认不加载
func TestModule_OptionalDefaultOff(t *testing.T) {
	loadModulesForTest(t, "")
	for _, k := range optionalModuleKeys {
		if ModuleEnabled(k) {
			t.Errorf("可选模块 %s 默认应不加载（未配置）", k)
		}
	}
}

// TestModule_OptionalEnabledByConfig 配置 ENABLED_MODULES 后可选模块加载
func TestModule_OptionalEnabledByConfig(t *testing.T) {
	loadModulesForTest(t, "inventory,ai")
	for _, k := range []string{ModuleInventory, ModuleAI} {
		if !ModuleEnabled(k) {
			t.Errorf("配置后模块 %s 应启用", k)
		}
	}
	if ModuleEnabled(ModulePurchase) {
		t.Errorf("未配置模块 purchase 不应启用")
	}
}

// TestModule_RequireMiddleware 未启用模块接口返回 403，已启用放行
func TestModule_RequireMiddleware(t *testing.T) {
	loadModulesForTest(t, "") // ai 未启用
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 用中间件 + 探测 handler
	r.GET("/ai-test", RequireModule(ModuleAI), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// 未启用 → 403
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ai-test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("未启用模块应 403, got %d", w.Code)
	}

	// 启用后 → 200
	loadModulesForTest(t, "ai")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Errorf("启用模块应 200, got %d", w2.Code)
	}
}

// TestModule_EnabledModuleList 已启模块列表=核心+已启可选，核心始终在前
func TestModule_EnabledModuleList(t *testing.T) {
	loadModulesForTest(t, "ai,expense")
	list := EnabledModuleList()
	// 核心模块必须全部在前
	for i, c := range coreModuleKeys {
		if i < len(list) && list[i] != c {
			t.Fatalf("核心模块顺序被破坏: list[%d]=%s, 期望 %s", i, list[i], c)
		}
	}
	// 已启可选包含 ai 与 expense
	seen := map[string]bool{}
	for _, k := range list {
		seen[k] = true
	}
	if !seen[ModuleAI] || !seen[ModuleExpense] {
		t.Errorf("已启可选模块应包含 ai 与 expense, got %v", list)
	}
	if seen[ModuleInventory] {
		t.Errorf("未配置的 inventory 不应出现在列表")
	}
}

// TestModule_ListEnabledModulesAPI 接口返回模块列表
func TestModule_ListEnabledModulesAPI(t *testing.T) {
	loadModulesForTest(t, "video")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/modules", ListEnabledModules)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/modules", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("模块接口应 200, got %d", w.Code)
	}
	if !strings.Contains(body, "\"video\"") || !strings.Contains(body, "\"dashboard\"") {
		t.Errorf("模块列表应包含 core+dashboard 与可选 video, body=%s", body)
	}
}

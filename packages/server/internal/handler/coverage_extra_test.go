package handler

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// TestPureHelpersCoverage 覆盖无需外部服务的边界分支，避免质量门只依赖接口集成测试。
func TestPureHelpersCoverage(t *testing.T) {
	for _, tc := range []struct {
		ratio float64
		want  string
	}{
		{-0.1, "green"}, {0, "green"}, {0.2, "yellow_low"},
		{0.5, "yellow"}, {0.8, "orange"}, {1, "red"}, {1.2, "red"},
	} {
		if got := deriveColorLevel(tc.ratio); got != tc.want {
			t.Errorf("deriveColorLevel(%v)=%q, want %q", tc.ratio, got, tc.want)
		}
	}
	var agg roadAgg
	agg.CrossingIdsAppend(3)
	agg.CrossingIdsAppend(7)
	if len(agg.Crossings) != 2 || agg.Crossings[1] != 7 {
		t.Fatalf("路口 ID 追加失败: %+v", agg.Crossings)
	}

	for _, tc := range []struct {
		ext, want string
	}{
		{".jpg", model.MediaPhoto}, {".docx", model.MediaDoc}, {".mp4", model.MediaVideo},
	} {
		if got := categoryOf(tc.ext); got != tc.want {
			t.Errorf("categoryOf(%q)=%q, want %q", tc.ext, got, tc.want)
		}
	}
	if thumbOf(".jpg", "/a.jpg") != "/a.jpg" || thumbOf(".mp4", "/a.mp4") != "" {
		t.Error("thumbOf 分类结果错误")
	}
	for _, raw := range []string{"rtsp://host/live", "https://example.com/a.m3u8", "HTTP://example.com"} {
		if !validStreamURL(raw) {
			t.Errorf("validStreamURL(%q) 应通过", raw)
		}
	}
	for _, raw := range []string{"", "ftp://host/a", "https://", "not-url"} {
		if validStreamURL(raw) {
			t.Errorf("validStreamURL(%q) 不应通过", raw)
		}
	}
	if sanitizeHwID("../A-01_中文") != "A01" {
		t.Errorf("sanitizeHwID 清洗错误: %q", sanitizeHwID("../A-01_中文"))
	}
	for _, tc := range []struct {
		in   string
		want uint32
		err  bool
	}{
		{"", 0, false}, {"123", 123, false}, {"4294967295", ^uint32(0), false}, {"-1", 0, true}, {"4294967296", 0, true},
	} {
		got, err := parseUintStrict(tc.in)
		if got != tc.want || (err != nil) != tc.err {
			t.Errorf("parseUintStrict(%q)=(%d,%v), want (%d,%v)", tc.in, got, err, tc.want, tc.err)
		}
	}
	for _, tc := range []struct {
		ext, mime string
		want      bool
	}{
		{".jpg", "image/jpeg", true}, {".jpg", "text/plain", false},
		{".mp4", "video/mp4", true}, {".mp4", "application/octet-stream", true},
		{".pdf", "application/pdf", true}, {".pdf", "text/plain", false}, {".bin", "", true},
	} {
		if got := mimeAllowed(tc.ext, tc.mime); got != tc.want {
			t.Errorf("mimeAllowed(%q,%q)=%v, want %v", tc.ext, tc.mime, got, tc.want)
		}
	}

	for _, tc := range []struct {
		in    string
		major uint32
		minor uint32
		build uint32
		err   bool
	}{
		{"v1.2.3", 1, 2, 3, false}, {"2.4", 2, 4, 0, false}, {"bad", 0, 0, 0, true},
	} {
		major, minor, build, err := parseVersion(tc.in)
		if major != tc.major || minor != tc.minor || build != tc.build || (err != nil) != tc.err {
			t.Errorf("parseVersion(%q)=(%d,%d,%d,%v)", tc.in, major, minor, build, err)
		}
	}
	for _, ext := range []string{".bin", ".hex", ".fw", ".elf", ".img", ".dat"} {
		if !isFirmwareExt(ext) {
			t.Errorf("固件扩展名 %s 应通过", ext)
		}
	}
	if isFirmwareExt(".exe") {
		t.Error(".exe 不应作为固件扩展名")
	}
	tmp, err := os.CreateTemp("", "tsloms-md5-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmp.Name()
	_, _ = tmp.WriteString("tsloms")
	_ = tmp.Close()
	t.Cleanup(func() { _ = os.Remove(tmpName) })
	if fileMD5(tmpName) == "" || fileMD5(tmpName+".missing") != "" {
		t.Error("fileMD5 分支错误")
	}

	if demoHWID(4) == "" || mqttFaultType(-1) != "lamp_off" || mqttFaultType(-14) != "power_loss" || mqttFaultType(0) != "unknown" {
		t.Error("演示辅助函数结果错误")
	}
	v := f64ptr(1.25)
	if v == nil || *v != 1.25 {
		t.Error("f64ptr 结果错误")
	}

	if op, _, ok := ParseStatusFilter("active"); !ok || op != "IN" {
		t.Error("active 状态筛选解析错误")
	}
	if op, arg, ok := ParseStatusFilter("resolved"); !ok || op != "=" || arg != "resolved" {
		t.Error("普通状态筛选解析错误")
	}
	if _, _, ok := ParseStatusFilter(""); ok {
		t.Error("空状态不应启用筛选")
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?start_date=2026-01-01&end_date=2026-01-02", nil)
	start, end := ParseFaultTimeRange(ctx)
	if start == nil || end == nil || !end.After(*start) {
		t.Error("故障时间范围解析错误")
	}

	// 验证码覆盖：正确、错误、一次性失效和过期分支。
	uuid, _, answer := generateCaptcha()
	if verifyCaptcha(uuid, "bad") || !verifyCaptcha(uuid, ""+itoaForTest(answer)) {
		t.Error("验证码校验分支错误")
	}
	if verifyCaptcha(uuid, itoaForTest(answer)) {
		t.Error("验证码通过后应一次性失效")
	}
	captchaStore.Lock()
	captchaStore.m["expired-test"] = &captchaEntry{Answer: 1, ExpiresAt: time.Now().Add(-time.Minute)}
	captchaStore.Unlock()
	if verifyCaptcha("expired-test", "1") {
		t.Error("过期验证码不应通过")
	}

	// 模块集合解析是纯内存逻辑；恢复环境变量，避免影响其他测试。
	parseEnabledModulesFrom("ai, inventory")
	if !isCoreModule(ModuleDevice) || isCoreModule(ModuleAI) || !enabledSet[ModuleAI] {
		t.Error("模块集合解析错误")
	}
	if len(EnabledModuleInfos()) == 0 {
		t.Error("应至少返回核心模块")
	}
	roleCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if roleFromCtx(roleCtx) != "" {
		t.Error("无角色上下文应返回空")
	}
	roleCtx.Set("user_role", model.RoleAdmin)
	if roleFromCtx(roleCtx) != model.RoleAdmin {
		t.Error("角色上下文读取失败")
	}
	ls := &model.LicenseState{}
	ensureModuleTrial(ls, ModuleAI, time.Now(), 30)
}

func TestModuleSettingsAndFaultExportCoverage(t *testing.T) {
	r := covSetup(t)
	r.GET("/modules/settings", ListModuleSettings)
	r.PUT("/modules/settings", UpdateModuleSettings)
	r.GET("/faults/export", ExportFaults)

	code, body := doReq(t, r, "GET", "/modules/settings", "")
	mustOK(t, code, body, "模块设置列表")
	code, _ = doReq(t, r, "PUT", "/modules/settings", `{}`)
	if code != 400 {
		t.Errorf("模块参数缺失应 400, got %d", code)
	}
	code, _ = doReq(t, r, "PUT", "/modules/settings", `{"module_key":"device","enabled":false}`)
	if code != 400 {
		t.Errorf("核心模块不可关闭，应 400，got %d", code)
	}
	code, body = doReq(t, r, "PUT", "/modules/settings", `{"module_key":"ai","enabled":true}`)
	mustOK(t, code, body, "启用 AI 模块")
	code, body = doReq(t, r, "PUT", "/modules/settings", `{"module_key":"ai","enabled":false}`)
	mustOK(t, code, body, "关闭 AI 模块")

	first := time.Now().Add(-time.Hour)
	last := time.Now()
	model.DB.Create(&model.FaultRecord{DeviceHwID: "cov-device", ErrCode: -1, FaultType: "lamp_off", FaultLevel: "critical", Status: model.FaultStatusOccurred, FirstSeen: first, LastSeen: last})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/faults/export?status=active", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 || len(w.Body.String()) < 20 {
		t.Fatalf("故障导出失败: code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestCrossingDetailValidationCoverage(t *testing.T) {
	r := covSetup(t)
	r.GET("/map/crossing/:id/detail", CrossingDetail)
	r.GET("/crossings/:id", GetCrossing)
	r.GET("/map/road-data", GetRoadMapData)
	for _, path := range []string{"/map/crossing/abc/detail", "/map/crossing/99999/detail"} {
		code, _ := doReq(t, r, "GET", path, "")
		if code != 400 && code != 404 {
			t.Errorf("路口详情 %s 应返回 400/404，got %d", path, code)
		}
	}
	x := model.Crossing{PointNo: "COV-1", Name: "覆盖路口", RoadName: "测试路"}
	model.DB.Create(&x)
	RecomputeCrossingCache()
	code, body := doReq(t, r, "GET", "/map/road-data", "")
	if code != 200 || body["code"].(float64) != 0 {
		t.Fatalf("道路地图聚合失败: code=%d body=%v", code, body)
	}
	code, body = doReq(t, r, "GET", "/crossings/"+uid(x.ID), "")
	if code != 200 || body["code"].(float64) != 0 {
		t.Fatalf("获取路口失败: code=%d body=%v", code, body)
	}
	code, _ = doReq(t, r, "GET", "/crossings/abc", "")
	if code != 400 {
		t.Errorf("非法路口 ID 应 400，got %d", code)
	}
	code, body = doReq(t, r, "GET", "/map/crossing/"+uid(x.ID)+"/detail", "")
	if code != 200 || body["code"].(float64) != 0 {
		t.Fatalf("有效路口详情失败: code=%d body=%v", code, body)
	}
}

func itoaForTest(v int) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 4)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}

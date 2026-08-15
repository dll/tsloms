package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// setupFirmwareEngine 构造固件相关路由的测试引擎
func setupFirmwareEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/firmwares", ListFirmwares)
		g.POST("/firmwares", UploadFirmware)
		g.GET("/firmwares/:id", GetFirmware)
		g.PUT("/firmwares/:id", UpdateFirmware)
		g.POST("/firmwares/:id/publish", PublishFirmware)
		g.DELETE("/firmwares/:id", DeleteFirmware)
		g.GET("/firmware-upgrades", ListFirmwareUpgrades)
		g.POST("/firmware-upgrades", CreateFirmwareUpgrade)
		g.DELETE("/firmware-upgrades/:id", DeleteFirmwareUpgrade)
	}
	return r
}

func seedFirmware(v string) model.FirmwarePackage {
	major, minor, build, _ := parseVersion(v)
	f := model.FirmwarePackage{
		Version: v, Major: major, Minor: minor, Build: build,
		SwVersion: (major << 28) | (minor << 24),
		FileName:  v + ".bin", FilePath: "firmware/" + v + ".bin",
		Size: 100, MD5: "abc", Description: "desc", Published: false,
		Uploader: "admin",
	}
	model.DB.Create(&f)
	return f
}

func TestFirmware_ListAndFilter(t *testing.T) {
	r := setupFirmwareEngine(t)
	seedFirmware("v1.2.3")
	seedFirmware("v2.0.0")
	model.DB.Model(&model.FirmwarePackage{}).Where("version = ?", "v2.0.0").Update("published", true)

	code, body := doReq(t, r, "GET", "/api/v1/firmwares", "")
	if code != http.StatusOK || body["code"].(float64) != 0 {
		t.Fatalf("code=%d body=%v", code, body)
	}
	if body["data"].(map[string]interface{})["total"].(float64) != 2 {
		t.Errorf("total 期望 2, got %v", body["data"].(map[string]interface{})["total"])
	}
	_, b2 := doReq(t, r, "GET", "/api/v1/firmwares?published=true", "")
	if b2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("published 筛选 total 期望 1, got %v", b2["data"].(map[string]interface{})["total"])
	}
}

func TestFirmware_GetNotFound(t *testing.T) {
	r := setupFirmwareEngine(t)
	code, _ := doReq(t, r, "GET", "/api/v1/firmwares/999", "")
	if code != http.StatusNotFound {
		t.Errorf("不存在固件应 404, got %d", code)
	}
	code2, _ := doReq(t, r, "GET", "/api/v1/firmwares/abc", "")
	if code2 != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code2)
	}
}

func TestFirmware_GetOk(t *testing.T) {
	r := setupFirmwareEngine(t)
	f := seedFirmware("v1.0.1")
	code, body := doReq(t, r, "GET", "/api/v1/firmwares/"+uid(f.ID), "")
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	fw := body["data"].(map[string]interface{})["firmware"].(map[string]interface{})
	if fw["version"] != "v1.0.1" {
		t.Errorf("version=%v", fw["version"])
	}
	if fw["major"].(float64) != 1 || fw["minor"].(float64) != 0 || fw["build"].(float64) != 1 {
		t.Errorf("版本位解析错误: %v", fw)
	}
}

func TestFirmware_UpdateDescription(t *testing.T) {
	r := setupFirmwareEngine(t)
	f := seedFirmware("v1.0.1")
	code, _ := doReq(t, r, "PUT", "/api/v1/firmwares/"+uid(f.ID), `{"description":"新描述"}`)
	if code != http.StatusOK {
		t.Fatalf("update 失败 code=%d", code)
	}
	var after model.FirmwarePackage
	model.DB.First(&after, f.ID)
	if after.Description != "新描述" {
		t.Errorf("描述未更新: %q", after.Description)
	}
}

func TestFirmware_UpdateVersion(t *testing.T) {
	r := setupFirmwareEngine(t)
	f := seedFirmware("v1.0.1")
	code, _ := doReq(t, r, "PUT", "/api/v1/firmwares/"+uid(f.ID), `{"version":"v1.1.1"}`)
	if code != http.StatusOK {
		t.Fatalf("改版本失败 code=%d", code)
	}
	seedFirmware("v9.9.9")
	code2, _ := doReq(t, r, "PUT", "/api/v1/firmwares/"+uid(f.ID), `{"version":"v9.9.9"}`)
	if code2 != http.StatusBadRequest {
		t.Errorf("重复版本应 400, got %d", code2)
	}
	code3, _ := doReq(t, r, "PUT", "/api/v1/firmwares/"+uid(f.ID), `{"version":"not-a-version"}`)
	if code3 != http.StatusBadRequest {
		t.Errorf("非法版本应 400, got %d", code3)
	}
	code4, _ := doReq(t, r, "PUT", "/api/v1/firmwares/888", `{"version":"v1.1.1"}`)
	if code4 != http.StatusNotFound {
		t.Errorf("更新不存在固件应 404, got %d", code4)
	}
}

func TestFirmware_PublishAndUnpublish(t *testing.T) {
	r := setupFirmwareEngine(t)
	f := seedFirmware("v3.0.0")
	code, _ := doReq(t, r, "POST", "/api/v1/firmwares/"+uid(f.ID)+"/publish", `{"published":true}`)
	if code != http.StatusOK {
		t.Fatalf("发布失败 code=%d", code)
	}
	var after model.FirmwarePackage
	model.DB.First(&after, f.ID)
	if !after.Published {
		t.Error("发布后 published 应为 true")
	}
	code, _ = doReq(t, r, "POST", "/api/v1/firmwares/"+uid(f.ID)+"/publish", `{"published":false}`)
	if code != http.StatusOK {
		t.Fatalf("下线失败 code=%d", code)
	}
	model.DB.First(&after, f.ID)
	if after.Published {
		t.Error("下线后 published 应为 false")
	}
	code2, _ := doReq(t, r, "POST", "/api/v1/firmwares/abc/publish", `{"published":true}`)
	if code2 != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code2)
	}
	code3, _ := doReq(t, r, "POST", "/api/v1/firmwares/777/publish", `{"published":true}`)
	if code3 != http.StatusNotFound {
		t.Errorf("不存在固件发布应 404, got %d", code3)
	}
}

func TestFirmware_Delete(t *testing.T) {
	r := setupFirmwareEngine(t)
	f := seedFirmware("v4.0.0")
	code, _ := doReq(t, r, "DELETE", "/api/v1/firmwares/"+uid(f.ID), "")
	if code != http.StatusOK {
		t.Fatalf("删除失败 code=%d", code)
	}
	var cnt int64
	model.DB.Model(&model.FirmwarePackage{}).Where("id = ?", f.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("删除后仍存在 %d 条", cnt)
	}
	code2, _ := doReq(t, r, "DELETE", "/api/v1/firmwares/"+uid(f.ID), "")
	if code2 != http.StatusNotFound {
		t.Errorf("重复删除应 404, got %d", code2)
	}
}

func TestFirmwareUpload_Multipart(t *testing.T) {
	r := setupFirmwareEngine(t)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("version", "v5.0.0")
	_ = w.WriteField("description", "新固件")
	part, _ := w.CreateFormFile("file", "test.bin")
	_, _ = part.Write([]byte{0x01, 0x02, 0x03})
	_ = w.Close()

	req := httptest.NewRequest("POST", "/api/v1/firmwares", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("上传失败 code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFirmware_Upgrades(t *testing.T) {
	r := setupFirmwareEngine(t)
	fw := seedFirmware("v2.0.0")
	model.DB.Model(&model.FirmwarePackage{}).Where("id = ?", fw.ID).Update("published", true)
	dev := model.Device{HwID: 42, OnlineStatus: true}
	model.DB.Create(&dev)

	code, body := doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{"device_hw_id":42,"firmware_id":`+uid(fw.ID)+`}`)
	if code != http.StatusOK {
		t.Fatalf("创建升级失败 code=%d body=%v", code, body)
	}
	recID := uint(0)
	if data, ok := body["data"].(map[string]interface{}); ok {
		recID = uint(data["record"].(float64))
	}
	_, b2 := doReq(t, r, "GET", "/api/v1/firmware-upgrades?device_hw_id=42", "")
	if b2["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("升级列表 total 期望 1, got %v", b2["data"].(map[string]interface{})["total"])
	}
	code3, _ := doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{"device_hw_id":42,"firmware_id":`+uid(fw.ID)+`}`)
	if code3 != http.StatusBadRequest {
		t.Errorf("重复升级应 400, got %d", code3)
	}
	code4, _ := doReq(t, r, "DELETE", "/api/v1/firmware-upgrades/"+uid(recID), "")
	if code4 != http.StatusOK {
		t.Errorf("删除升级应 200, got %d", code4)
	}
}

func TestFirmware_UpgradeValidation(t *testing.T) {
	r := setupFirmwareEngine(t)
	code, _ := doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{"device_hw_id":999,"firmware_id":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("设备不存在应 400, got %d", code)
	}
	dev := model.Device{HwID: 55, OnlineStatus: true}
	model.DB.Create(&dev)
	code, _ = doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{"device_hw_id":55,"firmware_id":999}`)
	if code != http.StatusNotFound {
		t.Errorf("固件不存在应 404, got %d", code)
	}
	fw := seedFirmware("v6.0.0")
	code, _ = doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{"device_hw_id":55,"firmware_id":`+uid(fw.ID)+`}`)
	if code != http.StatusBadRequest {
		t.Errorf("未发布固件应 400, got %d", code)
	}
	code, _ = doReq(t, r, "POST", "/api/v1/firmware-upgrades", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	code, _ = doReq(t, r, "DELETE", "/api/v1/firmware-upgrades/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法删除ID应 400, got %d", code)
	}
	code, _ = doReq(t, r, "DELETE", "/api/v1/firmware-upgrades/555", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在升级应 404, got %d", code)
	}
}

func TestFirmware_Helpers(t *testing.T) {
	maj, min, bld, err := parseVersion("v1.2.30")
	if err != nil || maj != 1 || min != 2 || bld != 30 {
		t.Errorf("parseVersion v1.2.30 = %d.%d.%d err=%v", maj, min, bld, err)
	}
	maj, min, bld, _ = parseVersion("1.2")
	if maj != 1 || min != 2 || bld != 0 {
		t.Errorf("parseVersion 1.2 = %d.%d.%d", maj, min, bld)
	}
	if _, _, _, err := parseVersion("bad"); err == nil {
		t.Error("非法版本应报错")
	}
	for _, ok := range []struct {
		ext  string
		want bool
	}{
		{".bin", true}, {".hex", true}, {".fw", true}, {".elf", true}, {".img", true}, {".dat", true},
		{".txt", false}, {".zip", false}, {".pdf", false}, {"", false},
	} {
		if got := isFirmwareExt(ok.ext); got != ok.want {
			t.Errorf("isFirmwareExt(%q)=%v want %v", ok.ext, got, ok.want)
		}
	}
	if v := firmwareView(model.FirmwarePackage{ID: 1, Version: "x"}); v["version"] != "x" {
		t.Errorf("firmwareView %v", v)
	}
}

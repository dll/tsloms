package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

func mediaEngine(t *testing.T) *gin.Engine {
	t.Helper()
	// 临时媒体目录避免污染仓库
	oldMedia := config.Get().MediaDir
	oldPrefix := config.Get().MediaURLPrefix
	t.Cleanup(func() {
		config.Get().MediaDir = oldMedia
		config.Get().MediaURLPrefix = oldPrefix
	})
	config.Get().MediaDir = filepath.Join(t.TempDir(), "media")
	config.Get().MediaURLPrefix = "/media"

	model.InitTestDB()
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/media", ListDeviceMedia)
		g.POST("/media/upload", UploadDeviceMedia)
		g.POST("/media/streams", CreateRTSPMedia)
		g.DELETE("/media/:id", DeleteDeviceMedia)
	}
	return r
}

func TestMedia_List(t *testing.T) {
	r := mediaEngine(t)
	model.DB.Create(&model.DeviceMedia{DeviceHwID: "11", MediaType: model.MediaEvidence, Category: model.MediaPhoto, Source: model.MediaSourceUpload})
	model.DB.Create(&model.DeviceMedia{DeviceHwID: "22", MediaType: model.MediaMonitoring, Category: model.MediaVideo, Source: model.MediaSourceRTSP})

	for _, q := range []string{"", "?device_hw_id=11", "?media_type=evidence", "?source=upload"} {
		code, body := doReq(t, r, "GET", "/api/v1/media"+q, "")
		mustOK(t, code, body, "媒体列表 "+q)
	}
}

func TestMedia_Upload(t *testing.T) {
	r := mediaEngine(t)
	// 成功上传图片（举证需路口）
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("device_hw_id", "11")
	_ = w.WriteField("media_type", "evidence")
	_ = w.WriteField("intersection", "人民路口")
	_ = w.WriteField("light_color", "red")
	part, _ := w.CreateFormFile("file", "photo.jpg")
	_, _ = part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0}) // JPEG 魔数
	_ = w.Close()
	req := httptest.NewRequest("POST", "/api/v1/media/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("上传失败 code=%d body=%s", rr.Code, rr.Body.String())
	}
	// 缺路口（举证必填）→ 400
	body2 := &bytes.Buffer{}
	w2 := multipart.NewWriter(body2)
	_ = w2.WriteField("device_hw_id", "11")
	part2, _ := w2.CreateFormFile("file", "a.jpg")
	_, _ = part2.Write([]byte{0xFF, 0xD8})
	_ = w2.Close()
	req2 := httptest.NewRequest("POST", "/api/v1/media/upload", body2)
	req2.Header.Set("Content-Type", w2.FormDataContentType())
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("缺路口应 400, got %d", rr2.Code)
	}
	// 非法 hw_id（含字母）→ 400
	body3 := &bytes.Buffer{}
	w3 := multipart.NewWriter(body3)
	_ = w3.WriteField("device_hw_id", "11abc")
	_ = w3.WriteField("intersection", "路口")
	part3, _ := w3.CreateFormFile("file", "a.jpg")
	_, _ = part3.Write([]byte{0xFF, 0xD8})
	_ = w3.Close()
	req3 := httptest.NewRequest("POST", "/api/v1/media/upload", body3)
	req3.Header.Set("Content-Type", w3.FormDataContentType())
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusBadRequest {
		t.Errorf("非法hw_id应 400, got %d", rr3.Code)
	}
	// 不支持扩展名 → 400
	body4 := &bytes.Buffer{}
	w4 := multipart.NewWriter(body4)
	_ = w4.WriteField("device_hw_id", "11")
	_ = w4.WriteField("intersection", "路口")
	part4, _ := w4.CreateFormFile("file", "malware.exe")
	_, _ = part4.Write([]byte{0x4D, 0x5A})
	_ = w4.Close()
	req4 := httptest.NewRequest("POST", "/api/v1/media/upload", body4)
	req4.Header.Set("Content-Type", w4.FormDataContentType())
	rr4 := httptest.NewRecorder()
	r.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusBadRequest {
		t.Errorf("非法扩展名应 400, got %d", rr4.Code)
	}
	// 无文件 → 400
	body5 := &bytes.Buffer{}
	w5 := multipart.NewWriter(body5)
	_ = w5.WriteField("device_hw_id", "11")
	_ = w5.Close()
	req5 := httptest.NewRequest("POST", "/api/v1/media/upload", body5)
	req5.Header.Set("Content-Type", w5.FormDataContentType())
	rr5 := httptest.NewRecorder()
	r.ServeHTTP(rr5, req5)
	if rr5.Code != http.StatusBadRequest {
		t.Errorf("无文件应 400, got %d", rr5.Code)
	}
}

func TestMedia_CreateRTSP(t *testing.T) {
	r := mediaEngine(t)
	model.DB.Create(&model.Device{HwID: "33", Intersection: "监控路口", OnlineStatus: true})
	// 成功（rtsp，无兼容地址 → warning）
	code, body := doReq(t, r, "POST", "/api/v1/media/streams", `{"device_hw_id":"33","media_type":"monitoring","title":"监控","url":"rtsp://192.168.1.1:554/stream"}`)
	mustOK(t, code, body, "RTSP登记")
	// 缺参数
	code, _ = doReq(t, r, "POST", "/api/v1/media/streams", `{}`)
	if code != http.StatusBadRequest {
		t.Errorf("缺参数应 400, got %d", code)
	}
	// 非法 media_type
	code, _ = doReq(t, r, "POST", "/api/v1/media/streams", `{"device_hw_id":"33","media_type":"evidence","url":"rtsp://x/stream"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法media_type应 400, got %d", code)
	}
	// 非法 url
	code, _ = doReq(t, r, "POST", "/api/v1/media/streams", `{"device_hw_id":"33","media_type":"monitoring","url":"not-a-url"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法url应 400, got %d", code)
	}
	// 非法 compatible_url
	code, _ = doReq(t, r, "POST", "/api/v1/media/streams", `{"device_hw_id":"33","media_type":"monitoring","url":"rtsp://x/stream","compatible_url":"bad"}`)
	if code != http.StatusBadRequest {
		t.Errorf("非法compatible_url应 400, got %d", code)
	}
	// 设备不存在
	code, _ = doReq(t, r, "POST", "/api/v1/media/streams", `{"device_hw_id":"999","media_type":"monitoring","url":"rtsp://x/stream"}`)
	if code != http.StatusBadRequest {
		t.Errorf("设备不存在应 400, got %d", code)
	}
}

func TestMedia_Delete(t *testing.T) {
	r := mediaEngine(t)
	// 上传的图片（可删文件）
	m := model.DeviceMedia{DeviceHwID: "11", Source: model.MediaSourceUpload, URL: "/media/202601/a.jpg"}
	model.DB.Create(&m)
	code, _ := doReq(t, r, "DELETE", "/api/v1/media/"+uid(m.ID), "")
	mustOK(t, code, map[string]interface{}{"code": float64(0)}, "删除媒体")
	// 不存在
	code, _ = doReq(t, r, "DELETE", "/api/v1/media/99999", "")
	if code != http.StatusNotFound {
		t.Errorf("删除不存在应 404, got %d", code)
	}
	// 非法ID
	code, _ = doReq(t, r, "DELETE", "/api/v1/media/abc", "")
	if code != http.StatusBadRequest {
		t.Errorf("非法ID应 400, got %d", code)
	}
}

func TestMedia_Helpers(t *testing.T) {
	// categoryOf
	if categoryOf(".jpg") != model.MediaPhoto || categoryOf(".png") != model.MediaPhoto || categoryOf(".mp4") != model.MediaVideo {
		t.Error("categoryOf 错误")
	}
	// thumbOf
	if thumbOf(".jpg", "/media/x.jpg") != "/media/x.jpg" {
		t.Error("thumbOf 图片应返回自身")
	}
	if thumbOf(".mp4", "/media/x.mp4") != "" {
		t.Error("thumbOf 视频应空")
	}
	// validStreamURL
	for _, ok := range []struct {
		u    string
		want bool
	}{
		{"rtsp://192.168.1.1/stream", true},
		{"https://example.com/x.m3u8", true},
		{"ftp://bad", false},
		{"not-url", false},
		{"rtsp://", false}, // 无 host
	} {
		if got := validStreamURL(ok.u); got != ok.want {
			t.Errorf("validStreamURL(%q)=%v want %v", ok.u, got, ok.want)
		}
	}
	// parseUintStrict
	if v, err := parseUintStrict(""); err != nil || v != 0 {
		t.Errorf("parseUintStrict 空 = %v, %v", v, err)
	}
	if v, _ := parseUintStrict("123"); v != 123 {
		t.Errorf("parseUintStrict 123 = %v", v)
	}
	if _, err := parseUintStrict("12a"); err == nil {
		t.Error("parseUintStrict 非法应报错")
	}
	// mimeAllowed
	if !mimeAllowed(".jpg", "image/jpeg") {
		t.Error("jpg+image/jpeg 应允许")
	}
	if mimeAllowed(".jpg", "text/html") {
		t.Error("jpg+text/html 应拒绝")
	}
	if !mimeAllowed(".mp4", "video/mp4") {
		t.Error("mp4+video/mp4 应允许")
	}
	if !mimeAllowed(".jpg", "") {
		t.Error("空MIME 应允许")
	}
	// MediaDir / mediaURLPrefix 默认
	oldD, oldP := config.Get().MediaDir, config.Get().MediaURLPrefix
	config.Get().MediaDir = ""
	config.Get().MediaURLPrefix = ""
	if MediaDir() == "" || mediaURLPrefix() == "" {
		t.Error("MediaDir/mediaURLPrefix 默认应非空")
	}
	config.Get().MediaDir, config.Get().MediaURLPrefix = oldD, oldP
}

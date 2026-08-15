package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// covSetup 统一初始化测试引擎（内存 SQLite），返回 gin 引擎
func covSetup(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	model.InitTestDB()
	return gin.New()
}

// doReq 发送 JSON/空请求到测试引擎，返回状态码 + 解析后的 body
func doReq(t *testing.T, r *gin.Engine, method, path, body string) (int, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req, _ = http.NewRequest(method, path, nil)
	} else {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return w.Code, out
}

// uid 无符号整数转字符串
func uid(n uint) string { return strconv.FormatUint(uint64(n), 10) }

// mustOK 断言 code==0，否则 Fatal
func mustOK(t *testing.T, code int, body map[string]interface{}, ctx string) {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("%s: HTTP %d body=%v", ctx, code, body)
	}
	if body["code"].(float64) != 0 {
		t.Fatalf("%s: 业务 code=%v (%v)", ctx, body["code"], body["message"])
	}
}

// now 当前时间（测试用，避免重复 import time）
func now() time.Time { return time.Now() }

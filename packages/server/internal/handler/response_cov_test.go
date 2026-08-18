package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 覆盖 response.go 的响应构造与分页纯函数。

func TestResponse_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/t", func(c *gin.Context) { ok(c, gin.H{"a": 1}) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	for _, key := range []string{`"code":0`, `"msg":"success"`, `"a":1`} {
		found := contains(body, key)
		if !found {
			t.Errorf("ok 输出缺少 %q: %s", key, body)
		}
	}
}

func TestResponse_Health(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/h", Health)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/h", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("Health 缺 status: %s", w.Body.String())
	}
}

func TestResponse_failHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		call   func(*gin.Context)
		status int
		errKey string
	}{
		{"badRequest", func(c *gin.Context) { badRequest(c, "bad") }, http.StatusBadRequest, "bad_request"},
		{"unauthorized", func(c *gin.Context) { unauthorized(c, "unauth") }, http.StatusUnauthorized, "unauthorized"},
		{"forbidden", func(c *gin.Context) { forbidden(c, "no") }, http.StatusForbidden, "forbidden"},
		{"notFound", func(c *gin.Context) { notFound(c, "none") }, http.StatusNotFound, "not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/e", c.call)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/e", nil))
			if w.Code != c.status {
				t.Fatalf("status=%d, want %d", w.Code, c.status)
			}
			if !contains(w.Body.String(), c.errKey) {
				t.Errorf("输出缺 %q: %s", c.errKey, w.Body.String())
			}
			if !contains(w.Body.String(), `"code":-1`) {
				t.Errorf("失败响应应为 code:-1: %s", w.Body.String())
			}
		})
	}
}

func TestResponse_serverError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/e", func(c *gin.Context) { serverError(c, nil) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/e", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	if !contains(w.Body.String(), "服务器内部错误") {
		t.Errorf("serverError 应回显通用文案: %s", w.Body.String())
	}
	// 非 nil error + 非生产 → 回显具体错误
	gin.SetMode(gin.TestMode)
	r2 := gin.New()
	r2.GET("/e2", func(c *gin.Context) { serverError(c, innerTestErr("boom")) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/e2", nil))
	_ = w2
}

type innerTestErr string

func (e innerTestErr) Error() string { return string(e) }

func TestResponse_parseUint(t *testing.T) {
	if v, err := parseUint("0"); err != nil || v != 0 {
		t.Fatalf("parseUint(0) = %d,%v", v, err)
	}
	if v, err := parseUint("42"); err != nil || v != 42 {
		t.Fatalf("parseUint(42) = %d,%v", v, err)
	}
	if _, err := parseUint("abc"); err == nil {
		t.Fatal("parseUint(abc) 应报错")
	}
	if _, err := parseUint("-1"); err == nil {
		t.Fatal("parseUint(-1) 应报错")
	}
}

func TestResponse_paginate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCtx := func(query string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/p"+query, nil)
		return c
	}
	// 默认
	p, ps := paginate(newCtx(""))
	if p != 1 || ps != defaultPageSize {
		t.Errorf("默认 p=%d ps=%d", p, ps)
	}
	// 显式合法值
	p, ps = paginate(newCtx("?page=3&page_size=50"))
	if p != 3 || ps != 50 {
		t.Errorf("合法 p=%d ps=%d", p, ps)
	}
	// page=0 → 兜底 1
	p, _ = paginate(newCtx("?page=0"))
	if p != 1 {
		t.Errorf("page=0 应回退 1, got %d", p)
	}
	// page_size 越界（0 和 120）
	_, ps = paginate(newCtx("?page_size=0"))
	if ps != defaultPageSize {
		t.Errorf("page_size=0 应回退默认, got %d", ps)
	}
	_, ps = paginate(newCtx("?page_size=120"))
	if ps != maxPageSize {
		t.Errorf("page_size=120 应截断到 100, got %d", ps)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

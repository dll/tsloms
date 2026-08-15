package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newCtx(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("GET", "/api/v1/x?"+query, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func TestPaginate(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantPage  uint
		wantSize  uint
	}{
		{name: "默认值", query: "", wantPage: 1, wantSize: 20},
		{name: "正常分页", query: "page=3&page_size=25", wantPage: 3, wantSize: 25},
		{name: "page为0回退", query: "page=0&page_size=10", wantPage: 1, wantSize: 10},
		{name: "page为负回退", query: "page=-5&page_size=10", wantPage: 1, wantSize: 10},
		{name: "page非数字回退", query: "page=abc&page_size=10", wantPage: 1, wantSize: 10},
		{name: "page_size为0回退", query: "page=2&page_size=0", wantPage: 2, wantSize: 20},
		{name: "page_size超上限100回退到100", query: "page=2&page_size=1000", wantPage: 2, wantSize: 100},
		{name: "page_size非数字回退", query: "page=2&page_size=xyz", wantPage: 2, wantSize: 20},
		{name: "恰好100不裁切", query: "page=1&page_size=100", wantPage: 1, wantSize: 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := newCtx(c.query)
			page, size := paginate(ctx)
			if page != c.wantPage || size != c.wantSize {
				t.Fatalf("paginate(%q) = (%d,%d), want (%d,%d)", c.query, page, size, c.wantPage, c.wantSize)
			}
		})
	}
}

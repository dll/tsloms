package handler

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// baiduTileProxy 百度瓦片代理
// 百度 maponline*.bdimg.com 对跨域浏览器请求返回 403（无 CORS 头），
// 无法被 Cesium 直接加载。本代理由后端转发（服务器请求无 Origin 限制，百度返回 200），
// 前端改为加载同源代理地址，从而正常显示百度底图。
func BaiduTileProxy(c *gin.Context) {
	x := c.Query("x")
	y := c.Query("y")
	z := c.Query("z")
	if x == "" || y == "" || z == "" {
		badRequest(c, "缺少 x/y/z 参数")
		return
	}

	// 轮询百度子域名
	sub := (mustParseInt(x) + mustParseInt(y)) % 4
	tileURL := fmt.Sprintf("https://maponline%d.bdimg.com/tile/?qt=vtile&x=%s&y=%s&z=%s&styles=pl&scaler=1",
		sub, x, y, z)

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		serverError(c, err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TSLOMS/1.0)")
	// 不设置 Origin/Referer，避免百度 403

	resp, err := client.Do(req)
	if err != nil {
		serverError(c, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Status(resp.StatusCode)
		return
	}

	// 透传图片
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "public, max-age=86400") // 瓦片缓存 1 天
	c.Status(http.StatusOK)
	io.Copy(c.Writer, resp.Body)
}

// gaodeTileProxy 高德瓦片代理（备用：若个别高德端子域对浏览器受限）
func GaodeTileProxy(c *gin.Context) {
	x := c.Query("x")
	y := c.Query("y")
	z := c.Query("z")
	style := c.DefaultQuery("style", "8")
	if x == "" || y == "" || z == "" {
		badRequest(c, "缺少 x/y/z 参数")
		return
	}
	sub := mustParseInt(x)%4 + 1
	host := "wprd" // 路网（current web raster endpoint; webrd 已废弃返回 404）
	if style == "6" {
		host = "webst" // 卫星
	}
	tileURL := fmt.Sprintf("https://%s%02d.is.autonavi.com/appmaptile?style=%s&x=%s&y=%s&z=%s", host, sub, style, x, y, z)

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", tileURL, nil)
	if err != nil {
		serverError(c, err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TSLOMS/1.0)")
	req.Header.Set("Referer", "https://www.autonavi.com/")

	resp, err := client.Do(req)
	if err != nil {
		serverError(c, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.Status(resp.StatusCode)
		return
	}
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "public, max-age=86400")
	c.Status(http.StatusOK)
	io.Copy(c.Writer, resp.Body)
}

// mustParseInt 简化整数解析（代理参数来自受控调用，出错返回 0）
func mustParseInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

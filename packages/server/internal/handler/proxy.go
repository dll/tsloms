package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tsloms/server/internal/config"
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

// AmapPlaceSearch 高德 POI 地名搜索代理
// 瓦片底图无需 key（公开端点），但搜索/地理编码需高德 Web 服务 Key（restapi.amap.com）。
// 本接口在服务端持有 key，前端无 key；未配置 key 时返回可识别错误由前端降级为本地搜索。
// GET /proxy/amap/place?kw=人民路&city=上海&loc=121.47,31.23
func AmapPlaceSearch(c *gin.Context) {
	kw := strings.TrimSpace(c.Query("kw"))
	if kw == "" {
		badRequest(c, "缺少搜索关键字")
		return
	}
	key := config.Get().AMapWebKey
	if key == "" {
		ok(c, gin.H{"pois": []gin.H{}, "fallback": true, "message": "未配置 AMAP_WEB_KEY，使用本地点位搜索"})
		return
	}

	q := url.Values{}
	q.Set("key", key)
	q.Set("keywords", kw)
	q.Set("output", "json")
	// 可选：城市约束 / 周边搜索半径
	if city := c.Query("city"); city != "" {
		q.Set("city", city)
	}
	if loc := c.Query("loc"); loc != "" {
		q.Set("location", loc)
		q.Set("radius", c.DefaultQuery("radius", "10000"))
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://restapi.amap.com/v3/place/text?" + q.Encode())
	if err != nil {
		serverError(c, err)
		return
	}
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
		Info   string `json:"info"`
		Pois   []struct {
			Name     string `json:"name"`
			Location string `json:"location"` // "lng,lat"
			Address  string `json:"address"`
			Type     string `json:"type"`
		} `json:"pois"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		serverError(c, err)
		return
	}

	out := make([]gin.H, 0, len(body.Pois))
	for _, p := range body.Pois {
		parts := strings.Split(p.Location, ",")
		if len(parts) != 2 {
			continue
		}
		lng, err1 := strconv.ParseFloat(parts[0], 64)
		lat, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, gin.H{"name": p.Name, "lat": lat, "lng": lng, "address": p.Address})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"list": out, "source": "amap"}})
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

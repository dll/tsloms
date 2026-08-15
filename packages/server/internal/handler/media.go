package handler

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// MediaDir 媒体文件存储根目录（可经环境变量 MEDIA_DIR 配置）
func MediaDir() string {
	dir := config.Get().MediaDir
	if dir == "" {
		dir = "./uploads/media"
	}
	return dir
}

// mediaURLBase 媒体对外访问前缀（经 /media/* 反代；默认拼接主机）
func mediaURLPrefix() string {
	p := config.Get().MediaURLPrefix
	if p == "" {
		p = "/media"
	}
	return p
}

// ListDeviceMedia 查询设备媒体列表
// 支持按设备、媒体类型筛选，分页
func ListDeviceMedia(c *gin.Context) {
	page, pageSize := paginate(c)

	query := model.DB.Model(&model.DeviceMedia{})
	if hwID := c.Query("device_hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}
	if mt := c.Query("media_type"); mt != "" {
		query = query.Where("media_type = ?", mt)
	}
	if src := c.Query("source"); src != "" {
		query = query.Where("source = ?", src)
	}

	var total int64
	query.Count(&total)

	var list []model.DeviceMedia
	query.Order("created_at DESC").
		Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	ok(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// UploadDeviceMedia 上传设备媒体（手机短视频举证/图片）
// multipart: device_hw_id, media_type, title, note, file
func UploadDeviceMedia(c *gin.Context) {
	hwID := c.PostForm("device_hw_id")
	mediaType := c.PostForm("media_type")
	if mediaType == "" {
		mediaType = model.MediaEvidence
	}
	title := c.PostForm("title")
	note := c.PostForm("note")

	// 信号灯信息（举证/上传必填路口，便于定位路口与派单）
	intersection := strings.TrimSpace(c.PostForm("intersection"))
	lightColor := c.PostForm("light_color")
	faultDesc := c.PostForm("fault_desc")
	isActiveFault := c.PostForm("is_active_fault") == "true"

	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择上传文件")
		return
	}

	// 举证/上传视频必须填写信号灯信息（仅凭视频难以定位路口/识别故障/派单）
	if mediaType == model.MediaEvidence || mediaType == "" {
		if intersection == "" {
			badRequest(c, "请填写路口名称（便于定位与派单）")
			return
		}
	}

	// 解析 hwID：仅接受纯数字，避免路径穿越（sanitize，防 ../ 注入文件名）
	hwIDUint, err := parseUintStrict(hwID)
	if err != nil {
		badRequest(c, "设备硬件ID不合法（须为数字）")
		return
	}

	// 校验扩展名
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".mp4": true, ".mov": true, ".webm": true, ".avi": true}
	if !allowed[ext] {
		badRequest(c, "不支持的文件类型（jpg/png/gif/mp4/mov/webm/avi）")
		return
	}
	// 校验大小（视频最大 200MB）
	if file.Size > 200*1024*1024 {
		badRequest(c, "文件过大（最大200MB）")
		return
	}

	// 打开文件内容做 MIME 嗅探（前 512 字节），防止恶意文件伪装成图片/视频上传
	src, err := file.Open()
	if err != nil {
		serverError(c, err)
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	_ = src.Close()
	if n > 0 {
		if !mimeAllowed(ext, http.DetectContentType(head[:n])) {
			badRequest(c, "文件内容与扩展名不符，已拒绝上传")
			return
		}
	}

	// 创建存储目录：{mediaDir}/{yyyyMM}
	dir := filepath.Join(MediaDir(), time.Now().Format("200601"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverError(c, err)
		return
	}

	// 生成唯一文件名（纯数字 hwID + 时间戳 + 扩展名，无用户可控字符）
	fname := fmt.Sprintf("%d_%d%s", hwIDUint, time.Now().UnixMilli(), ext)
	fpath := filepath.Join(dir, fname)
	if err := c.SaveUploadedFile(file, fpath); err != nil {
		serverError(c, err)
		return
	}

	// 相对 URL
	rel := filepath.ToSlash(filepath.Join(mediaURLPrefix(), time.Now().Format("200601"), fname))

	media := model.DeviceMedia{
		DeviceHwID:    hwIDUint,
		MediaType:     mediaType,
		Category:      categoryOf(ext),
		Title:         title,
		Source:        model.MediaSourceUpload,
		URL:           rel,
		Thumbnail:     thumbOf(ext, rel),
		Note:          note,
		UploadedBy:    c.GetString("username"),
		Intersection:  intersection,
		LightColor:    lightColor,
		FaultDesc:     faultDesc,
		IsActiveFault: isActiveFault,
	}
	if err := model.DB.Create(&media).Error; err != nil {
		_ = os.Remove(fpath)
		serverError(c, err)
		return
	}

	// 操作日志
	recordOperation(c, model.OpCreate, fmt.Sprintf("media/%d", media.ID), "上传设备媒体")

	ok(c, gin.H{"media": media, "message": "上传成功"})
}

// CreateRTSPMedia 登记路灯监控 RTSP / 云URL 流
// body: device_hw_id, media_type(monitoring/timelapse), title, url, compatible_url, thumbnail, duration
func CreateRTSPMedia(c *gin.Context) {
	var req struct {
		DeviceHwID    uint32 `json:"device_hw_id" binding:"required"`
		MediaType     string `json:"media_type" binding:"required"`
		Title         string `json:"title"`
		URL           string `json:"url" binding:"required"`
		CompatibleURL string `json:"compatible_url"`
		Thumbnail     string `json:"thumbnail"`
		Duration      int    `json:"duration"`
		Note          string `json:"note"`
		Intersection  string `json:"intersection"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id、media_type、url 必填）")
		return
	}

	// ---- 登记校验 ----
	// 1. media_type 白名单：登记仅限监控/时间视频；举证走上传接口
	switch req.MediaType {
	case model.MediaMonitoring, model.MediaTimelapse:
	default:
		badRequest(c, "media_type 不合法：仅支持 monitoring / timelapse（举证请使用上传）")
		return
	}

	// 2. url 合法协议（rtsp/rtsps/http/https）
	if !validStreamURL(req.URL) {
		badRequest(c, "视频地址不合法：需以 rtsp://、rtsps://、http:// 或 https:// 开头")
		return
	}

	// 3. compatible_url 若填写也需合法
	if req.CompatibleURL != "" && !validStreamURL(req.CompatibleURL) {
		badRequest(c, "兼容播放地址不合法：需为 http(s)://、rtsp(s):// 或 HLS/FLV 地址")
		return
	}

	// 4. 设备存在性
	var dev model.Device
	if err := model.DB.Where("hw_id = ?", req.DeviceHwID).First(&dev).Error; err != nil {
		badRequest(c, "关联设备不存在")
		return
	}

	// 5. RTSP 源且未填兼容地址 -> 仍登记，但返回提示（浏览器无法直接直播）
	isRTSP := strings.HasPrefix(strings.ToLower(req.URL), "rtsp://") || strings.HasPrefix(strings.ToLower(req.URL), "rtsps://")
	warning := ""
	if isRTSP && req.CompatibleURL == "" {
		warning = "RTSP 源无法在浏览器直接播放，建议补充兼容播放地址(HLS/FLV)以便监控大屏同屏直播"
	}

	// source 根据 url 判断：rtsp:// 为 rtsp，否则 url
	source := model.MediaSourceURL
	if isRTSP {
		source = model.MediaSourceRTSP
	}

	media := model.DeviceMedia{
		DeviceHwID:    req.DeviceHwID,
		MediaType:     req.MediaType,
		Category:      model.MediaVideo,
		Title:         req.Title,
		Source:        source,
		URL:           req.URL,
		CompatibleURL: req.CompatibleURL,
		Thumbnail:     req.Thumbnail,
		Duration:      req.Duration,
		Note:          req.Note,
		Intersection:  req.Intersection,
	}
	if err := model.DB.Create(&media).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("media/%d", media.ID), "登记监控/视频流")
	resp := gin.H{"media": media, "message": "登记成功"}
	if warning != "" {
		resp["warning"] = warning
	}
	ok(c, resp)
}

// DeleteDeviceMedia 删除设备媒体
func DeleteDeviceMedia(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "媒体ID无效")
		return
	}
	var media model.DeviceMedia
	if err := model.DB.First(&media, id).Error; err != nil {
		notFound(c, "媒体不存在")
		return
	}
	if err := model.DB.Delete(&media).Error; err != nil {
		serverError(c, err)
		return
	}
	// 本地文件可尝试删除
	if media.Source == model.MediaSourceUpload {
		_ = os.Remove(filepath.Join(".", strings.TrimPrefix(media.URL, mediaURLPrefix())))
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("media/%d", id), "删除设备媒体")
	ok(c, gin.H{"message": "删除成功"})
}

// categoryOf 根据扩展名判断图片/视频
func categoryOf(ext string) string {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif":
		return model.MediaPhoto
	default:
		return model.MediaVideo
	}
}

// thumbOf 图片用自身作封面，视频暂无封面则空
func thumbOf(ext, url string) string {
	if categoryOf(ext) == model.MediaPhoto {
		return url
	}
	return ""
}

// validStreamURL 校验登记的视频地址协议：rtsp/rtsps/http/https
func validStreamURL(s string) bool {
	u := strings.TrimSpace(s)
	low := strings.ToLower(u)
	if strings.HasPrefix(low, "rtsp://") || strings.HasPrefix(low, "rtsps://") ||
		strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
			return true
		}
	}
	return false
}

// parseUintStrict 严格解析非负整数：空字符串返回 0（无设备），非纯数字报错。
// 用于上传文件名前缀，杜绝路径穿越注入。
func parseUintStrict(s string) (uint32, error) {
	if s == "" {
		return 0, nil
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("非数字")
		}
	}
	var n uint64
	for _, ch := range s {
		n = n*10 + uint64(ch-'0')
		if n > math.MaxUint32 {
			return 0, fmt.Errorf("超出范围")
		}
	}
	return uint32(n), nil
}

// mimeAllowed 校验文件内容 MIME 类型是否与扩展名声明的类别一致。
// 仅接受图片(image/*)或视频(video/*)，防止伪装恶意文件上传。
func mimeAllowed(ext, detected string) bool {
	if detected == "" {
		return true
	}
	isImg := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"
	isVid := ext == ".mp4" || ext == ".mov" || ext == ".webm" || ext == ".avi"
	if isImg {
		return strings.HasPrefix(detected, "image/")
	}
	if isVid {
		// 视频容器较多，宽松放行常见的视频/二进制容器
		return strings.HasPrefix(detected, "video/") || detected == "application/octet-stream" ||
			detected == "application/mp4" || detected == "video/mp4"
	}
	return false
}

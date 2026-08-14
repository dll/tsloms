package handler

import (
	"fmt"
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
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

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
		Offset(int((page-1)*pageSize)).Limit(int(pageSize)).Find(&list)

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

	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择上传文件")
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

	// 创建存储目录：{mediaDir}/{yyyyMM}
	dir := filepath.Join(MediaDir(), time.Now().Format("200601"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverError(c, err)
		return
	}

	// 生成唯一文件名（时间戳 + 随机）
	fname := fmt.Sprintf("%s_%d%s", hwID, time.Now().UnixMilli(), ext)
	fpath := filepath.Join(dir, fname)
	if err := c.SaveUploadedFile(file, fpath); err != nil {
		serverError(c, err)
		return
	}

	// 相对 URL
	rel := filepath.ToSlash(filepath.Join(mediaURLPrefix(), time.Now().Format("200601"), fname))

	hwIDUint := uint32(0)
	if hwID != "" {
		_, _ = fmt.Sscanf(hwID, "%d", &hwIDUint)
	}

	media := model.DeviceMedia{
		DeviceHwID: hwIDUint,
		MediaType:  mediaType,
		Category:   categoryOf(ext),
		Title:      title,
		Source:     model.MediaSourceUpload,
		URL:        rel,
		Thumbnail:  thumbOf(ext, rel),
		Note:       note,
		UploadedBy: c.GetString("username"),
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id、media_type、url 必填）")
		return
	}
	// source 根据 url 判断：rtsp:// 为 rtsp，否则 url
	source := model.MediaSourceURL
	if strings.HasPrefix(strings.ToLower(req.URL), "rtsp://") || strings.HasPrefix(strings.ToLower(req.URL), "rtsps://") {
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
	}
	if err := model.DB.Create(&media).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("media/%d", media.ID), "登记监控/视频流")
	ok(c, gin.H{"media": media, "message": "登记成功"})
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

package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// FirmwareDir 固件文件存储目录
func FirmwareDir() string {
	dir := config.Get().MediaDir
	if dir == "" {
		dir = "./uploads/media"
	}
	return filepath.Join(dir, "firmware")
}

// firmwareURLPrefix 固件下载访问前缀
func firmwareURLPrefix() string {
	p := config.Get().MediaURLPrefix
	if p == "" {
		p = "/media"
	}
	return p + "/firmware"
}

// versionRegex 固件版本号匹配：v 开头 + 至少一段数字（v1.2.3 或 v1.2）
var versionRegex = regexp.MustCompile(`^[vV]?(\d+)(?:\.(\d+))?(?:\.(\d+))?$`)

// parseVersion 解析固件版本号，返回 (major, minor, build)；非法返回 error
func parseVersion(v string) (uint32, uint32, uint32, error) {
	m := versionRegex.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return 0, 0, 0, fmt.Errorf("版本号格式不合法，需形如 v1.2.3 / 1.2.3")
	}
	u := func(s string) uint32 {
		n, _ := strconv.ParseUint(s, 10, 32)
		return uint32(n)
	}
	return u(m[1]), u(m[2]), u(m[3]), nil
}

// ListFirmwares 固件包列表（分页，可按发布状态筛选）
func ListFirmwares(c *gin.Context) {
	page, pageSize := paginate(c)

	query := model.DB.Model(&model.FirmwarePackage{})
	if pub := c.Query("published"); pub == "true" || pub == "1" {
		query = query.Where("published = ?", true)
	}

	var total int64
	query.Count(&total)

	var list []model.FirmwarePackage
	query.Order("created_at DESC").
		Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	items := make([]gin.H, 0, len(list))
	for _, f := range list {
		items = append(items, firmwareView(f))
	}

	ok(c, gin.H{"list": items, "total": total, "page": page, "page_size": pageSize})
}

func firmwareView(f model.FirmwarePackage) gin.H {
	return gin.H{
		"id": f.ID, "version": f.Version, "major": f.Major, "minor": f.Minor, "build": f.Build,
		"sw_version": f.SwVersion, "file_name": f.FileName, "file_path": f.FilePath,
		"size": f.Size, "md5": f.MD5, "description": f.Description,
		"published": f.Published, "published_at": f.PublishedAt,
		"uploader": f.Uploader, "created_at": f.CreatedAt,
	}
}

// UploadFirmware 上传固件包（multipart: version, description, file）
// 生成唯一文件名，计算 MD5，写入固件目录
func UploadFirmware(c *gin.Context) {
	version := strings.TrimSpace(c.PostForm("version"))
	description := c.PostForm("description")
	if version == "" {
		badRequest(c, "请填写固件版本号")
		return
	}
	major, minor, build, err := parseVersion(version)
	if err != nil {
		badRequest(c, err.Error())
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择固件文件")
		return
	}
	// 固件通常是 .bin /.hex /.fw 等二进制文件
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !isFirmwareExt(ext) {
		badRequest(c, "固件文件类型不支持（bin/hex/fw/elf 等二进制）")
		return
	}
	// 固件最大 50MB
	if file.Size > 50*1024*1024 {
		badRequest(c, "固件文件过大（最大50MB）")
		return
	}

	// 版本号唯一校验
	var exists model.FirmwarePackage
	if err := model.DB.Where("version = ?", c.PostForm("version")).First(&exists).Error; err == nil {
		badRequest(c, "该固件版本已存在")
		return
	}

	dir := FirmwareDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverError(c, err)
		return
	}
	ts := time.Now().Format("20060102150405")
	// 文件名：version 清洗后 + 时间戳 + 原扩展名（防路径穿越）
	safeVer := regexp.MustCompile(`[^0-9A-Za-z._-]`).ReplaceAllString(version, "_")
	fname := fmt.Sprintf("fw_%s_%s%s", safeVer, ts, ext)
	fpath := filepath.Join(dir, fname)
	if err := c.SaveUploadedFile(file, fpath); err != nil {
		serverError(c, err)
		return
	}

	// 计算 MD5
	md5sum := fileMD5(fpath)
	rel := filepath.ToSlash(filepath.Join("firmware", fname))

	uploader := c.GetString("op_username")
	if uploader == "" {
		uploader = "system"
	}

	// 计算设备固件位域值：major(bit31:28) minor(bit27:24)，其余保持 0
	swVer := (major << 28) | (minor << 24)

	pkg := model.FirmwarePackage{
		Version:     c.PostForm("version"),
		Major:       major,
		Minor:       minor,
		Build:       build,
		SwVersion:   swVer,
		FileName:    file.Filename,
		FilePath:    rel,
		Size:        file.Size,
		MD5:         md5sum,
		Description: description,
		Published:   false,
		Uploader:    uploader,
	}
	if err := model.DB.Create(&pkg).Error; err != nil {
		_ = os.Remove(fpath)
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("firmware/%d", pkg.ID), "上传固件包 "+pkg.Version)
	ok(c, gin.H{"firmware": firmwareView(pkg), "url": firmwareURLPrefix() + "/" + fname})
}

// GetFirmware 固件包详情
func GetFirmware(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "固件ID无效")
		return
	}
	var f model.FirmwarePackage
	if err := model.DB.First(&f, id).Error; err != nil {
		notFound(c, "固件包不存在")
		return
	}
	ok(c, gin.H{"firmware": firmwareView(f)})
}

// UpdateFirmware 更新固件包描述/说明
func UpdateFirmware(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "固件ID无效")
		return
	}
	var req struct {
		Description string `json:"description"`
		Version     string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var f model.FirmwarePackage
	if err := model.DB.First(&f, id).Error; err != nil {
		notFound(c, "固件包不存在")
		return
	}
	updates := map[string]interface{}{}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Version != "" && req.Version != f.Version {
		major, minor, build, perr := parseVersion(req.Version)
		if perr != nil {
			badRequest(c, perr.Error())
			return
		}
		var dup model.FirmwarePackage
		if err := model.DB.Where("version = ?", req.Version).First(&dup).Error; err == nil {
			badRequest(c, "该固件版本已存在")
			return
		}
		updates["version"] = req.Version
		updates["major"] = major
		updates["minor"] = minor
		updates["build"] = build
		updates["sw_version"] = (major << 28) | (minor << 24)
	}
	if len(updates) > 0 {
		if err := model.DB.Model(&f).Updates(updates).Error; err != nil {
			serverError(c, err)
			return
		}
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("firmware/%d", id), "更新固件包 "+f.Version)
	ok(c, gin.H{"message": "更新成功"})
}

// PublishFirmware 发布/下线固件包（仅管理员/运维）
// body: published bool
func PublishFirmware(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "固件ID无效")
		return
	}
	var req struct {
		Published bool `json:"published"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var f model.FirmwarePackage
	if err := model.DB.First(&f, id).Error; err != nil {
		notFound(c, "固件包不存在")
		return
	}
	updates := map[string]interface{}{"published": req.Published}
	var t *time.Time
	if req.Published {
		now := time.Now()
		t = &now
	}
	updates["published_at"] = t
	if err := model.DB.Model(&f).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}
	action := "下线固件"
	if req.Published {
		action = "发布固件"
	}
	recordOperation(c, model.OpUpdate, fmt.Sprintf("firmware/%d", id), action+" "+f.Version)
	ok(c, gin.H{"message": action + "成功"})
}

// DeleteFirmware 删除固件包（仅管理员）
func DeleteFirmware(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "固件ID无效")
		return
	}
	var f model.FirmwarePackage
	if err := model.DB.First(&f, id).Error; err != nil {
		notFound(c, "固件包不存在")
		return
	}
	if err := model.DB.Delete(&f).Error; err != nil {
		serverError(c, err)
		return
	}
	// 尝试删除本地文件
	if f.FilePath != "" {
		_ = os.Remove(filepath.Join("uploads", "media", strings.TrimPrefix(f.FilePath, "firmware/")))
		_ = os.Remove(filepath.Join(FirmwareDir(), filepath.Base(f.FilePath)))
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("firmware/%d", id), "删除固件包 "+f.Version)
	ok(c, gin.H{"message": "删除成功"})
}

// ListFirmwareUpgrades 设备固件升级记录（分页，可按设备/状态筛选）
func ListFirmwareUpgrades(c *gin.Context) {
	page, pageSize := paginate(c)

	query := model.DB.Model(&model.FirmwareUpgradeRecord{})
	if hwID := c.Query("device_hw_id"); hwID != "" {
		query = query.Where("device_hw_id = ?", hwID)
	}
	if st := c.Query("status"); st != "" {
		query = query.Where("status = ?", st)
	}
	var total int64
	query.Count(&total)

	var list []model.FirmwareUpgradeRecord
	query.Order("created_at DESC").
		Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)

	items := make([]gin.H, 0, len(list))
	for _, r := range list {
		items = append(items, gin.H{
			"id": r.ID, "firmware_id": r.FirmwareID, "device_hw_id": r.DeviceHwID,
			"target_version": r.TargetVer, "status": r.Status, "error_msg": r.ErrorMsg,
			"started_at": r.StartedAt, "finished_at": r.FinishedAt, "created_at": r.CreatedAt,
		})
	}
	ok(c, gin.H{"list": items, "total": total, "page": page, "page_size": pageSize})
}

// CreateFirmwareUpgrade 发起指定设备的固件升级任务
// body: device_hw_id, firmware_id
func CreateFirmwareUpgrade(c *gin.Context) {
	var req struct {
		DeviceHwID string `json:"device_hw_id" binding:"required"`
		FirmwareID uint   `json:"firmware_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（device_hw_id、firmware_id 必填）")
		return
	}
	// 设备存在
	var dev model.Device
	if err := model.DB.Where("hw_id = ?", req.DeviceHwID).First(&dev).Error; err != nil {
		badRequest(c, "关联设备不存在")
		return
	}
	// 固件存在且已发布
	var fw model.FirmwarePackage
	if err := model.DB.First(&fw, req.FirmwareID).Error; err != nil {
		notFound(c, "固件包不存在")
		return
	}
	if !fw.Published {
		badRequest(c, "固件未发布，无法发起升级")
		return
	}

	// 避免同一设备重复待升级/升级中任务
	var dup model.FirmwareUpgradeRecord
	if err := model.DB.Where("device_hw_id = ? AND status IN ?", req.DeviceHwID,
		[]string{model.FirmwareUpgradePending, model.FirmwareUpgradeUpgrading}).First(&dup).Error; err == nil {
		badRequest(c, "该设备已有未完成的升级任务")
		return
	}

	now := time.Now()
	rec := model.FirmwareUpgradeRecord{
		FirmwareID: req.FirmwareID,
		DeviceHwID: req.DeviceHwID,
		TargetVer:  fw.Version,
		Status:     model.FirmwareUpgradePending,
		StartedAt:  &now,
	}
	if err := model.DB.Create(&rec).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, fmt.Sprintf("firmware-upgrade/%d", rec.ID), fmt.Sprintf("发起设备 %s 固件升级到 %s", req.DeviceHwID, fw.Version))
	ok(c, gin.H{"record": rec.ID, "message": "升级任务已创建"})
}

// DeleteFirmwareUpgrade 删除升级记录（仅管理员）
func DeleteFirmwareUpgrade(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "记录ID无效")
		return
	}
	var r model.FirmwareUpgradeRecord
	if err := model.DB.First(&r, id).Error; err != nil {
		notFound(c, "升级记录不存在")
		return
	}
	if err := model.DB.Delete(&r).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, fmt.Sprintf("firmware-upgrade/%d", id), "删除升级记录")
	ok(c, gin.H{"message": "删除成功"})
}

// ---------- 文件工具 ----------

// isFirmwareExt 固件文件扩展名白名单
func isFirmwareExt(ext string) bool {
	switch ext {
	case ".bin", ".hex", ".fw", ".elf", ".img", ".dat":
		return true
	}
	return false
}

// fileMD5 计算文件 MD5 校验值
func fileMD5(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

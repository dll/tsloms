package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 用户头像/工作照上传 + 个人资料（人事字段）自助维护
// ----------------------------------------------------------------------------
// · POST /user/avatar  上传“工作照”，作为头像，展示在右上角（multipart: file）
// · PUT  /user/profile 用户自助维护人事字段（工号/性别/身份证/住址/文化程度/工程等级等）
// ============================================================================

const avatarMaxBytes = 5 << 20 // 5MB

// validAvatarExt 头像允许的图片扩展名
func validAvatarExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// UploadMyAvatar POST /user/avatar （工作照/头像）
// multipart 字段 file=图片；落盘到 {mediaDir}/{yyyyMM}/avatar/，更新当前用户 avatar 字段。
func UploadMyAvatar(c *gin.Context) {
	userID := c.GetUint("user_id")
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择要上传的工作照")
		return
	}
	if file.Size > avatarMaxBytes {
		badRequest(c, "图片过大（最大 5MB）")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !validAvatarExt(ext) {
		badRequest(c, "仅支持 jpg/png/gif/webp/bmp 图片")
		return
	}

	// 随机文件名，避免覆盖
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	fname := fmt.Sprintf("avatar_%s_%d%s", hex.EncodeToString(buf), time.Now().Unix(), ext)

	// 存储目录 {mediaDir}/{yyyyMM}/avatar
	sub := time.Now().Format("200601")
	dir := filepath.Join(MediaDir(), sub, "avatar")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverError(c, err)
		return
	}
	fpath := filepath.Join(dir, fname)
	if err := c.SaveUploadedFile(file, fpath); err != nil {
		serverError(c, err)
		return
	}

	// 对外 URL
	url := filepath.ToSlash(filepath.Join(mediaURLPrefix(), sub, "avatar", fname))

	// 更新当前用户头像
	if err := model.DB.Model(&user).Update("avatar", url).Error; err != nil {
		serverError(c, err)
		return
	}

	recordOperation(c, model.OpUpdate, fmt.Sprintf("user/%d", user.ID), "上传工作照/头像")
	ok(c, gin.H{"avatar": url, "message": "头像已更新"})
}

// UpdateMyProfile PUT /user/profile
// 用户自助维护人事字段；仅能改自己的资料（不改角色/账号/密码）。
func UpdateMyProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	var req struct {
		RealName      *string `json:"real_name"`
		Phone         *string `json:"phone"`
		Email         *string `json:"email"`
		WorkNo        *string `json:"work_no"`
		Gender        *string `json:"gender"`
		IDCard        *string `json:"id_card"`
		Address       *string `json:"address"`
		Education     *string `json:"education"`
		EngineerLevel *string `json:"engineer_level"`
		Avatar        *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.RealName != nil {
		updates["real_name"] = strings.TrimSpace(*req.RealName)
	}
	if req.Phone != nil {
		p := strings.TrimSpace(*req.Phone)
		if p != "" && !validPhoneFormat(p) {
			badRequest(c, "手机号格式不正确（需 11 位大陆手机号）")
			return
		}
		updates["phone"] = p
		if p != "" {
			updates["phone_login"] = p
			updates["phone_verified"] = true
		}
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.WorkNo != nil {
		updates["work_no"] = strings.TrimSpace(*req.WorkNo)
	}
	if req.Gender != nil {
		updates["gender"] = strings.TrimSpace(*req.Gender)
	}
	if req.IDCard != nil {
		updates["id_card"] = strings.TrimSpace(*req.IDCard)
	}
	if req.Address != nil {
		updates["address"] = strings.TrimSpace(*req.Address)
	}
	if req.Education != nil {
		updates["education"] = strings.TrimSpace(*req.Education)
	}
	if req.EngineerLevel != nil {
		updates["engineer_level"] = strings.TrimSpace(*req.EngineerLevel)
	}
	if req.Avatar != nil {
		updates["avatar"] = strings.TrimSpace(*req.Avatar)
	}

	if len(updates) == 0 {
		badRequest(c, "无可更新字段")
		return
	}
	if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}

	model.DB.First(&user, userID)
	recordOperation(c, model.OpUpdate, fmt.Sprintf("user/%d", user.ID), "更新个人资料")
	ok(c, gin.H{"user": userPayload(&user), "message": "资料已更新"})
}

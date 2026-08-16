package model

import (
	"fmt"
	"math/rand"
	"strings"

	"gorm.io/gorm"
)

// SeedRBAC 初始化权限字典与内置角色（admin/operator/viewer）及其默认权限，幂等
func SeedRBAC(db *gorm.DB) error {
	// 1) 权限字典：按 code 幂等插入
	permIDByCode := map[string]uint{}
	for _, p := range AllPermissions {
		var existing Permission
		err := db.Where("code = ?", p.Code).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if e := db.Create(&p).Error; e != nil {
				return e
			}
			permIDByCode[p.Code] = p.ID
		} else if err != nil {
			return err
		} else {
			permIDByCode[p.Code] = existing.ID
			// 同步名称/模块/排序，保持与新版本一致
			db.Model(&existing).Updates(map[string]interface{}{
				"name": p.Name, "module": p.Module, "sort": p.Sort,
			})
		}
	}

	// 2) 内置角色 + 默认权限
	builtins := []struct {
		code string
		name string
	}{
		{BuiltinRoleAdmin, "管理员"},
		{BuiltinRoleOperator, "运维人员"},
		{BuiltinRoleViewer, "查看人员"},
	}
	for _, b := range builtins {
		var role Role
		err := db.Where("code = ?", b.code).First(&role).Error
		if err == gorm.ErrRecordNotFound {
			role = Role{Code: b.code, Name: b.name, Builtin: true}
			if e := db.Create(&role).Error; e != nil {
				return e
			}
		} else if err != nil {
			return err
		}

		// 同步该角色默认权限（先删后插，保证与代码一致）
		db.Where("role_id = ?", role.ID).Delete(&RolePermission{})
		for _, code := range BuiltinRolePerms[b.code] {
			pid, ok := permIDByCode[code]
			if !ok {
				continue
			}
			if e := db.Create(&RolePermission{
				RoleID: role.ID, PermissionID: pid, RoleCode: role.Code,
			}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

// SeedAdmin 初始化默认管理员账户（仅在 users 表为空时创建）
// initPwd: 首次启动管理员密码；为空时自动生成 16 位随机强密码并通过返回值告知调用方（避免固定弱默认密码）
// 返回值: error；生成的随机密码通过返回值字符串返回（调用方负责记录到安全渠道）
func SeedAdmin(initPwd string) (string, error) {
	if DB == nil {
		return "", fmt.Errorf("数据库未初始化")
	}

	var count int64
	DB.Model(&User{}).Count(&count)
	if count > 0 {
		return "", nil
	}

	pwd := initPwd
	generated := false
	if pwd == "" {
		pwd = randomStrongPassword(16)
		generated = true
	}

	admin := User{
		Username:     "admin",
		PasswordHash: HashPassword(pwd),
		Role:         RoleAdmin,
	}
	if err := DB.Create(&admin).Error; err != nil {
		return "", err
	}
	if generated {
		return pwd, nil
	}
	return "", nil
}

// randomStrongPassword 生成包含大小写字母与数字的随机强密码（保证每类至少一个）
func randomStrongPassword(n int) string {
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	const all = lower + upper + digits
	if n < 3 {
		n = 3
	}
	sb := make([]byte, n)
	// 重试直到三字符类（大写/小写/数字）至少各出现一次，杜绝偶发缺类
	for {
		for i := range sb {
			sb[i] = all[rand.Intn(len(all))]
		}
		if hasClasses(sb, lower, upper, digits) {
			break
		}
	}
	return string(sb)
}

// hasClasses 判断字节数组是否覆盖给定字符类（每类至少出现一次）
func hasClasses(p []byte, classes ...string) bool {
	for _, cset := range classes {
		ok := false
		for _, b := range p {
			if strings.ContainsRune(cset, rune(b)) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

package model

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestSeedSuperAdmin_AccountCreated 超级管理员 419116 被幂等种子创建，bcrypt 加密入库，角色 super_admin
func TestSeedSuperAdmin_AccountCreated(t *testing.T) {
	db := InitTestDB()
	if _, err := SeedSuperAdmin(db, ""); err != nil {
		t.Fatalf("SeedSuperAdmin 失败: %v", err)
	}
	var u User
	if err := db.Where("username = ?", SuperAdminUsername).First(&u).Error; err != nil {
		t.Fatalf("超级管理员未创建: %v", err)
	}
	if u.Role != RoleSuperAdmin {
		t.Errorf("角色=%s, 期望 super_admin", u.Role)
	}
	if u.PasswordHash == "" || strings.HasPrefix(u.PasswordHash, "Osgis") {
		t.Error("密码应为 bcrypt 密文，而非明文/硬编码")
	}
	// 幂等：再次调用不重复创建
	if _, err := SeedSuperAdmin(db, ""); err != nil {
		t.Fatalf("重复 SeedSuperAdmin 失败: %v", err)
	}
	var cnt int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&cnt)
	if cnt != 1 {
		t.Errorf("超级管理员应只 1 个, 实际 %d", cnt)
	}
}

// TestSeedSuperAdmin_PasswordValidates 显式传入初始密码可校验通过（默认空则生成随机密码，不再硬编码明文）
func TestSeedSuperAdmin_PasswordValidates(t *testing.T) {
	db := InitTestDB()
	// AutoMigrate 已预建超管（随机密码），先删掉再以已知密码重建验证
	const known = "StrongPw2026!"
	db.Where("username = ?", SuperAdminUsername).Delete(&User{})
	if _, err := SeedSuperAdmin(db, known); err != nil {
		t.Fatal(err)
	}
	var u User
	db.Where("username = ?", SuperAdminUsername).First(&u)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(known)); err != nil {
		t.Errorf("显式密码校验失败: %v", err)
	}
}

// TestSeedSuperAdmin_GeneratedRandom 未提供密码时生成随机强密码并返回（不硬编码）
func TestSeedSuperAdmin_GeneratedRandom(t *testing.T) {
	db := InitTestDB()
	db.Where("username = ?", SuperAdminUsername).Delete(&User{})
	pwd, err := SeedSuperAdmin(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if pwd == "" {
		t.Fatal("未配置密码时应生成随机密码并返回")
	}
	if len(pwd) < 16 {
		t.Errorf("随机密码过短: %d", len(pwd))
	}
	var u User
	db.Where("username = ?", SuperAdminUsername).First(&u)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pwd)); err != nil {
		t.Errorf("随机密码校验失败: %v", err)
	}
}

// TestRolePerms_SuperAdminHasModuleManage super_admin 含 module:manage；admin 不含。
func TestRolePerms_SuperAdminHasModuleManage(t *testing.T) {
	saPerms := BuiltinRolePerms[BuiltinRoleSuperAdmin]
	adminPerms := BuiltinRolePerms[BuiltinRoleAdmin]
	has := func(list []string, code string) bool {
		for _, c := range list {
			if c == code {
				return true
			}
		}
		return false
	}
	if !has(saPerms, "module:manage") {
		t.Error("超级管理员应含 module:manage")
	}
	if has(adminPerms, "module:manage") {
		t.Error("系统管理员(admin)不应含 module:manage（已降级）")
	}
}

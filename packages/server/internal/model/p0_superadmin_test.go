package model

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestSeedSuperAdmin_AccountCreated 超级管理员 419116 被幂等种子创建，bcrypt 加密入库，角色 super_admin
func TestSeedSuperAdmin_AccountCreated(t *testing.T) {
	db := InitTestDB()
	if err := SeedSuperAdmin(db); err != nil {
		t.Fatalf("SeedSuperAdmin 失败: %v", err)
	}
	var u User
	if err := db.Where("username = ?", SuperAdminUsername).First(&u).Error; err != nil {
		t.Fatalf("超级管理员未创建: %v", err)
	}
	if u.Role != RoleSuperAdmin {
		t.Errorf("角色=%s, 期望 super_admin", u.Role)
	}
	if u.PasswordHash == "" || u.PasswordHash == SuperAdminPassword {
		t.Error("密码应为 bcrypt 密文，而非明文")
	}
	// 幂等：再次调用不重复创建
	if err := SeedSuperAdmin(db); err != nil {
		t.Fatalf("重复 SeedSuperAdmin 失败: %v", err)
	}
	var cnt int64
	db.Model(&User{}).Where("username = ?", SuperAdminUsername).Count(&cnt)
	if cnt != 1 {
		t.Errorf("超级管理员应只 1 个, 实际 %d", cnt)
	}
}

// TestSeedSuperAdmin_PasswordValidates 419116/Osgis!!! 密码可校验通过
func TestSeedSuperAdmin_PasswordValidates(t *testing.T) {
	db := InitTestDB()
	if err := SeedSuperAdmin(db); err != nil {
		t.Fatal(err)
	}
	var u User
	db.Where("username = ?", SuperAdminUsername).First(&u)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(SuperAdminPassword)); err != nil {
		t.Errorf("密码校验失败: %v", err)
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

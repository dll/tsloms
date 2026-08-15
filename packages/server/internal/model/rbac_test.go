package model

import (
	"testing"
)

// TestSeedRBAC 验证权限字典与内置角色种子正确写入
func TestSeedRBAC(t *testing.T) {
	db := InitTestDB() // 会调用 AutoMigrate 与 SeedRBAC

	// 权限字典数量
	var permCount int64
	db.Model(&Permission{}).Count(&permCount)
	if permCount != int64(len(AllPermissions)) {
		t.Fatalf("权限字典数量 = %d, 期望 %d", permCount, len(AllPermissions))
	}

	// 内置角色
	var roles []Role
	db.Find(&roles)
	if len(roles) != 3 {
		t.Fatalf("内置角色数量 = %d, 期望 3", len(roles))
	}
}

// TestEffectivePermissions 验证各类用户的权限计算
func TestEffectivePermissions(t *testing.T) {
	InitTestDB()

	// 创建三种用户
	var admin, operator, viewer User
	admin = User{Username: "rbac_admin", PasswordHash: "x", Role: RoleAdmin}
	operator = User{Username: "rbac_operator", PasswordHash: "x", Role: RoleOperator}
	viewer = User{Username: "rbac_viewer", PasswordHash: "x", Role: RoleViewer}
	DB.Create(&admin)
	DB.Create(&operator)
	DB.Create(&viewer)

	// admin 应拥有全部权限
	ap, err := EffectivePermissions(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range AllPermissions {
		if !ap[p.Code] {
			t.Fatalf("admin 缺少权限 %s", p.Code)
		}
	}

	// operator 应有业务写权限，无用户/角色管理，无 AI 配置
	op, _ := EffectivePermissions(operator.ID)
	if !op["device:create"] {
		t.Fatal("operator 应具备 device:create")
	}
	if op["user:manage"] {
		t.Fatal("operator 不应具备 user:manage")
	}
	if op["role:manage"] {
		t.Fatal("operator 不应具备 role:manage")
	}
	if op["ai:config"] {
		t.Fatal("operator 不应具备 ai:config")
	}
	if !op["ai:ops"] {
		t.Fatal("operator 应具备 ai:ops")
	}

	// viewer 应无写权限
	vp, _ := EffectivePermissions(viewer.ID)
	for _, code := range allPermCodes() {
		if vp[code] {
			t.Fatalf("viewer 不应具备写权限 %s", code)
		}
	}
}

// TestUserPermissionOverride 验证用户级权限覆写
func TestUserPermissionOverride(t *testing.T) {
	InitTestDB()

	var op User
	op = User{Username: "rbac_override", PasswordHash: "x", Role: RoleOperator}
	DB.Create(&op)

	// 显式授予：仅 device:create + workorder:create（覆盖角色默认）
	DB.Create(&UserPermission{UserID: op.ID, Permission: "device:create", Granted: true})
	DB.Create(&UserPermission{UserID: op.ID, Permission: "workorder:create", Granted: true})

	perms, err := EffectivePermissions(op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !perms["device:create"] {
		t.Fatal("应含 device:create")
	}
	if !perms["workorder:create"] {
		t.Fatal("应含 workorder:create")
	}
	if perms["fault:update"] {
		t.Fatal("显式授权应覆盖角色默认, 不应含 fault:update")
	}
}

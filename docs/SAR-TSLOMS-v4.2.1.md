# TSLOMS 结项审核报告（SAR）v4.2.1 — RBAC 权限隔离改造

> 版本：v4.2.1 ｜ 日期：2026-08-15 ｜ 基线：commit `a46c4d8`
> 承接 v4.2（库存进销存/费用闭环）。本版落实「权限隔离」：由固定三角色升级为**可配置 RBAC**（组织结构驱动的角色 + 功能权限）。

---

## 一、需求

- **原**：后端仅 3 固定角色（admin/operator/viewer），路由 `RequireAdmin()/RequireOperator()` 按角色名硬编码；用户只能单选固定角色；departments 仅展示不参与判定。
- **目标**：用户管理可从组织结构出发分配「角色 + 功能权限」，支持自定义角色与细粒度控制。

## 二、交付内容

### 后端（packages/server）
- `internal/model/rbac.go`：`Permission` / `Role` / `RolePermission` / `UserPermission` 模型；**28 个功能权限点（14 模块）**字典；内置 admin/operator/viewer 默认权限映射；`EffectivePermissions()`/`EffectivePermissionCodes()` 权限计算（角色默认 + 用户覆写）。
- `internal/model/db.go`：AutoMigrate 注册 4 表 + 幂等 `SeedRBAC()`。
- `internal/middleware/rbac.go`：`RequirePerm(perm)` / `RequirePerms(...)` 中间件。
- `internal/handler/rbac.go`：`ListPermissions` / `ListRoles` / `CreateRole` / `UpdateRole` / `DeleteRole` / `GetUserPermissions` / `SetUserPermissions`；`MyPermissions`。
- `cmd/server/main.go`：约 50 处 `RequireOperator/RequireAdmin` 升级为细粒度 `RequirePerm(...)`；新增 `/rbac/*`（守卫 `role:manage`）与 `/my/permissions`；媒体上传等新增 `media:upload` 守卫。
- `internal/model/rbac_test.go`：种子与权限计算单测。

### 前端（packages/admin）
- `src/store/auth.ts`：新增 `permissions` ref + `hasPerm()`；登录/加载时拉 `/my/permissions`。
- `src/views/layout/index.vue`：菜单（AI 子菜单、系统设置）按权限联动显隐。
- `src/views/settings/index.vue`：新增**角色管理**页签（自定义角色 CRUD + 权限勾选）；用户管理支持指定角色 + **功能权限自定义覆写**（含角色切换预填）。
- `src/api/rbac.ts`：RBAC 接口封装。

## 三、关键设计

- **权限计算**：有效权限 = 角色默认权限 ∪ 用户显式授权 − 用户显式拒绝；无覆写则走角色默认，兼容现有账号。
- **路由粒度**：设备 create/update/delete、故障 dispatch、工单 status/assign、用户/组织/角色管理、AI 配置/操作等按功能点隔离。
- **内置三角色种子**：admin=全 28、operator=业务写（无用户/组织/角色管理、无 AI 配置）、viewer=仅读；允许创建自定义角色。

## 四、测试结果

- `go build` / `go vet` / `gofmt` ✅；`go test ./...` 全绿（含 rbac 模型单测）
- `vue-tsc` / `eslint` / `npm run build` ✅
- **生产 E2E 24 项全 PASS**：admin 全 28 权限、operator 含 `device:create` 不含 `user:manage`、viewer 写操作与角色管理被拒(403)、权限集为空、自定义角色 CUD、测试数据已清理。
- 部署：后端二进制 + 前端 dist 原子替换（备份 `server.pre-rbac-*`/`dist.pre-rbac-*`）；网关/后台 200。

---

*本报告基于代码静态审核 + 单元测试 + 生产 E2E 实测。*

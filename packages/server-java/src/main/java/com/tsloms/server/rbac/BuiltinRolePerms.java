// 内置角色默认权限集合：对齐 Go 版 rbac.go BuiltinRolePerms。
package com.tsloms.server.rbac;

import static com.tsloms.server.rbac.PermissionCatalog.MODULE_MANAGE;
import static com.tsloms.server.rbac.PermissionCatalog.allCodes;
import static com.tsloms.server.rbac.PermissionCatalog.without;

import com.tsloms.server.model.UserRoles;
import java.util.List;
import java.util.Map;

public final class BuiltinRolePerms {

    /** 角色 code → 默认权限编码集合（不可变）。 */
    public static final Map<String, List<String>> MAP = Map.of(
            // 超级管理员：全部权限（含模块设置）
            UserRoles.SUPER_ADMIN, allCodes(),
            // 系统管理员：全部权限，但【不含模块设置】（只能维护系统运行）
            UserRoles.ADMIN, without(allCodes(), MODULE_MANAGE),
            // 运维人员：业务写操作（不含用户/组织/角色管理、AI 配置、核心删除、模块设置）
            UserRoles.OPERATOR,
            List.of(
                    "device:create", "device:update",
                    "intersection:update",
                    "fault:update", "fault:dispatch", "fault:review",
                    "workorder:create", "workorder:update", "workorder:assign",
                    "warning:manage", "warning:rule", "crossing:manage",
                    "patrol:manage", "patrol:run", "patrol:selfcheck",
                    "media:upload",
                    "firmware:manage",
                    "inventory:manage",
                    "supplier:manage",
                    "purchase:manage",
                    "expense:manage",
                    "ai:ops"),
            // 查看人员：仅只读（无写权限）
            UserRoles.VIEWER, List.of());

    private BuiltinRolePerms() {
    }
}

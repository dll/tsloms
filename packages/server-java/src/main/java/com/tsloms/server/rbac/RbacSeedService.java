// RBAC 种子：启动时幂等写入权限点字典与内置角色（对齐 Go 版 seed.go 核心行为）。
package com.tsloms.server.rbac;

import com.tsloms.server.model.Permission;
import com.tsloms.server.model.Role;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.repository.PermissionRepository;
import com.tsloms.server.repository.RoleRepository;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class RbacSeedService implements ApplicationRunner {

    private static final Logger log = LoggerFactory.getLogger(RbacSeedService.class);

    /** 内置角色默认名称/描述（对齐 Go 版 seed）。 */
    private static final Map<String, String[]> BUILTIN_ROLES = Map.of(
            UserRoles.SUPER_ADMIN, new String[]{"超级管理员", "全部权限，含模块启用设置"},
            UserRoles.ADMIN, new String[]{"系统管理员", "维护系统运行，不含模块设置"},
            UserRoles.OPERATOR, new String[]{"运维人员", "信号灯维护者"},
            UserRoles.VIEWER, new String[]{"查看人员", "只读"});

    private final PermissionRepository permissions;
    private final RoleRepository roles;
    private final com.tsloms.server.repository.RolePermissionRepository rolePerms;

    public RbacSeedService(PermissionRepository permissions, RoleRepository roles,
                           com.tsloms.server.repository.RolePermissionRepository rolePerms) {
        this.permissions = permissions;
        this.roles = roles;
        this.rolePerms = rolePerms;
    }

    @Override
    @Transactional
    public void run(ApplicationArguments args) {
        // 权限点：只增不删（code 幂等）
        for (PermissionCatalog.PermDef def : PermissionCatalog.ALL) {
            if (!permissions.existsByCode(def.code())) {
                Permission p = new Permission();
                p.code = def.code();
                p.name = def.name();
                p.module = def.module();
                p.sort = def.sort();
                permissions.save(p);
            }
        }
        // 内置角色：缺失则建
        BUILTIN_ROLES.forEach((code, meta) -> {
            if (roles.findByCode(code).isEmpty()) {
                Role r = new Role();
                r.code = code;
                r.name = meta[0];
                r.description = meta[1];
                r.builtin = true;
                roles.save(r);
            }
        });
        // 内置角色的默认权限关联（对齐 Go 版 seed：按 BuiltinRolePerms 幂等补齐）
        Map<String, java.util.List<String>> builtinPerms = BuiltinRolePerms.MAP;
        for (var entry : builtinPerms.entrySet()) {
            Role role = roles.findByCode(entry.getKey()).orElse(null);
            if (role == null) {
                continue;
            }
            var existing = new java.util.HashSet<Long>();
            rolePerms.findByRoleId(role.id).forEach(rp -> existing.add(rp.permissionId));
            for (String code : entry.getValue()) {
                Permission p = permissions.findByCode(code).orElse(null);
                if (p == null || existing.contains(p.id)) {
                    continue;
                }
                com.tsloms.server.model.RolePermission rp =
                        new com.tsloms.server.model.RolePermission();
                rp.roleId = role.id;
                rp.permissionId = p.id;
                rp.roleCode = code(entry.getKey());
                rolePerms.save(rp);
            }
        }
        log.info("[TSLOMS] RBAC 种子就绪：{} 个权限点 / {} 个内置角色",
                permissions.count(), roles.count());
    }

    private static String code(String roleCode) {
        return roleCode;
    }
}

// 有效权限计算服务：规则逐条对齐 Go 版 rbac.go EffectivePermissions。
package com.tsloms.server.rbac;

import static com.tsloms.server.rbac.BuiltinRolePerms.MAP;
import static com.tsloms.server.rbac.PermissionCatalog.MODULE_MANAGE;
import static com.tsloms.server.rbac.PermissionCatalog.allCodes;
import static com.tsloms.server.rbac.PermissionCatalog.without;

import com.tsloms.server.model.UserPermission;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.repository.PermissionRepository;
import com.tsloms.server.repository.RolePermissionRepository;
import com.tsloms.server.repository.RoleRepository;
import com.tsloms.server.repository.UserPermissionRepository;
import com.tsloms.server.repository.UserRepository;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class RbacService {

    private final UserRepository users;
    private final RoleRepository roles;
    private final RolePermissionRepository rolePerms;
    private final PermissionRepository permissions;
    private final UserPermissionRepository userPerms;

    public RbacService(UserRepository users, RoleRepository roles,
                       RolePermissionRepository rolePerms, PermissionRepository permissions,
                       UserPermissionRepository userPerms) {
        this.users = users;
        this.roles = roles;
        this.rolePerms = rolePerms;
        this.permissions = permissions;
        this.userPerms = userPerms;
    }

    /**
     * 计算用户的有效权限集合。规则（对齐 Go 版）：
     *
     * <ol>
     *   <li>内置角色直接取默认映射（super_admin 全量 / admin 除模块设置）；
     *   <li>其余角色：查 roles 表 → role_permissions → 权限码；角色未入库时按内置映射兜底；
     *   <li>user_permissions 有任一 granted=true 记录 → 全量替换为授权集合；
     *       否则仅剔除 granted=false 的显式拒绝项；
     *   <li>硬约束：module:manage 仅 super_admin 可持有，其余一律剔除（防越权绕过）。
     * </ol>
     */
    @Transactional(readOnly = true)
    public Set<String> effectivePermissions(Long userId) {
        var user = users.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("用户不存在: " + userId));

        Set<String> set = new HashSet<>();
        switch (user.role) {
            case UserRoles.SUPER_ADMIN -> set.addAll(allCodes());
            case UserRoles.ADMIN -> set.addAll(without(allCodes(), MODULE_MANAGE));
            default -> {
                roles.findByCode(user.role).ifPresentOrElse(role ->
                                rolePerms.findByRoleId(role.id).forEach(rp ->
                                        permissions.findById(rp.permissionId)
                                                .ifPresent(p -> set.add(p.code))),
                        () -> {
                            // 角色未入库（异常兜底）时按内置映射兜底
                            List<String> fallback = MAP.get(user.role);
                            if (fallback != null && !fallback.isEmpty()) {
                                set.addAll(fallback);
                            }
                        });
            }
        }

        // 用户级覆写
        List<UserPermission> ups = userPerms.findByUserId(userId);
        boolean grantedAny = ups.stream().anyMatch(up -> up.granted);
        if (grantedAny) {
            // 有显式授权记录：清空角色默认，以授权项为准
            set.clear();
            ups.forEach(up -> {
                if (up.granted) {
                    set.add(up.permission);
                }
            });
        } else {
            // 无显式授权：仅剔除显式拒绝项
            ups.forEach(up -> {
                if (!up.granted) {
                    set.remove(up.permission);
                }
            });
        }

        // 硬约束：模块设置仅超级管理员可拥有
        ensureModuleManageRestricted(user.role, set);
        return set;
    }

    /** 有效权限编码列表（排序稳定）。 */
    @Transactional(readOnly = true)
    public List<String> effectivePermissionCodes(Long userId) {
        return effectivePermissions(userId).stream().sorted().toList();
    }

    private void ensureModuleManageRestricted(String role, Set<String> set) {
        if (!UserRoles.SUPER_ADMIN.equals(role)) {
            set.remove(MODULE_MANAGE);
        }
    }
}

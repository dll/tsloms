// RbacService 有效权限计算测试：语义逐条对齐 Go 版 rbac_test.go。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.model.Permission;
import com.tsloms.server.model.Role;
import com.tsloms.server.model.RolePermission;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserPermission;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.rbac.PermissionCatalog;
import com.tsloms.server.rbac.RbacService;
import com.tsloms.server.repository.PermissionRepository;
import com.tsloms.server.repository.RolePermissionRepository;
import com.tsloms.server.repository.RoleRepository;
import com.tsloms.server.repository.UserPermissionRepository;
import com.tsloms.server.repository.UserRepository;
import java.util.Set;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.context.annotation.Import;

@DataJpaTest
@Import(RbacService.class)
class RbacServiceTest {

    @Autowired
    RbacService rbac;
    @Autowired UserRepository users;
    @Autowired RoleRepository roles;
    @Autowired PermissionRepository perms;
    @Autowired RolePermissionRepository rolePerms;
    @Autowired UserPermissionRepository userPerms;

    private Long newUser(String role) {
        User u = new User();
        u.username = "u_" + role + "_" + System.nanoTime();
        u.passwordHash = "x";
        u.role = role;
        users.saveAndFlush(u);
        return u.id;
    }

    private void seedRole(String code, String... permCodes) {
        Role r = new Role();
        r.code = code;
        r.name = code;
        roles.saveAndFlush(r);
        for (String pc : permCodes) {
            Permission p = new Permission();
            p.code = pc;
            p.name = pc;
            p.module = pc.split(":")[0];
            perms.saveAndFlush(p);
            RolePermission rp = new RolePermission();
            rp.roleId = r.id;
            rp.permissionId = p.id;
            rp.roleCode = code;
            rolePerms.saveAndFlush(rp);
        }
    }

    @Test
    void 超级管理员_全部权限含模块设置() {
        Set<String> set = rbac.effectivePermissions(newUser(UserRoles.SUPER_ADMIN));
        assertThat(set).containsAll(PermissionCatalog.allCodes());
        assertThat(set).contains(PermissionCatalog.MODULE_MANAGE);
    }

    @Test
    void 系统管理员_除模块设置外全量() {
        Set<String> set = rbac.effectivePermissions(newUser(UserRoles.ADMIN));
        assertThat(set).contains("user:manage", "fault:delete");
        assertThat(set).doesNotContain(PermissionCatalog.MODULE_MANAGE);
    }

    @Test
    void 运维人员_角色未入库时按内置映射兜底() {
        Set<String> set = rbac.effectivePermissions(newUser(UserRoles.OPERATOR));
        assertThat(set).contains("device:create", "patrol:run", "ai:ops");
        // 不含用户管理与 AI 配置与模块设置
        assertThat(set).doesNotContain("user:manage", "ai:config",
                PermissionCatalog.MODULE_MANAGE);
    }

    @Test
    void 查看人员_只读空集合() {
        assertThat(rbac.effectivePermissions(newUser(UserRoles.VIEWER))).isEmpty();
    }

    @Test
    void 自定义角色_按库内关联计算() {
        seedRole("maintainer", "device:create", "media:upload");
        Set<String> set = rbac.effectivePermissions(newUser("maintainer"));
        assertThat(set).containsExactlyInAnyOrder("device:create", "media:upload");
    }

    @Test
    void 显式授权_全量替换角色默认() {
        seedRole("custom", "device:create", "fault:update");
        Long uid = newUser("custom");
        grant(uid, "media:upload", true);
        Set<String> set = rbac.effectivePermissions(uid);
        assertThat(set).containsExactly("media:upload");
    }

    @Test
    void 显式拒绝_从默认中剔除() {
        seedRole("denier", "device:create", "fault:update", "workorder:create");
        Long uid = newUser("denier");
        grant(uid, "fault:update", false);
        Set<String> set = rbac.effectivePermissions(uid);
        assertThat(set).containsExactlyInAnyOrder("device:create", "workorder:create");
    }

    @Test
    void 硬约束_非超管显式授权module_manage也被剔除() {
        seedRole("sneaky", "device:create");
        Long uid = newUser("sneaky");
        grant(uid, PermissionCatalog.MODULE_MANAGE, true);
        Set<String> set = rbac.effectivePermissions(uid);
        assertThat(set).doesNotContain(PermissionCatalog.MODULE_MANAGE);
    }

    @Test
    void 用户不存在_抛出异常() {
        org.assertj.core.api.Assertions.assertThatThrownBy(
                        () -> rbac.effectivePermissions(999999L))
                .isInstanceOf(IllegalArgumentException.class);
    }

    private void grant(Long userId, String permCode, boolean granted) {
        UserPermission up = new UserPermission();
        up.userId = userId;
        up.permission = permCode;
        up.granted = granted;
        userPerms.saveAndFlush(up);
    }
}

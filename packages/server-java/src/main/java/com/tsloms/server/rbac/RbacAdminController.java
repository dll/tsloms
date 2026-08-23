// RBAC 管理接口：契约对齐 Go 版 handler/rbac.go（全部 role:manage 权限）。
package com.tsloms.server.rbac;

import com.tsloms.server.model.Permission;
import com.tsloms.server.model.Role;
import com.tsloms.server.model.RolePermission;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserPermission;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.repository.PermissionRepository;
import com.tsloms.server.repository.RolePermissionRepository;
import com.tsloms.server.repository.RoleRepository;
import com.tsloms.server.repository.UserPermissionRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class RbacAdminController {

    private final PermissionRepository permissions;
    private final RoleRepository roles;
    private final RolePermissionRepository rolePerms;
    private final UserPermissionRepository userPerms;
    private final UserRepository users;
    private final RbacService rbacService;

    public RbacAdminController(PermissionRepository permissions, RoleRepository roles,
                               RolePermissionRepository rolePerms,
                               UserPermissionRepository userPerms,
                               UserRepository users, RbacService rbacService) {
        this.permissions = permissions;
        this.roles = roles;
        this.rolePerms = rolePerms;
        this.userPerms = userPerms;
        this.users = users;
        this.rbacService = rbacService;
    }

    /** GET /my/permissions：当前用户角色与有效权限（登录即可，供前端菜单联动）。 */
    @GetMapping("/my/permissions")
    public ApiResponse<Map<String, Object>> myPermissions(HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        String role = (String) request.getAttribute(AuthInterceptor.ATTR_USER_ROLE);
        List<String> perms = rbacService.effectivePermissionCodes(userId);
        return ApiResponse.ok(Map.of("role", role == null ? "" : role, "permissions", perms));
    }

    /** GET /rbac/permissions：权限点字典（按模块分组，sort,id 排序）。 */
    @GetMapping("/rbac/permissions")
    @RequirePerm("role:manage")
    public ApiResponse<Map<String, Object>> listPermissions() {
        List<Permission> all = permissions.findAll(
                Sort.by(Sort.Direction.ASC, "sort").and(Sort.by(Sort.Direction.ASC, "id")));

        List<Map<String, Object>> modules = new ArrayList<>();
        Map<String, List<Map<String, Object>>> byModule = new LinkedHashMap<>();
        for (Permission p : all) {
            byModule.computeIfAbsent(p.module, k -> new ArrayList<>())
                    .add(Map.of("id", p.id, "code", p.code, "name", p.name,
                            "module", p.module, "sort", p.sort));
        }
        byModule.forEach((m, ps) -> modules.add(Map.of("module", m, "permissions", ps)));
        return ApiResponse.ok(Map.of("list", modules, "total", all.size()));
    }

    /** GET /rbac/roles：角色列表（含默认权限编码；内置优先）。 */
    @GetMapping("/rbac/roles")
    @RequirePerm("role:manage")
    public ApiResponse<Map<String, Object>> listRoles() {
        List<Role> rows = roles.findAll(
                Sort.by(Sort.Direction.DESC, "builtin").and(Sort.by(Sort.Direction.ASC, "id")));
        Map<Long, String> permCodeById = new HashMap<>();
        permissions.findAll().forEach(p -> permCodeById.put(p.id, p.code));

        List<Map<String, Object>> list = new ArrayList<>();
        for (Role r : rows) {
            List<String> codes = rolePerms.findByRoleId(r.id).stream()
                    .map(rp -> permCodeById.get(rp.permissionId))
                    .filter(java.util.Objects::nonNull)
                    .toList();
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", r.id);
            m.put("code", r.code);
            m.put("name", r.name);
            m.put("builtin", r.builtin);
            m.put("description", r.description);
            m.put("permissions", codes);
            m.put("created_at", r.createdAt);
            list.add(m);
        }
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** 角色请求体。 */
    public record RoleRequest(String code, String name, String description,
                              List<String> permissions) {
    }

    /** POST /rbac/roles：创建自定义角色。 */
    @PostMapping("/rbac/roles")
    @RequirePerm("role:manage")
    public ResponseEntity<?> createRole(@RequestBody RoleRequest req) {
        if (req.code() == null || req.code().isBlank()
                || req.name() == null || req.name().isBlank()) {
            return badRequest("参数错误（code/name 必填）");
        }
        if (UserRoles.ADMIN.equals(req.code()) || UserRoles.OPERATOR.equals(req.code())
                || UserRoles.VIEWER.equals(req.code())) {
            return badRequest("不能创建与内置角色同名的角色");
        }
        if (roles.findByCode(req.code()).isPresent()) {
            return badRequest("角色编码已存在");
        }
        Role r = new Role();
        r.code = req.code();
        r.name = req.name();
        r.description = req.description();
        r.builtin = false;
        roles.save(r);
        setRolePermissions(r.id, r.code, req.permissions());
        return ok(Map.of("id", r.id, "code", r.code, "message", "角色创建成功"));
    }

    /** PUT /rbac/roles/{id}：更新自定义角色（名称/描述/权限）。 */
    @PutMapping("/rbac/roles/{id}")
    @RequirePerm("role:manage")
    public ResponseEntity<?> updateRole(@PathVariable Long id, @RequestBody Map<String, Object> body) {
        var roleOpt = roles.findById(id);
        if (roleOpt.isEmpty()) {
            return notFound("角色不存在");
        }
        Role r = roleOpt.get();
        if (r.builtin) {
            return badRequest("内置角色不可编辑，可通过用户级权限覆写调整");
        }
        Object name = body.get("name");
        if (name != null) {
            r.name = String.valueOf(name);
        }
        Object desc = body.get("description");
        if (desc != null) {
            r.description = String.valueOf(desc);
        }
        roles.save(r);
        @SuppressWarnings("unchecked")
        List<String> perms = (List<String>) body.get("permissions");
        if (perms != null) {
            setRolePermissions(r.id, r.code, perms);
        }
        return ok(Map.of("message", "角色更新成功"));
    }

    /** DELETE /rbac/roles/{id}：内置角色与有用户的角色不可删。 */
    @DeleteMapping("/rbac/roles/{id}")
    @RequirePerm("role:manage")
    public ResponseEntity<?> deleteRole(@PathVariable Long id) {
        var roleOpt = roles.findById(id);
        if (roleOpt.isEmpty()) {
            return notFound("角色不存在");
        }
        Role r = roleOpt.get();
        if (r.builtin) {
            return badRequest("内置角色不可删除");
        }
        if (users.countByRole(r.code) > 0) {
            return badRequest("该角色已有用户绑定，不可删除");
        }
        rolePerms.findByRoleId(r.id).forEach(rolePerms::delete);
        roles.delete(r);
        return ok(Map.of("message", "角色删除成功"));
    }

    /** GET /rbac/users/{id}/permissions：用户权限回显（角色默认 + 覆写）。 */
    @GetMapping("/rbac/users/{id}/permissions")
    @RequirePerm("role:manage")
    public ResponseEntity<?> getUserPermissions(@PathVariable Long id) {
        var userOpt = users.findById(id);
        if (userOpt.isEmpty()) {
            return notFound("用户不存在");
        }
        User user = userOpt.get();

        // 角色默认权限（库内关联）
        List<String> roleDefaults = new ArrayList<>();
        roles.findByCode(user.role).ifPresent(role ->
                rolePerms.findByRoleId(role.id).forEach(rp ->
                        permissions.findById(rp.permissionId)
                                .ifPresent(p -> roleDefaults.add(p.code))));

        // 用户级覆写
        List<String> grants = new ArrayList<>();
        List<String> denies = new ArrayList<>();
        for (UserPermission up : userPerms.findByUserId(id)) {
            if (up.granted) {
                grants.add(up.permission);
            } else {
                denies.add(up.permission);
            }
        }
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("role", user.role);
        data.put("role_defaults", roleDefaults);
        data.put("user_grants", grants);
        data.put("user_denies", denies);
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 用户权限覆写请求体。 */
    public record SetUserPermsRequest(List<String> grants, List<String> denies) {
    }

    /** PUT /rbac/users/{id}/permissions：设置用户级覆写。 */
    @PutMapping("/rbac/users/{id}/permissions")
    @RequirePerm("role:manage")
    public ResponseEntity<?> setUserPermissions(@PathVariable Long id,
                                                @RequestBody SetUserPermsRequest req) {
        var userOpt = users.findById(id);
        if (userOpt.isEmpty()) {
            return notFound("用户不存在");
        }
        User user = userOpt.get();
        // 内置 admin 恒为全权限，禁止覆写
        if (UserRoles.ADMIN.equals(user.role) && "admin".equals(user.username)) {
            return badRequest("内置 admin 拥有全部权限，不可覆写");
        }
        SetUserPermsRequest body = req == null ? new SetUserPermsRequest(null, null) : req;
        List<String> validCodes = PermissionCatalog.allCodes();

        userPerms.findByUserId(id).forEach(userPerms::delete);
        if (body.grants() != null && !body.grants().isEmpty()) {
            body.grants().stream().filter(validCodes::contains).forEach(code -> {
                UserPermission up = new UserPermission();
                up.userId = id;
                up.permission = code;
                up.granted = true;
                userPerms.save(up);
            });
        } else if (body.denies() != null) {
            body.denies().stream().filter(validCodes::contains).forEach(code -> {
                UserPermission up = new UserPermission();
                up.userId = id;
                up.permission = code;
                up.granted = false;
                userPerms.save(up);
            });
        }
        return ok(Map.of("message", "功能权限已更新"));
    }

    /** 覆盖角色权限关联（对齐 Go 版 setRolePermissions：未知编码跳过）。 */
    private void setRolePermissions(Long roleId, String roleCode, List<String> codes) {
        rolePerms.findByRoleId(roleId).forEach(rolePerms::delete);
        if (codes == null) {
            return;
        }
        for (String code : codes) {
            permissions.findByCode(code).ifPresent(p -> {
                RolePermission rp = new RolePermission();
                rp.roleId = roleId;
                rp.permissionId = p.id;
                rp.roleCode = roleCode;
                rolePerms.save(rp);
            });
        }
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> ok(Map<String, Object> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

// 用户管理接口：契约对齐 Go 版 handler/user.go（user:manage 权限组）。
package com.tsloms.server.auth;

import com.tsloms.server.model.PasswordHasher;
import com.tsloms.server.model.User;
import com.tsloms.server.model.UserRoles;
import com.tsloms.server.model.UserStatuses;
import com.tsloms.server.repository.DepartmentRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * 用户管理（管理员）。路由与语义逐条对齐 Go 版：
 *
 * <pre>
 * GET    /api/v1/users                  分页+筛选（排除 super_admin）
 * POST   /api/v1/users                  创建（强密码/角色/手机号/唯一性校验）
 * PUT    /api/v1/users/{id}             局部更新
 * PUT    /api/v1/users/{id}/password    重置密码
 * DELETE /api/v1/users/{id}             删除（禁自删/内置 admin 保护）
 * </pre>
 */
@RestController
@RequestMapping("/api/v1")
public class UserAdminController {

    private static final Pattern PHONE = Pattern.compile("^1[3-9]\\d{9}$");

    private final UserRepository users;
    private final DepartmentRepository departments;
    private final PasswordHasher hasher;

    public UserAdminController(UserRepository users, DepartmentRepository departments,
                               PasswordHasher hasher) {
        this.users = users;
        this.departments = departments;
        this.hasher = hasher;
    }

    /** GET /users：分页 + 角色/部门/状态/关键字筛选。 */
    @GetMapping("/users")
    @RequirePerm("user:manage")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String role,
            @RequestParam(required = false) String status,
            @RequestParam(name = "department_id", required = false) String departmentId,
            @RequestParam(required = false) String keyword,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<User> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            // 超级管理员是系统内部账号，不进入普通用户管理列表
            preds.add(cb.notEqual(root.get("role"), UserRoles.SUPER_ADMIN));
            if (role != null && !role.isBlank()) {
                preds.add(cb.equal(root.get("role"), role));
            }
            if (status != null && !status.isBlank()) {
                preds.add(cb.equal(root.get("status"), status));
            }
            if (departmentId != null && !departmentId.isBlank()) {
                try {
                    long did = Long.parseLong(departmentId.trim());
                    if (did > 0) {
                        preds.add(cb.equal(root.get("departmentId"), did));
                    }
                } catch (NumberFormatException ignored) {
                    // 与 Go 版一致：解析失败忽略该筛选
                }
            }
            if (keyword != null && !keyword.isBlank()) {
                String like = "%" + keyword + "%";
                preds.add(cb.or(
                        cb.like(root.get("username"), like),
                        cb.like(root.get("realName"), like)));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = users.count(spec);
        List<User> rows = users.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.ASC, "id")))
                .getContent();

        // 部门名称映射（对齐 Go 版 deptNames）
        Map<Long, String> deptNames = new HashMap<>();
        rows.stream().map(u -> u.departmentId).filter(java.util.Objects::nonNull).distinct()
                .forEach(id -> departments.findById(id)
                        .ifPresent(d -> deptNames.put(d.id, d.name)));

        List<Map<String, Object>> safeUsers = new ArrayList<>();
        for (User u : rows) {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", u.id);
            m.put("username", u.username);
            m.put("role", u.role);
            m.put("real_name", u.realName);
            m.put("phone", u.phone);
            m.put("email", u.email);
            m.put("department_id", u.departmentId);
            m.put("department", u.departmentId == null ? "" : deptNames.getOrDefault(u.departmentId, ""));
            m.put("status", u.status);
            m.put("last_login_at", u.lastLoginAt);
            m.put("created_at", u.createdAt);
            safeUsers.add(m);
        }

        return ApiResponse.ok(Map.of(
                "list", safeUsers,
                "total", total,
                "page", pg.page(),
                "page_size", pg.pageSize()));
    }

    /** POST /users。 */
    /** 创建用户请求体（对齐 Go 版 CreateUser req，snake_case 由全局 Jackson 处理）。 */
    public record CreateUserReq(
            String username, String password, String role, String realName,
            String phone, String email, Long departmentId, String workNo,
            String avatar, String gender, String idCard, String address,
            String education, String engineerLevel) {
    }

    @PostMapping("/users")
    @RequirePerm("user:manage")
    public ResponseEntity<?> create(@RequestBody CreateUserReq req) {
        if (req.username() == null || req.username().isBlank()
                || req.password() == null || req.password().isBlank()
                || req.role() == null || req.role().isBlank()) {
            return badRequest("参数错误（用户名/密码必填）");
        }
        String pwMsg = validatePasswordStrength(req.password());
        if (!pwMsg.isEmpty()) {
            return badRequest(pwMsg);
        }
        if (!validRole(req.role())) {
            return badRequest("无效的角色");
        }
        String phone = trimToEmpty(req.phone());
        if (!phone.isEmpty() && !PHONE.matcher(phone).matches()) {
            return badRequest("手机号格式不正确（需 11 位大陆手机号）");
        }
        if (users.existsByUsername(req.username())) {
            return badRequest("用户名已存在");
        }
        if (req.departmentId() != null
                && departments.findById(req.departmentId()).isEmpty()) {
            return badRequest("部门不存在");
        }

        User u = new User();
        u.username = req.username().trim();
        u.passwordHash = hasher.hash(req.password());
        u.role = req.role();
        u.realName = trimToEmpty(req.realName());
        u.phone = phone;
        u.email = trimToEmpty(req.email());
        u.departmentId = req.departmentId();
        u.status = UserStatuses.ENABLED;
        u.workNo = trimToEmpty(req.workNo());
        u.avatar = trimToEmpty(req.avatar());
        u.gender = trimToEmpty(req.gender());
        u.idCard = trimToEmpty(req.idCard());
        u.address = trimToEmpty(req.address());
        u.education = trimToEmpty(req.education());
        u.engineerLevel = trimToEmpty(req.engineerLevel());
        users.save(u);

        return ok(Map.of(
                "id", u.id, "username", u.username, "role", u.role,
                "message", "用户创建成功"));
    }

    /**
     * PUT /users/{id}：局部更新。
     * 用 Map 承接以精确区分"未提供"与"显式空值"，行为对齐 Go 版指针字段。
     */
    @PutMapping("/users/{id}")
    @RequirePerm("user:manage")
    public ResponseEntity<?> update(@PathVariable Long id,
                                    @RequestBody Map<String, Object> body) {
        var userOpt = users.findById(id);
        if (userOpt.isEmpty()) {
            return notFound("用户不存在");
        }
        User user = userOpt.get();

        Map<String, Object> updates = new HashMap<>();
        if (body.containsKey("role")) {
            String role = str(body.get("role"));
            if (validRole(role)) {
                updates.put("role", role);
            }
        }
        if (body.containsKey("real_name")) {
            updates.put("real_name", str(body.get("real_name")).trim());
        }
        if (body.containsKey("phone")) {
            String p = str(body.get("phone")).trim();
            if (!p.isEmpty() && !PHONE.matcher(p).matches()) {
                return badRequest("手机号格式不正确（需 11 位大陆手机号）");
            }
            updates.put("phone", p);
            if (!p.isEmpty()) {
                updates.put("phone_login", p); // 手机号即登录账号，同步更新
            }
        }
        if (body.containsKey("email")) {
            updates.put("email", str(body.get("email")).trim());
        }
        for (String key : new String[]{"work_no", "avatar", "gender",
                "id_card", "address", "education", "engineer_level"}) {
            if (body.containsKey(key)) {
                updates.put(key, str(body.get(key)).trim());
            }
        }
        if (body.containsKey("department_id")) {
            Long did = asLong(body.get("department_id"));
            if (did == null || departments.findById(did).isEmpty()) {
                return badRequest("部门不存在");
            }
            updates.put("department_id", did);
        }
        if (body.containsKey("status")) {
            String st = str(body.get("status"));
            if (!UserStatuses.ENABLED.equals(st) && !UserStatuses.DISABLED.equals(st)) {
                return badRequest("无效的用户状态");
            }
            updates.put("status", st);
        }

        applyUpdates(user, updates);
        users.save(user);

        // 注意：Go 版 gin.H 允许空值字段，此处需用容忍 null 的 Map 构建
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("id", user.id);
        data.put("username", user.username);
        data.put("role", user.role);
        data.put("phone", user.phone);
        data.put("message", "用户更新成功");
        return ok(data);
    }

    /** 重置密码请求体。 */
    public record ResetPasswordRequest(String password) {
    }

    /** PUT /users/{id}/password。 */
    @PutMapping("/users/{id}/password")
    @RequirePerm("user:manage")
    public ResponseEntity<?> resetPassword(@PathVariable Long id,
                                           @RequestBody ResetPasswordRequest req) {
        if (req.password() == null || req.password().isBlank()) {
            return badRequest("密码必填");
        }
        String pwMsg = validatePasswordStrength(req.password());
        if (!pwMsg.isEmpty()) {
            return badRequest(pwMsg);
        }
        var userOpt = users.findById(id);
        if (userOpt.isEmpty()) {
            return notFound("用户不存在");
        }
        User user = userOpt.get();
        user.passwordHash = hasher.hash(req.password());
        users.save(user);
        return ok(Map.of("message", "密码重置成功"));
    }

    /** DELETE /users/{id}：禁止删除自己；内置 admin 保护。 */
    @DeleteMapping("/users/{id}")
    @RequirePerm("user:manage")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        Long currentUserId = AuthInterceptor.userId(request);
        if (id.equals(currentUserId)) {
            return badRequest("不能删除当前登录用户");
        }
        var userOpt = users.findById(id);
        if (userOpt.isEmpty()) {
            return notFound("用户不存在");
        }
        if ("admin".equals(userOpt.get().username)) {
            return badRequest("内置 admin 账号不可删除");
        }
        users.delete(userOpt.get());
        return ok(Map.of("message", "用户删除成功"));
    }

    // ------------------------------------------------------------------
    // 自助接口（登录即可，无 user:manage 要求）
    // ------------------------------------------------------------------

    /** PUT /user/phone：修改当前登录用户手机号。 */
    @PutMapping("/user/phone")
    public ResponseEntity<?> updateMyPhone(HttpServletRequest request,
                                           @RequestBody Map<String, Object> body) {
        String phone = str(body.get("phone"));
        if (phone.isEmpty()) {
            return badRequest("手机号不能为空");
        }
        Long userId = AuthInterceptor.userId(request);
        if (userId == null) {
            return ResponseEntity.status(401)
                    .body(ApiResponse.fail("unauthorized", "未登录"));
        }
        return users.findById(userId).map(u -> {
            u.phone = phone;
            users.save(u);
            return ResponseEntity.ok((Object) ApiResponse.ok(Map.of("message", "手机号已更新")));
        }).orElse(ResponseEntity.status(401).body(ApiResponse.fail("unauthorized", "未登录")));
    }

    /** PUT /user/center：设置/清除当前用户的地图中心。 */
    public record CenterRequest(Double lat, Double lng) {
    }

    @PutMapping("/user/center")
    public ResponseEntity<?> updateMyCenter(HttpServletRequest request,
                                            @RequestBody CenterRequest req) {
        Long userId = AuthInterceptor.userId(request);
        if (userId == null) {
            return ResponseEntity.status(401)
                    .body(ApiResponse.fail("unauthorized", "未登录"));
        }
        if (req.lat() != null && (req.lat() < -90 || req.lat() > 90)) {
            return badRequest("纬度范围 -90~90");
        }
        if (req.lng() != null && (req.lng() < -180 || req.lng() > 180)) {
            return badRequest("经度范围 -180~180");
        }
        return users.findById(userId).map(u -> {
            u.centerLat = req.lat();
            u.centerLng = req.lng();
            users.save(u);
            return ResponseEntity.ok((Object) ApiResponse.ok(Map.of("message", "地图中心点已更新")));
        }).orElse(ResponseEntity.status(401).body(ApiResponse.fail("unauthorized", "未登录")));
    }

    /** GET /users/assignable：可派单人员（admin+operator），登录用户均可调用。 */
    @GetMapping("/users/assignable")
    public ApiResponse<Map<String, Object>> assignable() {
        List<Map<String, Object>> list = new ArrayList<>();
        users.findAll(Sort.by(Sort.Direction.ASC, "id")).forEach(u -> {
            if (UserRoles.ADMIN.equals(u.role) || UserRoles.OPERATOR.equals(u.role)) {
                list.add(Map.of("id", u.id, "username", u.username, "role", u.role));
            }
        });
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    // ------------------------------------------------------------------
    // 校验与小工具（对齐 Go 版同名函数）
    // ------------------------------------------------------------------

    /** 强密码校验：≥10 位且同时含字母与数字（审计 #2）。 */
    static String validatePasswordStrength(String pw) {
        if (pw.length() < 10) {
            return "密码至少10位";
        }
        boolean hasLetter = false;
        boolean hasDigit = false;
        for (char r : pw.toCharArray()) {
            if ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
                hasLetter = true;
            } else if (r >= '0' && r <= '9') {
                hasDigit = true;
            }
        }
        if (!hasLetter || !hasDigit) {
            return "密码需同时包含字母和数字";
        }
        return "";
    }

    static boolean validRole(String role) {
        return UserRoles.SUPER_ADMIN.equals(role) || UserRoles.ADMIN.equals(role)
                || UserRoles.OPERATOR.equals(role) || UserRoles.VIEWER.equals(role);
    }

    private void applyUpdates(User user, Map<String, Object> updates) {
        if (updates.containsKey("role")) {
            user.role = (String) updates.get("role");
        }
        if (updates.containsKey("real_name")) {
            user.realName = (String) updates.get("real_name");
        }
        if (updates.containsKey("phone")) {
            user.phone = (String) updates.get("phone");
        }
        if (updates.containsKey("phone_login")) {
            user.phoneLogin = (String) updates.get("phone_login");
        }
        if (updates.containsKey("email")) {
            user.email = (String) updates.get("email");
        }
        if (updates.containsKey("work_no")) {
            user.workNo = (String) updates.get("work_no");
        }
        if (updates.containsKey("avatar")) {
            user.avatar = (String) updates.get("avatar");
        }
        if (updates.containsKey("gender")) {
            user.gender = (String) updates.get("gender");
        }
        if (updates.containsKey("id_card")) {
            user.idCard = (String) updates.get("id_card");
        }
        if (updates.containsKey("address")) {
            user.address = (String) updates.get("address");
        }
        if (updates.containsKey("education")) {
            user.education = (String) updates.get("education");
        }
        if (updates.containsKey("engineer_level")) {
            user.engineerLevel = (String) updates.get("engineer_level");
        }
        if (updates.containsKey("department_id")) {
            user.departmentId = (Long) updates.get("department_id");
        }
        if (updates.containsKey("status")) {
            user.status = (String) updates.get("status");
        }
    }

    private static String str(Object o) {
        return o == null ? "" : String.valueOf(o);
    }

    private static Long asLong(Object o) {
        if (o instanceof Number n) {
            return n.longValue();
        }
        try {
            return o == null ? null : Long.parseLong(String.valueOf(o));
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static String trimToEmpty(String s) {
        return s == null ? "" : s.trim();
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

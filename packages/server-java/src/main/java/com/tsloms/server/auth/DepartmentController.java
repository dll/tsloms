// 部门管理接口：契约对齐 Go 版 handler/department.go。
package com.tsloms.server.auth;

import com.tsloms.server.model.Department;
import com.tsloms.server.repository.DepartmentRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.RequirePerm;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
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
public class DepartmentController {

    private final DepartmentRepository departments;
    private final UserRepository users;

    public DepartmentController(DepartmentRepository departments, UserRepository users) {
        this.departments = departments;
        this.users = users;
    }

    /** GET /departments：登录即可；含各部门成员数（对齐 Go 版 ListDepartments）。 */
    @GetMapping("/departments")
    public ApiResponse<Map<String, Object>> list() {
        Map<Long, Long> memberCount = new LinkedHashMap<>();
        users.countByDepartmentIdGrouped()
                .forEach(r -> memberCount.put(r.getDepartmentId(), r.getCnt()));

        List<Map<String, Object>> out = new ArrayList<>();
        for (Department d : departments.findAll(org.springframework.data.domain.Sort.by("id"))) {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", d.id);
            m.put("name", d.name);
            m.put("parent_id", d.parentId);
            m.put("leader", d.leader);
            m.put("description", d.description);
            m.put("member_count", memberCount.getOrDefault(d.id, 0L));
            m.put("created_at", d.createdAt);
            out.add(m);
        }
        return ApiResponse.ok(Map.of("list", out, "total", out.size()));
    }

    /** 部门请求体。 */
    public record DeptRequest(String name, Long parentId, String leader, String description) {
    }

    /** POST /departments（dept:manage）。 */
    @PostMapping("/departments")
    @RequirePerm("dept:manage")
    public ResponseEntity<?> create(@RequestBody DeptRequest req) {
        if (req.name() == null || req.name().isBlank()) {
            return badRequest("部门名称必填");
        }
        if (departments.existsByName(req.name())) {
            return badRequest("部门名称已存在");
        }
        if (req.parentId() != null && departments.findById(req.parentId()).isEmpty()) {
            return badRequest("上级部门不存在");
        }
        Department d = new Department();
        d.name = req.name();
        d.parentId = req.parentId();
        d.leader = req.leader();
        d.description = req.description();
        departments.save(d);
        return ok(Map.of("id", d.id, "name", d.name, "message", "部门创建成功"));
    }

    /** PUT /departments/{id}（dept:manage）：局部更新，Map 区分未提供与空值。 */
    @PutMapping("/departments/{id}")
    @RequirePerm("dept:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody Map<String, Object> body) {
        var deptOpt = departments.findById(id);
        if (deptOpt.isEmpty()) {
            return notFound("部门不存在");
        }
        Department dept = deptOpt.get();

        if (body.containsKey("name")) {
            String name = str(body.get("name"));
            if (!name.isEmpty()) {
                boolean dup = departments.findAll().stream()
                        .anyMatch(x -> x.name.equals(name) && !x.id.equals(id));
                if (dup) {
                    return badRequest("部门名称已存在");
                }
                dept.name = name;
            }
        }
        if (body.containsKey("leader")) {
            dept.leader = str(body.get("leader"));
        }
        if (body.containsKey("description")) {
            dept.description = str(body.get("description"));
        }
        if (body.containsKey("parent_id")) {
            Long pid = asLong(body.get("parent_id"));
            if (pid != null) {
                if (pid.equals(id)) {
                    return badRequest("上级部门不能是自身");
                }
                if (departments.findById(pid).isEmpty()) {
                    return badRequest("上级部门不存在");
                }
                dept.parentId = pid;
            }
        }
        departments.save(dept);
        return ok(Map.of("message", "部门更新成功"));
    }

    /** DELETE /departments/{id}（dept:manage）：仍有成员则拒绝。 */
    @DeleteMapping("/departments/{id}")
    @RequirePerm("dept:manage")
    public ResponseEntity<?> delete(@PathVariable Long id) {
        var deptOpt = departments.findById(id);
        if (deptOpt.isEmpty()) {
            return notFound("部门不存在");
        }
        long memberCount = users.countByDepartmentId(id);
        if (memberCount > 0) {
            return badRequest("该部门下仍有用户，无法删除");
        }
        departments.delete(deptOpt.get());
        return ok(Map.of("message", "部门删除成功"));
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

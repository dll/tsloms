// 供应商接口：列表(all=1全量)/保存(id>0更新)/删除。契约对齐 Go 版 handler/supplier.go。
package com.tsloms.server.inventory;

import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.Supplier;
import com.tsloms.server.repository.SupplierRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
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

@RestController
@RequestMapping("/api/v1")
public class SupplierController {

    private final SupplierRepository suppliers;
    private final OperationLogService opLog;

    public SupplierController(SupplierRepository suppliers, OperationLogService opLog) {
        this.suppliers = suppliers;
        this.opLog = opLog;
    }

    /** GET /suppliers：all=1 全量（下拉用），否则 keyword 分页筛选。 */
    @GetMapping("/suppliers")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String all,
            @RequestParam(required = false) String keyword,
            HttpServletRequest request) {
        if ("1".equals(all) || "true".equals(all)) {
            List<Supplier> rows = suppliers.findAll(Sort.by(Sort.Direction.DESC, "createdAt"));
            Map<String, Object> data = new LinkedHashMap<>();
            data.put("list", rows);
            data.put("total", rows.size());
            return ApiResponse.ok(data);
        }
        Pagination.Page pg = Pagination.of(request);
        Specification<Supplier> spec = (root, query, cb) -> {
            if (keyword == null || keyword.isBlank()) {
                return cb.conjunction();
            }
            String like = "%" + keyword.trim() + "%";
            return cb.or(cb.like(root.get("name"), like),
                    cb.like(root.get("contact"), like),
                    cb.like(root.get("phone"), like));
        };
        long total = suppliers.count(spec);
        List<Object> rows = new ArrayList<>();
        suppliers.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .forEach(rows::add);
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** 保存请求体（id>0 为更新）。 */
    public record SaveRequest(Long id, String name, String contact, String phone,
                              String address, String email, String status, String note) {
    }

    /** POST /suppliers：新增。 */
    @PostMapping("/suppliers")
    @RequirePerm("supplier:manage")
    public ResponseEntity<?> create(@RequestBody SaveRequest req, HttpServletRequest request) {
        if (req.name() == null || req.name().isBlank()) {
            return badRequest("参数错误（name 必填）");
        }
        Supplier s = new Supplier();
        apply(s, req);
        suppliers.save(s);
        opLog.record(request, OpTypes.CREATE, "supplier/" + s.id, "新增供应商 " + s.name);
        return ok(Map.of("supplier", s, "message", "供应商已创建"));
    }

    /** PUT /suppliers/{id}：更新。 */
    @PutMapping("/suppliers/{id}")
    @RequirePerm("supplier:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody SaveRequest req,
                                    HttpServletRequest request) {
        var opt = suppliers.findById(id);
        if (opt.isEmpty()) {
            return notFound("供应商不存在");
        }
        apply(opt.get(), req);
        suppliers.save(opt.get());
        opLog.record(request, OpTypes.UPDATE, "supplier/" + id, "更新供应商 " + opt.get().name);
        return ok(Map.of("message", "供应商已更新"));
    }

    private void apply(Supplier s, SaveRequest req) {
        s.name = nz(req.name());
        s.contact = nz(req.contact());
        s.phone = nz(req.phone());
        s.address = nz(req.address());
        s.email = nz(req.email());
        s.status = (req.status() == null || req.status().isBlank())
                ? "active" : req.status();
        s.note = nz(req.note());
    }

    /** DELETE /suppliers/{id}。 */
    @DeleteMapping("/suppliers/{id}")
    @RequirePerm("supplier:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = suppliers.findById(id);
        if (opt.isEmpty()) {
            return notFound("供应商不存在");
        }
        suppliers.delete(opt.get());
        opLog.record(request, OpTypes.DELETE, "supplier/" + id, "删除供应商");
        return ok(Map.of("message", "删除成功"));
    }

    static String nz(String s) {
        return s == null ? "" : s;
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

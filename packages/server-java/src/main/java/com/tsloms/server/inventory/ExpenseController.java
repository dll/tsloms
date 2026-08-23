// 维修费用接口：统计/列表/保存/更新/确认/删除。契约对齐 Go 版 handler/expense.go。
package com.tsloms.server.inventory;

import com.tsloms.server.model.ExpenseTypes;
import com.tsloms.server.model.RepairExpense;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.RepairExpenseRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Comparator;
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
public class ExpenseController {

    private final RepairExpenseRepository expenses;
    private final WorkOrderRepository workOrders;
    @SuppressWarnings("unused")
    private final UserRepository users;
    private final OperationLogService opLog;

    public ExpenseController(RepairExpenseRepository expenses,
                             WorkOrderRepository workOrders, UserRepository users,
                             OperationLogService opLog) {
        this.expenses = expenses;
        this.workOrders = workOrders;
        this.users = users;
        this.opLog = opLog;
    }

    /** GET /expenses/stats：总额/分类合计/设备维修成本 TOP10。 */
    @GetMapping("/expenses/stats")
    public ApiResponse<Map<String, Object>> stats() {
        List<RepairExpense> all = expenses.findAll();
        double total = 0;
        double material = 0;
        double labor = 0;
        double traffic = 0;
        double other = 0;
        Map<String, Double> byDevice = new LinkedHashMap<>();
        for (var e : all) {
            total += e.amount;
            switch (e.type) {
                case ExpenseTypes.MATERIAL -> material += e.amount;
                case ExpenseTypes.LABOR -> labor += e.amount;
                case ExpenseTypes.TRAFFIC -> traffic += e.amount;
                default -> other += e.amount;
            }
            if (e.deviceHwId != null && !e.deviceHwId.isEmpty()) {
                byDevice.merge(e.deviceHwId, e.amount, Double::sum);
            }
        }
        List<Map<String, Object>> topDevices = byDevice.entrySet().stream()
                .sorted(Map.Entry.<String, Double>comparingByValue().reversed())
                .limit(10)
                .map(e -> Map.<String, Object>of("device_hw_id", e.getKey(),
                        "total", e.getValue()))
                .toList();
        return ApiResponse.ok(Map.of(
                "total_amount", Math.round(total * 100.0) / 100.0,
                "total_count", all.size(),
                "material", Math.round(material * 100.0) / 100.0,
                "labor", Math.round(labor * 100.0) / 100.0,
                "traffic", Math.round(traffic * 100.0) / 100.0,
                "other", Math.round(other * 100.0) / 100.0,
                "top_devices", topDevices));
    }

    /** GET /expenses：分页多条件筛选（confirmed=true/1 过滤已确认）。 */
    @GetMapping("/expenses")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(name = "work_order_id", required = false) String workOrderId,
            @RequestParam(required = false) String type,
            @RequestParam(required = false) String from,
            @RequestParam(required = false) String to,
            @RequestParam(required = false) String confirmed,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<RepairExpense> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (deviceHwId != null && !deviceHwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), deviceHwId));
            }
            if (workOrderId != null && !workOrderId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("workOrderId"), Long.parseLong(workOrderId)));
                } catch (NumberFormatException ignored) {
                    // 与 Go 一致忽略非法值
                }
            }
            if (type != null && !type.isBlank()) {
                preds.add(cb.equal(root.get("type"), type));
            }
            LocalDate f = parseDate(from);
            if (f != null) {
                preds.add(cb.greaterThanOrEqualTo(root.get("workDate"), f));
            }
            LocalDate t = parseDate(to);
            if (t != null) {
                preds.add(cb.lessThanOrEqualTo(root.get("workDate"), t));
            }
            if ("true".equals(confirmed) || "1".equals(confirmed)) {
                preds.add(cb.isTrue(root.get("confirmed")));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = expenses.count(spec);
        List<RepairExpense> rows = expenses.findAll(spec,
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .getContent();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** 保存请求体（id>0 更新；work_date yyyy-MM-dd）。 */
    public record SaveRequest(Long id, String type, Double amount, String deviceHwId,
                              Long workOrderId, String description, String workDate,
                              Boolean confirmed, String note) {
    }

    /** POST /expenses（expense:manage）：新增（FE 单号生成+工单校验带出设备）。 */
    @PostMapping("/expenses")
    @RequirePerm("expense:manage")
    public ResponseEntity<?> create(@RequestBody SaveRequest req, HttpServletRequest request) {
        return doSave(req, request, false);
    }

    /** PUT /expenses/{id}：更新。 */
    @PutMapping("/expenses/{id}")
    @RequirePerm("expense:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody SaveRequest req,
                                    HttpServletRequest request) {
        SaveRequest withId = new SaveRequest(id, req.type(), req.amount(), req.deviceHwId(),
                req.workOrderId(), req.description(), req.workDate(), req.confirmed(),
                req.note());
        return doSave(withId, request, true);
    }

    private ResponseEntity<?> doSave(SaveRequest req, HttpServletRequest request,
                                     boolean isUpdate) {
        List<String> okTypes = List.of(ExpenseTypes.MATERIAL, ExpenseTypes.LABOR,
                ExpenseTypes.TRAFFIC, ExpenseTypes.OTHER);
        if (req.type() == null || !okTypes.contains(req.type())) {
            return badRequest("费用类型不合法");
        }
        if (req.amount() == null || req.amount() < 0) {
            return badRequest("费用金额不能为负");
        }

        // 工单存在时校验并自动补设备 ID
        String hwId = nz(req.deviceHwId());
        Long workOrderId = req.workOrderId();
        if (workOrderId != null && workOrderId > 0) {
            WorkOrder wo = workOrders.findById(workOrderId).orElse(null);
            if (wo == null) {
                return badRequest("关联工单不存在");
            }
            if (hwId.isEmpty()) {
                hwId = wo.deviceHwId;
            }
        } else {
            workOrderId = null;
        }

        LocalDate workDate = null;
        if (req.workDate() != null && !req.workDate().isBlank()) {
            try {
                workDate = LocalDate.parse(req.workDate().trim(),
                        DateTimeFormatter.ofPattern("yyyy-MM-dd"));
            } catch (Exception e) {
                return badRequest("日期格式应为 yyyy-MM-dd");
            }
        }

        if (isUpdate) {
            var opt = expenses.findById(req.id());
            if (opt.isEmpty()) {
                return notFound("费用记录不存在");
            }
            RepairExpense e = opt.get();
            apply(e, req, workDate, hwId, workOrderId);
            expenses.save(e);
            opLog.record(request, com.tsloms.server.model.OpTypes.UPDATE,
                    "expense/" + e.id, "更新维修费用");
            return ok(Map.of("message", "费用已更新"));
        }

        RepairExpense e = new RepairExpense();
        e.expenseNo = BizNo.next(expenses::countByExpenseNoStartingWith, "FE");
        apply(e, req, workDate, hwId, workOrderId);
        e.operator = operatorOf(request);
        expenses.save(e);
        opLog.record(request, com.tsloms.server.model.OpTypes.CREATE,
                "expense/" + e.id, "登记维修费用");
        return ok(Map.of("message", "费用已创建", "expense_no", e.expenseNo));
    }

    private void apply(RepairExpense e, SaveRequest req, LocalDate workDate,
                       String hwId, Long workOrderId) {
        e.type = req.type();
        e.amount = req.amount();
        e.deviceHwId = hwId;
        e.workOrderId = workOrderId;
        e.description = nz(req.description());
        e.workDate = workDate;
        e.confirmed = Boolean.TRUE.equals(req.confirmed());
        e.note = nz(req.note());
    }

    /** PUT /expenses/{id}/confirm（expense:manage）：确认入账。 */
    @PutMapping("/expenses/{id}/confirm")
    @RequirePerm("expense:manage")
    public ResponseEntity<?> confirm(@PathVariable Long id, HttpServletRequest request) {
        var opt = expenses.findById(id);
        if (opt.isEmpty()) {
            return notFound("费用记录不存在");
        }
        RepairExpense e = opt.get();
        e.confirmed = true;
        expenses.save(e);
        opLog.record(request, com.tsloms.server.model.OpTypes.UPDATE,
                "expense/" + id, "确认费用 " + e.expenseNo);
        return ok(Map.of("message", "费用已确认"));
    }

    /** DELETE /expenses/{id}（expense:delete）。 */
    @DeleteMapping("/expenses/{id}")
    @RequirePerm("expense:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = expenses.findById(id);
        if (opt.isEmpty()) {
            return notFound("费用记录不存在");
        }
        expenses.delete(opt.get());
        opLog.record(request, com.tsloms.server.model.OpTypes.DELETE,
                "expense/" + id, "删除费用记录");
        return ok(Map.of("message", "删除成功"));
    }

    private static String nz(String s) {
        return s == null ? "" : s;
    }

    private static String operatorOf(HttpServletRequest request) {
        Object u = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        String name = u == null ? "" : String.valueOf(u);
        return name.isEmpty() ? "system" : name;
    }

    private static LocalDate parseDate(String s) {
        if (s == null || s.isBlank()) {
            return null;
        }
        try {
            return LocalDate.parse(s.trim(), DateTimeFormatter.ofPattern("yyyy-MM-dd"));
        } catch (Exception e) {
            return null;
        }
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

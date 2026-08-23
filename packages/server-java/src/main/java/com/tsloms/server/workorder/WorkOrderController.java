// 工单接口：列表/详情(SLA+故障)/手动创建/状态流转/派单/删除，契约对齐 Go 版 handler/workorder.go。
package com.tsloms.server.workorder;

import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.OperationLog;
import com.tsloms.server.model.User;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.OperationLogRepository;
import com.tsloms.server.repository.UserRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.util.ArrayList;
import java.util.HashMap;
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
public class WorkOrderController {

    private final WorkOrderRepository orders;
    private final FaultRecordRepository faults;
    private final UserRepository users;
    private final DeviceRepository devices;
    private final OperationLogRepository opLogs;
    private final OperationLogService opLog;

    public WorkOrderController(WorkOrderRepository orders, FaultRecordRepository faults,
                               UserRepository users, DeviceRepository devices,
                               OperationLogRepository opLogs, OperationLogService opLog) {
        this.orders = orders;
        this.faults = faults;
        this.users = users;
        this.devices = devices;
        this.opLogs = opLogs;
        this.opLog = opLog;
    }

    // ---------------- 视图构建（键名对齐 Go 版 workOrderView） ----------------

    private Map<String, Object> view(WorkOrder o, Map<Long, String> names) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", o.id);
        m.put("order_no", o.orderNo);
        m.put("fault_id", o.faultId);
        m.put("device_hw_id", o.deviceHwId);
        m.put("status", o.status);
        m.put("assignee_id", o.assigneeId);
        m.put("assignee_name",
                o.assigneeId == null ? "" : names.getOrDefault(o.assigneeId, ""));
        m.put("result", o.result);
        m.put("created_at", o.createdAt);
        m.put("closed_at", o.closedAt);
        double oh = WorkOrders.overdueHours(o);
        m.put("overdue", oh > 0);
        m.put("overdue_hours", oh);
        return m;
    }

    private Map<Long, String> assigneeNames(List<WorkOrder> rows) {
        Map<Long, String> out = new HashMap<>();
        rows.stream().map(w -> w.assigneeId).filter(java.util.Objects::nonNull).distinct()
                .forEach(id -> users.findById(id).ifPresent(u -> out.put(u.id, u.username)));
        return out;
    }

    /** GET /work-orders：分页 + 设备/状态/处理人/编号/时间范围筛选。 */
    @GetMapping("/work-orders")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String hwId,
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(required = false) String status,
            @RequestParam(name = "assignee_id", required = false) String assigneeId,
            @RequestParam(name = "order_no", required = false) String orderNo,
            @RequestParam(name = "start_time", required = false) String startTime,
            @RequestParam(name = "end_time", required = false) String endTime,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        final String hw = (hwId == null || hwId.isBlank()) ? deviceHwId : hwId;

        Specification<WorkOrder> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (hw != null && !hw.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), hw));
            }
            if (status != null && !status.isBlank()) {
                preds.add(cb.equal(root.get("status"), status));
            }
            if (assigneeId != null && !assigneeId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("assigneeId"), Long.parseLong(assigneeId.trim())));
                } catch (NumberFormatException ignored) {
                    // 与 Go 版一致：解析失败忽略筛选
                }
            }
            if (orderNo != null && !orderNo.isBlank()) {
                preds.add(cb.like(root.get("orderNo"), "%" + orderNo + "%"));
            }
            Instant start = parseDate(startTime);
            if (start != null) {
                preds.add(cb.greaterThanOrEqualTo(root.get("createdAt"), start));
            }
            Instant end = parseDate(endTime);
            if (end != null) {
                preds.add(cb.lessThanOrEqualTo(root.get("createdAt"), end.plus(Duration.ofDays(1))));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = orders.count(spec);
        List<WorkOrder> rows = orders
                .findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .getContent();
        Map<Long, String> names = assigneeNames(rows);

        List<Object> list = new ArrayList<>();
        rows.forEach(o -> list.add(view(o, names)));
        return ApiResponse.ok(Map.of(
                "list", list, "total", total,
                "page", pg.page(), "page_size", pg.pageSize()));
    }

    /** GET /work-orders/{id}：详情 + SLA + 关联故障 + 操作时间线。 */
    @GetMapping("/work-orders/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("工单不存在");
        }
        WorkOrder o = opt.get();
        double overdueHours = WorkOrders.overdueHours(o);

        Map<String, Object> sla = new LinkedHashMap<>();
        sla.put("overdue", overdueHours > 0);
        sla.put("overdue_hours", overdueHours);
        switch (o.status) {
            case "pending" -> {
                sla.put("deadline_hours", 24L);
                sla.put("stage", "待处理");
            }
            case "processing" -> {
                sla.put("deadline_hours", 48L);
                sla.put("stage", "处理中");
            }
            case "completed" -> sla.put("stage", "已完成");
            case "rejected" -> sla.put("stage", "已驳回");
            default -> { }
        }

        Map<String, Object> faultView = new LinkedHashMap<>();
        if (o.faultId != null && o.faultId > 0) {
            faults.findById(o.faultId).ifPresent(f -> {
                Map<String, Object> dev = new LinkedHashMap<>();
                devices.findByHwId(f.deviceHwId).ifPresent(d -> {
                    dev.put("id", d.id);
                    dev.put("hw_id", d.hwId);
                    dev.put("intersection", d.intersection);
                    dev.put("lat", d.lat);
                    dev.put("lng", d.lng);
                    dev.put("online_status", d.onlineStatus);
                });
                faultView.put("id", f.id);
                faultView.put("device_hw_id", f.deviceHwId);
                faultView.put("err_code", f.errCode);
                faultView.put("fault_type", f.faultType);
                faultView.put("fault_level", f.faultLevel);
                faultView.put("status", f.status);
                faultView.put("first_seen", f.firstSeen);
                faultView.put("last_seen", f.lastSeen);
                faultView.put("device", dev);
            });
        }

        String assigneeName = o.assigneeId == null ? "" : users.findById(o.assigneeId)
                .map(u -> u.username).orElse("");

        List<Map<String, Object>> timeline = new ArrayList<>();
        for (OperationLog t : opLogs.findByTargetOrderByCreatedAtDesc(
                "work-order/" + o.id, PageRequest.of(0, 50))) {
            Map<String, Object> tm = new LinkedHashMap<>();
            tm.put("id", t.id);
            tm.put("user_id", t.userId);
            tm.put("username", t.username);
            tm.put("op_type", t.opType);
            tm.put("target", t.target);
            tm.put("detail", t.detail);
            tm.put("created_at", t.createdAt);
            timeline.add(tm);
        }

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("work_order", view(o, assigneeNames(List.of(o))));
        data.put("sla", sla);
        data.put("fault", faultView);
        data.put("assignee", assigneeName);
        data.put("timeline", timeline);
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 手动建单请求体（对齐 Go 版 CreateWorkOrder req）。 */
    public record CreateRequest(Long faultId, String deviceHwId, Long assigneeId) {
    }

    /** POST /work-orders（workorder:create）：手动创建 pending 工单并关联故障。 */
    @PostMapping("/work-orders")
    @RequirePerm("workorder:create")
    public ResponseEntity<?> create(@RequestBody CreateRequest req,
                                    HttpServletRequest request) {
        if (req.faultId() == null || req.deviceHwId() == null || req.deviceHwId().isBlank()) {
            return badRequest("参数错误");
        }
        var faultOpt = faults.findById(req.faultId());
        if (faultOpt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        if (req.assigneeId() != null) {
            var check = validateAssignee(req.assigneeId());
            if (check != null) {
                return check;
            }
        }

        WorkOrder wo = new WorkOrder();
        wo.orderNo = WorkOrders.nextOrderNo(orders);
        wo.faultId = req.faultId();
        wo.deviceHwId = req.deviceHwId();
        wo.status = "pending";
        wo.assigneeId = req.assigneeId();
        wo.faultActiveScope = req.faultId(); // 活跃工单占据 fault 唯一位（M1）
        try {
            orders.saveAndFlush(wo);
        } catch (Exception e) {
            return serverError();
        }
        // 回填故障工单 ID
        faults.findById(req.faultId()).ifPresent(f -> {
            f.workOrderId = wo.id;
            faults.save(f);
        });

        opLog.record(request, OpTypes.CREATE, "work-order/" + wo.id, "创建维修工单");
        return ok(Map.of("work_order", rawView(wo), "message", "工单创建成功"));
    }

    /** 状态流转请求体。 */
    public record StatusRequest(String status, String result) {
    }

    /**
     * PUT /work-orders/{id}/status（workorder:update）：
     * pending→processing→completed/rejected；completed 联动解决故障；
     * rejected 释放活跃位；rejected→pending 重新激活。
     */
    @PutMapping("/work-orders/{id}/status")
    @RequirePerm("workorder:update")
    public ResponseEntity<?> updateStatus(@PathVariable Long id,
                                          @RequestBody StatusRequest req,
                                          HttpServletRequest request) {
        if (req.status() == null || req.status().isBlank()) {
            return badRequest("参数错误");
        }
        List<String> valid = List.of("pending", "processing", "completed", "rejected");
        if (!valid.contains(req.status())) {
            return badRequest("无效的工单状态");
        }
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("工单不存在");
        }
        WorkOrder wo = opt.get();

        Map<String, Object> updates = new HashMap<>();
        updates.put("status", req.status());
        if (req.result() != null && !req.result().isEmpty()) {
            updates.put("result", req.result());
        }
        // 默认：活跃态占据 fault 位
        if (wo.faultId != null && wo.faultId > 0) {
            updates.put("fault_active_scope", wo.faultId);
        }

        if ("completed".equals(req.status())) {
            Instant now = Instant.now();
            updates.put("closed_at", now);
            updates.put("fault_active_scope", null); // 完结释放活跃位
            faults.findById(wo.faultId).ifPresent(f -> {
                f.status = "resolved";
                f.lastSeen = now;
                f.resolvedAt = now;
                faults.save(f);
            });
        }
        if ("rejected".equals(req.status())) {
            updates.put("fault_active_scope", null); // 驳回释放活跃位
        }
        // 驳回后重新激活：清空关闭时间、重新占据 fault 活跃位、故障回到已确认
        if ("pending".equals(req.status()) && "rejected".equals(wo.status)) {
            updates.put("closed_at", null);
            if (wo.faultId != null && wo.faultId > 0) {
                updates.put("fault_active_scope", wo.faultId);
                faults.findById(wo.faultId).ifPresent(f -> {
                    f.status = "confirmed";
                    faults.save(f);
                });
            }
        }

        applyUpdates(wo, updates);
        orders.save(wo);
        opLog.record(request, OpTypes.UPDATE, "work-order/" + wo.id,
                "更新工单状态为 " + req.status());
        return ok(Map.of("work_order", rawView(wo), "message", "工单状态更新成功"));
    }

    /** 派单请求体。 */
    public record AssignRequest(Long assigneeId) {
    }

    /** PUT /work-orders/{id}/assign（workorder:assign）：指派/更换处理人，pending→processing。 */
    @PutMapping("/work-orders/{id}/assign")
    @RequirePerm("workorder:assign")
    public ResponseEntity<?> assign(@PathVariable Long id, @RequestBody AssignRequest req,
                                    HttpServletRequest request) {
        if (req.assigneeId() == null || req.assigneeId() <= 0) {
            return badRequest("请选择维修人员");
        }
        ResponseEntity<?> err = validateAssignee(req.assigneeId());
        if (err != null) {
            return err;
        }
        User assignee = users.findById(req.assigneeId()).orElseThrow();

        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("工单不存在");
        }
        WorkOrder wo = opt.get();

        wo.assigneeId = req.assigneeId();
        if ("pending".equals(wo.status)) {
            wo.status = "processing";
            faults.findById(wo.faultId).ifPresent(f -> {
                f.status = "dispatched";
                f.repairerId = req.assigneeId();
                f.dispatchedAt = Instant.now();
                faults.save(f);
            });
        }
        orders.save(wo);
        opLog.record(request, OpTypes.DISPATCH, "work-order/" + wo.id,
                "派单给用户" + assignee.username);
        return ok(Map.of("work_order", rawView(wo),
                "message", "派单成功（已指派给 " + assignee.username + "）"));
    }

    /** DELETE /work-orders/{id}（workorder:delete）：解除故障关联后删除。 */
    @DeleteMapping("/work-orders/{id}")
    @RequirePerm("workorder:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = orders.findById(id);
        if (opt.isEmpty()) {
            return notFound("工单不存在");
        }
        WorkOrder wo = opt.get();
        if (wo.faultId != null && wo.faultId > 0) {
            faults.findById(wo.faultId).ifPresent(f -> {
                f.workOrderId = null; // 解除关联（保留故障记录）
                faults.save(f);
            });
        }
        orders.delete(wo);
        opLog.record(request, OpTypes.DELETE, "work-order/" + wo.id, "删除工单" + wo.orderNo);
        return ok(Map.of("message", "工单已删除"));
    }

    // ------------------------------------------------------------------

    /** 不含姓名 enrich 的原始视图（create/update 返回用）。 */
    private Map<String, Object> rawView(WorkOrder o) {
        return view(o, Map.of());
    }

    /** 校验处理人存在且为运维/管理员；返回错误响应或 null。 */
    private ResponseEntity<?> validateAssignee(Long assigneeId) {
        var u = users.findById(assigneeId);
        if (u.isEmpty()) {
            return notFound("处理人不存在");
        }
        String role = u.get().role;
        if (!"admin".equals(role) && !"operator".equals(role)) {
            return badRequest("只能指派给运维人员或管理员");
        }
        return null;
    }

    private void applyUpdates(WorkOrder wo, Map<String, Object> updates) {
        if (updates.containsKey("status")) {
            wo.status = (String) updates.get("status");
        }
        if (updates.containsKey("result")) {
            wo.result = (String) updates.get("result");
        }
        if (updates.containsKey("closed_at")) {
            wo.closedAt = (Instant) updates.get("closed_at");
        }
        if (updates.containsKey("fault_active_scope")) {
            wo.faultActiveScope = (Long) updates.get("fault_active_scope");
        }
        if (updates.containsKey("assignee_id")) {
            wo.assigneeId = (Long) updates.get("assignee_id");
        }
    }

    private static Instant parseDate(String s) {
        if (s == null || s.isBlank()) {
            return null;
        }
        try {
            return LocalDate.parse(s.trim(),
                    DateTimeFormatter2.YMD).atStartOfDay(ZoneId.systemDefault()).toInstant();
        } catch (Exception e) {
            return null;
        }
    }

    /** yyyy-MM-dd 解析器（对齐 Go 版 time.Parse("2006-01-02")）。 */
    private static final class DateTimeFormatter2 {
        static final java.time.format.DateTimeFormatter YMD =
                java.time.format.DateTimeFormatter.ofPattern("yyyy-MM-dd");
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> serverError() {
        return ResponseEntity.internalServerError()
                .body(ApiResponse.fail("internal_error", "服务器内部错误"));
    }

    private ResponseEntity<?> ok(Map<String, Object> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

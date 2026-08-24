// 预警接口：列表/详情/忽略/批量忽略/转工单/自动忽略/导出。契约对齐 Go 版 handler/warning.go。
package com.tsloms.server.warning;

import com.tsloms.server.model.Warning;
import com.tsloms.server.model.WarningConsts;
import com.tsloms.server.repository.WarningRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class WarningController {

    private static final DateTimeFormatter TS =
            DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss").withZone(ZoneId.systemDefault());

    private final WarningRepository warnings;
    private final WorkOrderRepository workOrders;
    private final OperationLogService opLog;

    public WarningController(WarningRepository warnings, WorkOrderRepository workOrders,
                             OperationLogService opLog) {
        this.warnings = warnings;
        this.workOrders = workOrders;
        this.opLog = opLog;
    }

    static Map<String, Object> view(Warning w) {
        Map<String, Object> v = new LinkedHashMap<>();
        v.put("id", w.id);
        v.put("device_hw_id", w.deviceHwId);
        v.put("crossing_id", w.crossingId);
        v.put("equipment_uuid", w.equipmentUuid);
        v.put("warning_code", w.warningCode);
        v.put("warning_label", w.warningLabel);
        v.put("level", w.level);
        v.put("func", w.func);
        v.put("source", w.source);
        v.put("deal_state", w.dealState);
        v.put("status", w.status);
        v.put("fault_id", w.faultId);
        v.put("work_order_id", w.workOrderId);
        v.put("ignore_reason", w.ignoreReason);
        v.put("occurred_at", w.occurredAt);
        v.put("resolved_at", w.resolvedAt);
        v.put("remark", w.remark);
        v.put("created_at", w.createdAt);
        return v;
    }

    /** GET /warnings：多条件筛选分页（设备/路口/等级/来源/处置状态/转移状态）。 */
    @GetMapping("/warnings")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(name = "crossing_id", required = false) String crossingId,
            @RequestParam(required = false) String level,
            @RequestParam(required = false) String source,
            @RequestParam(name = "deal_state", required = false) String dealState,
            @RequestParam(required = false) String status,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        Specification<Warning> spec = specOf(deviceHwId, crossingId, level, source,
                dealState, status);
        long total = warnings.count(spec);
        List<Object> rows = new ArrayList<>();
        warnings.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "occurredAt")))
                .forEach(w -> rows.add(view(w)));
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    private Specification<Warning> specOf(String deviceHwId, String crossingId,
                                          String level, String source,
                                          String dealState, String status) {
        return (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (deviceHwId != null && !deviceHwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), deviceHwId));
            }
            if (crossingId != null && !crossingId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("crossingId"), Long.parseLong(crossingId)));
                } catch (NumberFormatException ignored) {
                    // 与 Go 一致忽略非法值
                }
            }
            if (level != null && !level.isBlank()) {
                preds.add(cb.equal(root.get("level"), level));
            }
            if (source != null && !source.isBlank()) {
                preds.add(cb.equal(root.get("source"), source));
            }
            if (dealState != null && !dealState.isBlank()) {
                preds.add(cb.equal(root.get("dealState"), dealState));
            }
            if (status != null && !status.isBlank()) {
                preds.add(cb.equal(root.get("status"), status));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };
    }

    /** GET /warnings/{id}。 */
    @GetMapping("/warnings/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = warnings.findById(id);
        if (opt.isEmpty()) {
            return notFound("预警不存在");
        }
        return ResponseEntity.ok(ApiResponse.ok(view(opt.get())));
    }

    /** 忽略请求体。 */
    public record IgnoreRequest(String reason) {
    }

    /** POST /warnings/{id}/ignore（warning:manage）：标记忽略。 */
    @PostMapping("/warnings/{id}/ignore")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> ignore(@PathVariable Long id, @RequestBody IgnoreRequest req,
                                    HttpServletRequest request) {
        var opt = warnings.findById(id);
        if (opt.isEmpty()) {
            return notFound("预警不存在");
        }
        Warning w = opt.get();
        w.dealState = WarningConsts.DEAL_IGNORED;
        w.ignoreReason = req == null || req.reason() == null ? "" : req.reason();
        w.resolvedAt = Instant.now();
        warnings.save(w);
        opLog.record(request, com.tsloms.server.model.OpTypes.UPDATE, "warning/" + id,
                "忽略预警 " + w.warningLabel);
        return ok(Map.of("message", "预警已忽略"));
    }

    /** 批量忽略请求体。 */
    public record BatchIgnoreRequest(List<Long> ids, String reason) {
    }

    /** POST /warnings/batch-ignore（warning:manage）。 */
    @PostMapping("/warnings/batch-ignore")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> batchIgnore(@RequestBody BatchIgnoreRequest req,
                                         HttpServletRequest request) {
        if (req.ids() == null || req.ids().isEmpty()) {
            return badRequest("请选择要忽略的预警");
        }
        int n = 0;
        for (Long id : req.ids()) {
            var opt = warnings.findById(id);
            if (opt.isPresent()) {
                Warning w = opt.get();
                w.dealState = WarningConsts.DEAL_IGNORED;
                w.ignoreReason = nz(req.reason());
                w.resolvedAt = Instant.now();
                warnings.save(w);
                n++;
            }
        }
        opLog.record(request, com.tsloms.server.model.OpTypes.UPDATE, "warning/batch",
                "批量忽略预警 " + n + " 条");
        return ok(Map.of("message", "已忽略 " + n + " 条预警"));
    }

    /** POST /warnings/{id}/to-workorder（warning:manage）：转工单。 */
    @PostMapping("/warnings/{id}/to-workorder")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> toWorkOrder(@PathVariable Long id, HttpServletRequest request) {
        var opt = warnings.findById(id);
        if (opt.isEmpty()) {
            return notFound("预警不存在");
        }
        Warning w = opt.get();
        if (WarningConsts.TRANSFERRED.equals(w.status)) {
            return badRequest("该预警已转工单");
        }
        // 独立预警建单：无故障关联的 pending 工单（fault_id 置空、不占活跃位）
        com.tsloms.server.model.WorkOrder wo = new com.tsloms.server.model.WorkOrder();
        wo.orderNo = com.tsloms.server.inventory.BizNo.next(
                workOrders::countByOrderNoStartingWith, "WO");
        wo.deviceHwId = w.deviceHwId;
        wo.status = "pending";
        try {
            workOrders.saveAndFlush(wo);
        } catch (Exception e) {
            return serverError();
        }
        w.status = WarningConsts.TRANSFERRED;
        w.workOrderId = wo.id;
        w.dealState = WarningConsts.DEAL_RESOLVED; // 已进入工单流转，视为已处置
        w.resolvedAt = Instant.now();
        warnings.save(w);

        opLog.record(request, com.tsloms.server.model.OpTypes.DISPATCH,
                "work-order/" + wo.id, "预警转工单 " + wo.orderNo);
        return ok(Map.of("work_order_id", wo.id, "order_no", wo.orderNo,
                "message", "已转工单"));
    }

    /** 自动忽略请求体（复用规则匹配语义，直接按筛选条件批量）。 */
    public record AutoIgnoreRequest(List<Long> ids) {
    }

    /** POST /warnings/auto-ignore（warning:manage）：按传入 ID 批量静默处理。 */
    @PostMapping("/warnings/auto-ignore")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> autoIgnore(@RequestBody AutoIgnoreRequest req,
                                        HttpServletRequest request) {
        int n = 0;
        if (req != null && req.ids() != null) {
            for (Long id : req.ids()) {
                var opt = warnings.findById(id);
                if (opt.isPresent()
                        && WarningConsts.DEAL_UNHANDLED.equals(opt.get().dealState)) {
                    Warning w = opt.get();
                    w.dealState = WarningConsts.DEAL_IGNORED;
                    w.ignoreReason = "自动忽略";
                    w.resolvedAt = Instant.now();
                    warnings.save(w);
                    n++;
                }
            }
        }
        return ok(Map.of("ignored", n));
    }

    /** GET /warnings/export（warning:manage）：CSV 导出（UTF-8 BOM）。 */
    @GetMapping("/warnings/export")
    @RequirePerm("warning:manage")
    public void export(HttpServletResponse response) throws IOException {
        List<Warning> rows = warnings.findAll(
                Sort.by(Sort.Direction.DESC, "occurredAt"));
        response.setContentType("text/csv;charset=UTF-8");
        response.setHeader("Content-Disposition", "attachment; filename=warnings.csv");
        OutputStream os = response.getOutputStream();
        os.write(new byte[]{(byte) 0xEF, (byte) 0xBB, (byte) 0xBF});
        StringBuilder sb = new StringBuilder();
        sb.append("ID,设备硬件ID,告警码,告警文案,等级,来源,处置状态,转移状态,发生时间,解决时间\n");
        for (Warning w : rows) {
            sb.append(w.id).append(',')
                    .append(w.deviceHwId).append(',')
                    .append(w.warningCode).append(',')
                    .append(nz(w.warningLabel)).append(',')
                    .append(w.level).append(',')
                    .append(w.source).append(',')
                    .append(w.dealState).append(',')
                    .append(w.status).append(',')
                    .append(TS.format(w.occurredAt)).append(',')
                    .append(w.resolvedAt == null ? "" : TS.format(w.resolvedAt))
                    .append('\n');
        }
        os.write(sb.toString().getBytes(StandardCharsets.UTF_8));
        os.flush();
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

    private ResponseEntity<?> serverError() {
        return ResponseEntity.internalServerError()
                .body(ApiResponse.fail("internal_error", "服务器内部错误"));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

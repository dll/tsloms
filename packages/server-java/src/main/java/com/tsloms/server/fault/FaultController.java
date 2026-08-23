// 故障接口：列表/详情/更新状态机/派单/删除/CSV 导出，契约对齐 Go 版 handler/fault.go。
// （ReviewFault 复核与多源证据端点属识别引擎范围，随 AI 模块阶段迁移。）
package com.tsloms.server.fault;

import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.User;
import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.UserRepository;
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
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
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
public class FaultController {

    private static final DateTimeFormatter TS = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");

    /** 故障类型中文映射（导出用，对齐 Go 版 faultTypeCN）。 */
    private static final Map<String, String> FAULT_TYPE_CN = Map.of(
            "lamp_off", "灯灭", "abnormal_on", "异常同亮", "timeout", "亮灯超时",
            "dim", "缺亮", "power_loss", "断电", "unknown", "未知");

    /** 旧 active 兼容语义 = 未解决三态。 */
    static final List<String> ACTIVE_STATUSES = List.of("occurred", "confirmed", "dispatched");

    private final FaultRecordRepository faults;
    private final WorkOrderRepository orders;
    private final UserRepository users;
    private final DeviceRepository devices;
    private final OperationLogService opLog;

    public FaultController(FaultRecordRepository faults, WorkOrderRepository orders,
                           UserRepository users, DeviceRepository devices,
                           OperationLogService opLog) {
        this.faults = faults;
        this.orders = orders;
        this.users = users;
        this.devices = devices;
        this.opLog = opLog;
    }

    // ---------------- 列表与详情 ----------------

    /** GET /faults：分页 + 设备/状态(active兼容)/类型/等级/研判/时间范围筛选。 */
    @GetMapping("/faults")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String hwId,
            @RequestParam(required = false) String status,
            @RequestParam(name = "fault_type", required = false) String faultType,
            @RequestParam(name = "fault_level", required = false) String faultLevel,
            @RequestParam(name = "recognition_status", required = false) String recognitionStatus,
            @RequestParam(name = "start_time", required = false) String startTime,
            @RequestParam(name = "start_date", required = false) String startDate,
            @RequestParam(name = "end_time", required = false) String endTime,
            @RequestParam(name = "end_date", required = false) String endDate,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<FaultRecord> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (hwId != null && !hwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), hwId));
            }
            if (status != null && !status.isBlank()) {
                if ("active".equals(status)) {
                    preds.add(root.get("status").in(ACTIVE_STATUSES));
                } else {
                    preds.add(cb.equal(root.get("status"), status));
                }
            }
            if (faultType != null && !faultType.isBlank()) {
                preds.add(cb.equal(root.get("faultType"), faultType));
            }
            if (faultLevel != null && !faultLevel.isBlank()) {
                preds.add(cb.equal(root.get("faultLevel"), faultLevel));
            }
            if (recognitionStatus != null && !recognitionStatus.isBlank()) {
                if ("active".equals(recognitionStatus)) {
                    preds.add(root.get("status").in(ACTIVE_STATUSES));
                } else {
                    preds.add(cb.equal(root.get("recognitionStatus"), recognitionStatus));
                }
            }
            Instant start = parseDate(firstNonBlank(startTime, startDate));
            if (start != null) {
                preds.add(cb.greaterThanOrEqualTo(root.get("firstSeen"), start));
            }
            Instant end = parseDate(firstNonBlank(endTime, endDate));
            if (end != null) {
                preds.add(cb.lessThanOrEqualTo(root.get("lastSeen"),
                        end.plus(Duration.ofDays(1))));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = faults.count(spec);
        List<FaultRecord> rows = faults.findAll(spec,
                        PageRequest.of(pg.page() - 1, pg.pageSize(),
                                Sort.by(Sort.Direction.DESC, "lastSeen")))
                .getContent();

        Map<Long, String> names = FaultViews.userNames(users, rows);
        List<Object> list = new ArrayList<>();
        rows.forEach(f -> list.add(FaultViews.view(users, f, names)));

        Map<String, Object> data = new HashMap<>();
        data.put("list", list);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** GET /faults/{id}：故障 + 关联设备 + 关联工单摘要。 */
    @GetMapping("/faults/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = faults.findById(id);
        if (opt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        FaultRecord f = opt.get();

        Map<String, Object> dev = new LinkedHashMap<>();
        devices.findByHwId(f.deviceHwId).ifPresent(d -> {
            dev.put("id", d.id);
            dev.put("hw_id", d.hwId);
            dev.put("intersection", d.intersection);
            dev.put("lat", d.lat);
            dev.put("lng", d.lng);
            dev.put("online_status", d.onlineStatus);
            dev.put("sw_version", d.swVersion);
            dev.put("conf_version", d.confVersion);
            dev.put("last_checkin_at", d.lastCheckinAt);
            dev.put("created_at", d.createdAt);
        });

        Map<String, Object> wo = new LinkedHashMap<>();
        orders.findFirstByFaultIdOrderByCreatedAtDesc(f.id)
                .ifPresent(o -> wo.putAll(Map.of(
                        "id", o.id, "order_no", o.orderNo, "fault_id", o.faultId,
                        "device_hw_id", o.deviceHwId, "status", o.status,
                        "assignee_id", o.assigneeId == null ? Map.of() : o.assigneeId,
                        "result", o.result == null ? "" : o.result,
                        "created_at", o.createdAt,
                        "closed_at", o.closedAt == null ? "" : o.closedAt)));

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("fault", FaultViews.view(users, f, FaultViews.userNames(users, List.of(f))));
        data.put("device", dev);
        data.put("work_order", wo);
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 更新请求体（对齐 Go 版 UpdateFault req）。 */
    public record UpdateFaultRequest(String status, Long ownerId, Long repairerId) {
    }

    /** PUT /faults/{id}（fault:update）：状态流转 + 负责人/维修人变更 + 完成联动关单。 */
    @PutMapping("/faults/{id}")
    @RequirePerm("fault:update")
    public ResponseEntity<?> update(@PathVariable Long id,
                                    @RequestBody UpdateFaultRequest req,
                                    HttpServletRequest request) {
        var opt = faults.findById(id);
        if (opt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        FaultRecord f = opt.get();

        if (req.status() != null && !req.status().isBlank()
                && !List.of("occurred", "confirmed", "dispatched", "resolved")
                        .contains(req.status())) {
            return badRequest("无效的故障状态");
        }

        Instant now = Instant.now();
        boolean changed = false;

        if (req.status() != null && !req.status().isBlank()) {
            f.status = req.status();
            switch (req.status()) {
                case "confirmed" -> f.confirmedAt = now;
                case "dispatched" -> f.dispatchedAt = now;
                case "resolved" -> {
                    f.resolvedAt = now;
                    f.lastSeen = now;
                }
                default -> { }
            }
            changed = true;
        }
        if (req.ownerId() != null) {
            if (req.ownerId() == 0) {
                f.ownerId = null;
            } else {
                if (users.findById(req.ownerId()).isEmpty()) {
                    return notFound("负责人不存在");
                }
                f.ownerId = req.ownerId();
                f.confirmedAt = now;
                // 设置了负责人且未指定状态、当前为“发生”时 → 自动推进到已确认
                if ((req.status() == null || req.status().isBlank())
                        && "occurred".equals(f.status)) {
                    f.status = "confirmed";
                }
            }
            changed = true;
        }
        if (req.repairerId() != null) {
            if (req.repairerId() == 0) {
                f.repairerId = null;
            } else {
                if (users.findById(req.repairerId()).isEmpty()) {
                    return notFound("维修人不存在");
                }
                f.repairerId = req.repairerId();
            }
            changed = true;
        }

        if (!changed) {
            return badRequest("无可更新字段");
        }
        faults.save(f);

        // 解决故障 → 同步完结关联未完成工单（释放活跃位）
        if ("resolved".equals(req.status())) {
            orders.findAll().stream()
                    .filter(w -> id.equals(w.faultId))
                    .filter(w -> "pending".equals(w.status) || "processing".equals(w.status))
                    .forEach(w -> {
                        w.status = "completed";
                        w.closedAt = now;
                        w.faultActiveScope = null;
                        orders.save(w);
                    });
        }

        faults.flush();
        FaultRecord latest = faults.findById(id).orElseThrow();
        opLog.record(request, OpTypes.UPDATE, "fault/" + id, "更新故障为 " + latest.status);
        return ok(Map.of(
                "fault", FaultViews.view(users, latest, FaultViews.userNames(users, List.of(latest))),
                "message", "故障更新成功"));
    }

    /** 派单请求体。 */
    public record DispatchRequest(Long assigneeId) {
    }

    /** POST /faults/{id}/dispatch（fault:dispatch）：建/复用处理中工单并指派。 */
    @PostMapping("/faults/{id}/dispatch")
    @RequirePerm("fault:dispatch")
    public ResponseEntity<?> dispatch(@PathVariable Long id,
                                      @RequestBody DispatchRequest req,
                                      HttpServletRequest request) {
        if (req.assigneeId() == null || req.assigneeId() <= 0) {
            return badRequest("请选择维修人员");
        }
        var faultOpt = faults.findById(id);
        if (faultOpt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        FaultRecord f = faultOpt.get();

        User assignee = users.findById(req.assigneeId()).orElse(null);
        if (assignee == null) {
            return notFound("维修人员不存在");
        }
        if (!"admin".equals(assignee.role) && !"operator".equals(assignee.role)) {
            return badRequest("只能指派给运维人员或管理员");
        }

        Instant now = Instant.now();
        // 已有未完成工单则复用，否则新建 processing 工单
        WorkOrder wo = orders.findAll().stream()
                .filter(w -> id.equals(w.faultId))
                .filter(w -> "pending".equals(w.status) || "processing".equals(w.status))
                .findFirst()
                .orElseGet(() -> {
                    WorkOrder nw = new WorkOrder();
                    nw.orderNo = com.tsloms.server.workorder.WorkOrders.nextOrderNo(orders);
                    nw.faultId = id;
                    nw.deviceHwId = f.deviceHwId;
                    nw.status = "processing";
                    nw.assigneeId = req.assigneeId();
                    nw.faultActiveScope = id; // 活跃工单占据 fault 唯一位（M1）
                    return orders.save(nw);
                });
        if (!req.assigneeId().equals(wo.assigneeId) || !"processing".equals(wo.status)) {
            wo.assigneeId = req.assigneeId();
            wo.status = "processing";
            orders.save(wo);
        }

        // 故障推进到已派单
        f.status = "dispatched";
        f.workOrderId = wo.id;
        f.repairerId = req.assigneeId();
        f.dispatchedAt = now;
        faults.save(f);

        FaultRecord latest = faults.findById(id).orElseThrow();
        WorkOrder latestWo = orders.findById(wo.id).orElseThrow();
        opLog.record(request, OpTypes.DISPATCH, "work-order/" + wo.id,
                "从故障派发工单给 " + assignee.username);
        return ok(Map.of(
                "fault", FaultViews.view(users, latest, FaultViews.userNames(users, List.of(latest))),
                "work_order", FaultViews.workOrderView(users, latestWo),
                "message", "派单成功"));
    }

    /** DELETE /faults/{id}（fault:delete）：有未完成工单拒绝；删除后解除历史工单引用。 */
    @DeleteMapping("/faults/{id}")
    @RequirePerm("fault:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = faults.findById(id);
        if (opt.isEmpty()) {
            return notFound("故障记录不存在");
        }
        long openOrders = orders.findAll().stream()
                .filter(w -> id.equals(w.faultId))
                .filter(w -> "pending".equals(w.status) || "processing".equals(w.status))
                .count();
        if (openOrders > 0) {
            return badRequest("该故障存在未完成工单，请先完结或删除关联工单后再删除故障");
        }
        faults.delete(opt.get());
        orders.findAll().stream()
                .filter(w -> id.equals(w.faultId))
                .forEach(w -> {
                    w.faultId = null;
                    orders.save(w);
                });
        opLog.record(request, OpTypes.DELETE, "fault/" + id, "删除故障记录");
        return ok(Map.of("message", "故障已删除", "id", id));
    }

    /** GET /faults/export：CSV 导出（UTF-8 BOM，最多 5000 条）。 */
    @GetMapping("/faults/export")
    public void export(HttpServletResponse response,
                       @RequestParam(required = false) String hwId,
                       @RequestParam(required = false) String status,
                       @RequestParam(name = "fault_type", required = false) String faultType,
                       @RequestParam(name = "fault_level", required = false) String faultLevel,
                       @RequestParam(name = "start_date", required = false) String startDate,
                       @RequestParam(name = "end_date", required = false) String endDate) throws IOException {
        Specification<FaultRecord> spec = buildExportSpec(hwId, status, faultType,
                faultLevel, startDate, endDate);
        List<FaultRecord> rows = faults.findAll(spec,
                PageRequest.of(0, 5000, Sort.by(Sort.Direction.DESC, "lastSeen")))
                .getContent();

        response.setContentType("text/csv;charset=UTF-8");
        String fname = "faults_" + LocalDate.now()
                .format(DateTimeFormatter.ofPattern("yyyyMMdd")) + ".csv";
        response.setHeader("Content-Disposition", "attachment; filename=" + fname);

        OutputStream os = response.getOutputStream();
        os.write(new byte[]{(byte) 0xEF, (byte) 0xBB, (byte) 0xBF}); // UTF-8 BOM
        StringBuilder sb = new StringBuilder();
        sb.append("ID,设备硬件ID,错误码,故障类型,等级,灯态,红灯电流,黄灯电流,绿灯电流,首次,末次,状态,研判,置信度,工单ID\n");
        for (FaultRecord f : rows) {
            ZoneId zone = ZoneId.systemDefault();
            sb.append(f.id == null ? "" : f.id).append(',')
                    .append(f.deviceHwId).append(',')
                    .append(f.errCode == null ? "" : f.errCode).append(',')
                    .append(FAULT_TYPE_CN.getOrDefault(f.faultType, f.faultType)).append(',')
                    .append(f.faultLevel == null ? "" : f.faultLevel).append(',')
                    .append(f.ledState == null ? "" : f.ledState).append(',')
                    .append(f.currentR == null ? 0 : f.currentR).append(',')
                    .append(f.currentY == null ? 0 : f.currentY).append(',')
                    .append(f.currentG == null ? 0 : f.currentG).append(',')
                    .append(TS.format(f.firstSeen.atZone(zone))).append(',')
                    .append(TS.format(f.lastSeen.atZone(zone))).append(',')
                    .append(f.status).append(',')
                    .append(f.recognitionStatus == null ? "" : f.recognitionStatus).append(',')
                    .append(f.confidence == null ? "" : String.format("%.2f", f.confidence)).append(',')
                    .append(f.workOrderId == null ? "" : f.workOrderId).append('\n');
        }
        os.write(sb.toString().getBytes(StandardCharsets.UTF_8));
        os.flush();
    }

    private Specification<FaultRecord> buildExportSpec(String hwId, String status,
                                                       String faultType, String faultLevel,
                                                       String startDate, String endDate) {
        return (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (hwId != null && !hwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), hwId));
            }
            if (status != null && !status.isBlank()) {
                if ("active".equals(status)) {
                    preds.add(root.get("status").in(ACTIVE_STATUSES));
                } else {
                    preds.add(cb.equal(root.get("status"), status));
                }
            }
            if (faultType != null && !faultType.isBlank()) {
                preds.add(cb.equal(root.get("faultType"), faultType));
            }
            if (faultLevel != null && !faultLevel.isBlank()) {
                preds.add(cb.equal(root.get("faultLevel"), faultLevel));
            }
            Instant start = parseDate(startDate);
            if (start != null) {
                preds.add(cb.greaterThanOrEqualTo(root.get("firstSeen"), start));
            }
            Instant end = parseDate(endDate);
            if (end != null) {
                preds.add(cb.lessThanOrEqualTo(root.get("lastSeen"),
                        end.plus(Duration.ofDays(1))));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };
    }

    // ------------------------------------------------------------------

    private static String firstNonBlank(String a, String b) {
        if (a != null && !a.isBlank()) {
            return a;
        }
        return b;
    }

    private static Instant parseDate(String s) {
        if (s == null || s.isBlank()) {
            return null;
        }
        try {
            return LocalDate.parse(s.trim(), DateTimeFormatter.ofPattern("yyyy-MM-dd"))
                    .atStartOfDay(ZoneId.systemDefault()).toInstant();
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

    private ResponseEntity<?> ok(Map<String, Object> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

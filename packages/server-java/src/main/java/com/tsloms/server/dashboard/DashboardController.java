// 仪表盘接口：契约对齐 Go 版 handler/dashboard.go（登录即可，无权限点要求）。
// AI 总览端点（/dashboard/ai-overview）待 AI 模块迁移后补齐。
package com.tsloms.server.dashboard;

import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.web.ApiResponse;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.time.temporal.WeekFields;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/dashboard")
public class DashboardController {

    /** 工单 SLA：pending 24h / processing 48h。 */
    private static final Duration PENDING_SLA = Duration.ofHours(24);
    private static final Duration PROCESSING_SLA = Duration.ofHours(48);

    private final DeviceRepository devices;
    private final FaultRecordRepository faults;
    private final WorkOrderRepository orders;

    public DashboardController(DeviceRepository devices, FaultRecordRepository faults,
                               WorkOrderRepository orders) {
        this.devices = devices;
        this.faults = faults;
        this.orders = orders;
    }

    /** GET /dashboard/overview：看板总览关键指标。 */
    @GetMapping("/overview")
    public ApiResponse<Map<String, Object>> overview() {
        long online = devices.countByOnlineStatus(true);
        long offline = devices.countByOnlineStatus(false);

        long activeFaults = faults.countByStatusIn(List.of("occurred", "confirmed", "dispatched"));
        long resolvedFaults = faults.countByStatus("resolved");
        Instant todayStart = LocalDate.now().atStartOfDay(ZoneId.systemDefault()).toInstant();
        long todayFaults = faults.countByFirstSeenGreaterThanEqual(todayStart);

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("devices", Map.of(
                "online", online, "offline", offline, "total", online + offline));
        data.put("faults", Map.of(
                "active", activeFaults, "resolved", resolvedFaults, "today", todayFaults));
        data.put("work_orders", Map.of(
                "pending", orders.countByStatus("pending"),
                "processing", orders.countByStatus("processing"),
                "completed", orders.countByStatus("completed"),
                "overdue", overdueCount()));
        return ApiResponse.ok(data);
    }

    /** GET /dashboard/fault-type-stats?days=30：故障类型占比（饼图）。 */
    @GetMapping("/fault-type-stats")
    public ApiResponse<Map<String, Object>> faultTypeStats(
            @RequestParam(name = "days", defaultValue = "30") int days) {
        int d = days > 0 ? days : 30;
        List<Map<String, Object>> stats = new ArrayList<>();
        faults.countByFaultTypeSince(since(d)).forEach(r -> stats.add(
                Map.of("fault_type", r.getK(), "count", r.getCnt())));
        return ApiResponse.ok(Map.of("stats", stats, "days", d));
    }

    /** GET /dashboard/work-order-stats：工单状态分布 + 超时数（饼图）。 */
    @GetMapping("/work-order-stats")
    public ApiResponse<Map<String, Object>> workOrderStats() {
        List<Map<String, Object>> stats = List.of("pending", "processing",
                "completed", "rejected").stream()
                .map(s -> Map.<String, Object>of("status", s,
                        "count", orders.countByStatus(s)))
                .toList();
        return ApiResponse.ok(Map.of("stats", stats, "overdue", overdueCount()));
    }

    /**
     * GET /dashboard/fault-trend?dimension=day&days=7：故障趋势（柱状图）。
     * Go 版在应用层分组保证 MySQL/SQLite 兼容——Java 同样在内存分组。
     */
    @GetMapping("/fault-trend")
    public ApiResponse<Map<String, Object>> faultTrend(
            @RequestParam(name = "dimension", defaultValue = "day") String dimension,
            @RequestParam(name = "days", defaultValue = "7") int days) {
        int d = days > 0 ? days : 7;
        String dim = switch (dimension) {
            case "week", "month" -> dimension;
            default -> "day";
        };
        DateTimeFormatter dayFmt = DateTimeFormatter.ofPattern("yyyy-MM-dd");
        DateTimeFormatter monthFmt = DateTimeFormatter.ofPattern("yyyy-MM");
        ZoneId zone = ZoneId.systemDefault();

        Map<String, Long> counts = new LinkedHashMap<>();
        for (FaultRecord f : faults.findByFirstSeenGreaterThanEqual(since(d))) {
            LocalDate date = f.firstSeen.atZone(zone).toLocalDate();
            String period = switch (dim) {
                case "week" -> date.get(WeekFields.of(Locale.CHINA).weekBasedYear()) + "-W"
                        + String.format("%02d", date.get(WeekFields.of(Locale.CHINA)
                                .weekOfWeekBasedYear()));
                case "month" -> date.format(monthFmt);
                default -> date.format(dayFmt);
            };
            counts.merge(period, 1L, Long::sum);
        }
        List<Map<String, Object>> trend = counts.entrySet().stream()
                .sorted(Map.Entry.comparingByKey())
                .map(e -> Map.<String, Object>of("period", e.getKey(), "count", e.getValue()))
                .toList();
        return ApiResponse.ok(Map.of("trend", trend, "dimension", dim, "days", d));
    }

    /** GET /dashboard/device-fault-rank?limit=10&days=30：设备故障排行 Top N。 */
    @GetMapping("/device-fault-rank")
    public ApiResponse<Map<String, Object>> deviceFaultRank(
            @RequestParam(name = "limit", defaultValue = "10") int limit,
            @RequestParam(name = "days", defaultValue = "30") int days) {
        int l = limit > 0 ? limit : 10;
        int d = days > 0 ? days : 30;
        List<Map<String, Object>> rank = new ArrayList<>();
        faults.countByDeviceSince(since(d), PageRequest.of(0, l))
                .forEach(r -> rank.add(Map.of("device_hw_id", r.getK(), "count", r.getCnt())));
        return ApiResponse.ok(Map.of("rank", rank, "days", d, "limit", l));
    }

    /** GET /dashboard/work-order-avg-closure?days=30：工单平均闭环时长（小时）。 */
    @GetMapping("/work-order-avg-closure")
    public ApiResponse<Map<String, Object>> workOrderAvgClosure(
            @RequestParam(name = "days", defaultValue = "30") int days) {
        int d = days > 0 ? days : 30;
        var rows = orders.findByStatusAndClosedAtNotNullAndCreatedAtGreaterThanEqual(
                "completed", since(d));
        double totalHours = 0;
        for (var w : rows) {
            totalHours += Duration.between(w.createdAt, w.closedAt).toMillis() / 3_600_000.0;
        }
        double avgHours = rows.isEmpty() ? 0 : totalHours / rows.size();
        return ApiResponse.ok(Map.of(
                "avg_hours", avgHours,
                "completed_count", rows.size(),
                "total_hours", totalHours,
                "days", d));
    }

    // ------------------------------------------------------------------

    private Instant since(int days) {
        return Instant.now().minus(Duration.ofDays(days));
    }

    private long overdueCount() {
        Instant now = Instant.now();
        return orders.countOverdue(now.minus(PENDING_SLA), now.minus(PROCESSING_SLA));
    }
}

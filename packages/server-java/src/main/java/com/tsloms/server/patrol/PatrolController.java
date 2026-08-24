// 巡检接口：任务 CRUD/执行/记录列表/排行/自检上报。契约对齐 Go 版 patrol 路由组。
package com.tsloms.server.patrol;

import com.tsloms.server.model.Device;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.model.PatrolModes;
import com.tsloms.server.model.PatrolRecord;
import com.tsloms.server.model.PatrolStatuses;
import com.tsloms.server.model.PatrolTask;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.PatrolRecordRepository;
import com.tsloms.server.repository.PatrolTaskRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Duration;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ThreadLocalRandom;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.transaction.annotation.Transactional;
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
@RequestMapping("/api/v1/patrol")
public class PatrolController {

    /** 设备离线判定阈值：30 分钟无签到视为离线（与 Go 版 offline_check 同口径）。 */
    private static final Duration OFFLINE_AFTER = Duration.ofMinutes(30);

    private final PatrolTaskRepository tasks;
    private final PatrolRecordRepository records;
    private final DeviceRepository devices;
    private final OperationLogService opLog;

    public PatrolController(PatrolTaskRepository tasks, PatrolRecordRepository records,
                            DeviceRepository devices, OperationLogService opLog) {
        this.tasks = tasks;
        this.records = records;
        this.devices = devices;
        this.opLog = opLog;
    }

    // ---------------- 视图 ----------------

    static Map<String, Object> taskView(PatrolTask t) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", t.id);
        m.put("name", t.name);
        m.put("mode", t.mode);
        m.put("area_id", t.areaId);
        m.put("street_id", t.streetId);
        m.put("time_window", t.timeWindow);
        m.put("target_count", t.targetCount);
        m.put("status", t.status);
        m.put("assignee_id", t.assigneeId);
        m.put("created_by", t.createdBy);
        m.put("last_run_at", t.lastRunAt);
        m.put("run_count", t.runCount);
        m.put("remark", t.remark);
        m.put("created_at", t.createdAt);
        return m;
    }

    static Map<String, Object> recordView(PatrolRecord r) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", r.id);
        m.put("task_id", r.taskId);
        m.put("device_id", r.deviceId);
        m.put("device_hw_id", r.deviceHwId);
        m.put("crossing_id", r.crossingId);
        m.put("patrol_type", r.patrolType);
        m.put("check_result", r.checkResult);
        m.put("check_detail", r.checkDetail);
        m.put("selfcheck_result", r.selfCheckResult);
        m.put("evidences", r.evidences);
        m.put("patrol_by", r.patrolBy);
        m.put("patrol_at", r.patrolAt);
        m.put("lat", r.lat);
        m.put("lng", r.lng);
        m.put("created_at", r.createdAt);
        return m;
    }

    // ---------------- 任务 CRUD ----------------

    /** GET /patrol/tasks：分页。 */
    @GetMapping("/tasks")
    public ResponseEntity<?> listTasks(HttpServletRequest request) {

        Pagination.Page pg = Pagination.of(request);
        long total = tasks.count();
        List<Object> rows = new ArrayList<>();
        tasks.findAll(PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .forEach(t -> rows.add(taskView(t)));
        return ok(taskData(rows, total, pg));
    }

    private Map<String, Object> taskData(List<?> rows, long total, Pagination.Page pg) {
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return data;
    }

    /** 任务请求体。 */
    public record TaskRequest(String name, String mode, Long areaId, Long streetId,
                              String timeWindow, Integer targetCount, Long assigneeId,
                              String remark) {
    }

    /** POST /patrol/tasks（patrol:manage）。 */
    @PostMapping("/tasks")
    @RequirePerm("patrol:manage")
    public ResponseEntity<?> create(@RequestBody TaskRequest req, HttpServletRequest request) {
        if (req.name() == null || req.name().isBlank() || req.mode() == null) {
            return badRequest("参数错误（name、mode 必填）");
        }
        PatrolTask t = new PatrolTask();
        apply(t, req);
        Long uid = AuthInterceptor.userId(request);
        t.createdBy = uid == null ? 0L : uid;
        tasks.save(t);
        opLog.record(request, OpTypes.CREATE, "patrol-task/" + t.id, "创建巡检任务 " + t.name);
        return ok(Map.of("message", "巡检任务已创建"));
    }

    /** GET /patrol/tasks/{id}。 */
    @GetMapping("/tasks/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = tasks.findById(id);
        if (opt.isEmpty()) {
            return notFound("巡检任务不存在");
        }
        return ok(taskView(opt.get()));
    }

    /** PUT /patrol/tasks/{id}（patrol:manage）。 */
    @PutMapping("/tasks/{id}")
    @RequirePerm("patrol:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody TaskRequest req,
                                    HttpServletRequest request) {
        var opt = tasks.findById(id);
        if (opt.isEmpty()) {
            return notFound("巡检任务不存在");
        }
        apply(opt.get(), req);
        tasks.save(opt.get());
        opLog.record(request, OpTypes.UPDATE, "patrol-task/" + id, "更新巡检任务");
        return ok(Map.of("message", "巡检任务已更新"));
    }

    private void apply(PatrolTask t, TaskRequest req) {
        t.name = nz(req.name());
        t.mode = nz(req.mode());
        t.areaId = req.areaId();
        t.streetId = req.streetId();
        t.timeWindow = nz(req.timeWindow());
        t.targetCount = req.targetCount() == null ? 0 : req.targetCount();
        t.assigneeId = req.assigneeId();
        t.remark = nz(req.remark());
    }

    /** DELETE /patrol/tasks/{id}（patrol:manage）。 */
    @DeleteMapping("/tasks/{id}")
    @RequirePerm("patrol:manage")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = tasks.findById(id);
        if (opt.isEmpty()) {
            return notFound("巡检任务不存在");
        }
        tasks.delete(opt.get());
        opLog.record(request, OpTypes.DELETE, "patrol-task/" + id, "删除巡检任务");
        return ok(Map.of("message", "删除成功"));
    }

    /**
     * POST /patrol/tasks/{id}/run（patrol:run）：执行一次巡检。
     * 按模式圈选设备 → 逐台生成记录（30 分钟内有签到=normal，否则 abnormal）→
     * 更新 run_count/last_run_at/status。
     */
    @PostMapping("/tasks/{id}/run")
    @RequirePerm("patrol:run")
    @Transactional
    public ResponseEntity<?> run(@PathVariable Long id, HttpServletRequest request) {
        var taskOpt = tasks.findById(id);
        if (taskOpt.isEmpty()) {
            return notFound("巡检任务不存在");
        }
        PatrolTask t = taskOpt.get();

        List<Device> picked = pickDevices(t);
        String operator = operatorOf(request);
        Instant now = Instant.now();
        int abnormal = 0;

        for (Device d : picked) {
            boolean onlineRecent = d.lastCheckinAt != null
                    && d.lastCheckinAt.until(now, ChronoUnit.MINUTES) <= 30
                    && d.onlineStatus;
            PatrolRecord r = new PatrolRecord();
            r.taskId = t.id;
            r.deviceId = d.id;
            r.deviceHwId = d.hwId;
            r.crossingId = d.crossingId;
            r.patrolType = t.mode;
            r.checkResult = onlineRecent
                    ? com.tsloms.server.model.PatrolResults.NORMAL
                    : com.tsloms.server.model.PatrolResults.ABNORMAL;
            r.checkDetail = onlineRecent
                    ? "设备在线且 30 分钟内有签到"
                    : "设备离线或超过 30 分钟未签到";
            r.selfCheckResult = "{\"online\":" + d.onlineStatus
                    + ",\"sw_version\":" + d.swVersion + "}";
            r.patrolBy = operator;
            r.patrolAt = now;
            if (!onlineRecent) {
                abnormal++;
            }
            records.save(r);
        }

        t.runCount += 1;
        t.lastRunAt = now;
        t.status = PatrolStatuses.DONE;
        tasks.save(t);

        opLog.record(request, OpTypes.CREATE, "patrol-task/" + t.id,
                "执行巡检 " + t.name + "，覆盖 " + picked.size() + " 台设备");
        return ok(Map.of(
                "message", "巡检完成",
                "checked", picked.size(),
                "abnormal", abnormal));
    }

    /** 圈选设备：random 取 N 台；其余模式全量（area/street 过滤待区划挂接完善）。 */
    private List<Device> pickDevices(PatrolTask t) {
        List<Device> all = devices.findAll(Sort.by(Sort.Direction.ASC, "id")).stream()
                .filter(d -> !"retired".equals(d.lifecycleStatus))
                .toList();
        if (PatrolModes.RANDOM.equals(t.mode) && t.targetCount > 0
                && t.targetCount < all.size()) {
            List<Device> shuffled = new ArrayList<>(all);
            java.util.Collections.shuffle(shuffled, ThreadLocalRandom.current());
            return shuffled.subList(0, t.targetCount);
        }
        return all;
    }

    // ---------------- 记录与排行 ----------------

    /** GET /patrol/records：分页筛选（task/device/type/result）。 */
    @GetMapping("/records")
    public ResponseEntity<?> records(
            @RequestParam(name = "task_id", required = false) String taskId,
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(name = "patrol_type", required = false) String patrolType,
            @RequestParam(name = "check_result", required = false) String checkResult,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<PatrolRecord> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (taskId != null && !taskId.isBlank()) {
                try {
                    preds.add(cb.equal(root.get("taskId"), Long.parseLong(taskId)));
                } catch (NumberFormatException ignored) {
                    // 与 Go 一致忽略非法值
                }
            }
            if (deviceHwId != null && !deviceHwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), deviceHwId));
            }
            if (patrolType != null && !patrolType.isBlank()) {
                preds.add(cb.equal(root.get("patrolType"), patrolType));
            }
            if (checkResult != null && !checkResult.isBlank()) {
                preds.add(cb.equal(root.get("checkResult"), checkResult));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = records.count(spec);
        List<Object> rows = new ArrayList<>();
        records.findAll(spec, PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "patrolAt")))
                .forEach(r -> rows.add(recordView(r)));
        return ok(recordData(rows, total, pg));
    }

    private Map<String, Object> recordData(List<?> rows, long total, Pagination.Page pg) {
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return data;
    }

    /**
     * GET /patrol/ranking?group_by=device|patrol_by&days=：
     * 巡检次数/异常数/异常率/最近巡检时间聚合（对齐 buildRankingItem）。
     */
    @GetMapping("/ranking")
    public ApiResponse<Map<String, Object>> ranking(
            @RequestParam(name = "group_by", defaultValue = "device") String groupBy,
            @RequestParam(name = "days", defaultValue = "30") int days) {
        int d = days > 0 ? days : 30;
        Instant since = Instant.now().minus(Duration.ofDays(d));
        boolean byDevice = !"patrol_by".equals(groupBy);

        Map<String, List<PatrolRecord>> grouped = new LinkedHashMap<>();
        for (PatrolRecord r : records.findAll()) {
            if (r.patrolAt.isBefore(since)) {
                continue;
            }
            String key = byDevice ? r.deviceHwId : r.patrolBy;
            grouped.computeIfAbsent(key, k -> new ArrayList<>()).add(r);
        }

        List<Map<String, Object>> list = new ArrayList<>();
        for (var entry : grouped.entrySet()) {
            List<PatrolRecord> rs = entry.getValue();
            int totalCnt = rs.size();
            int abnormalCnt = (int) rs.stream()
                    .filter(r -> com.tsloms.server.model.PatrolResults.ABNORMAL
                            .equals(r.checkResult)).count();
            Instant last = rs.stream().map(r -> r.patrolAt)
                    .max(Comparator.naturalOrder()).orElse(null);
            Map<String, Object> item = new LinkedHashMap<>();
            item.put("key", entry.getKey());
            item.put("patrol_count", totalCnt);
            item.put("abnormal_count", abnormalCnt);
            item.put("abnormal_rate",
                    Math.round(abnormalCnt * 10000.0 / totalCnt) / 10000.0);
            item.put("last_patrol_at", last);
            list.add(item);
        }
        list.sort((a, b) -> Integer.compare((int) b.get("patrol_count"),
                (int) a.get("patrol_count")));
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** 自检上报请求体。 */
    public record SelfCheckRequest(String deviceHwId, String result, String detail,
                                   Double lat, Double lng) {
    }

    /** POST /patrol/selfcheck（patrol:selfcheck）：信号灯自检结果落记录。 */
    @PostMapping("/selfcheck")
    @RequirePerm("patrol:selfcheck")
    public ResponseEntity<?> selfCheck(@RequestBody SelfCheckRequest req,
                                       HttpServletRequest request) {
        if (req.deviceHwId() == null || req.deviceHwId().isBlank()) {
            return badRequest("参数错误（device_hw_id 必填）");
        }
        String result = com.tsloms.server.model.PatrolResults.ABNORMAL.equals(req.result())
                ? com.tsloms.server.model.PatrolResults.ABNORMAL
                : com.tsloms.server.model.PatrolResults.NORMAL;

        PatrolRecord r = new PatrolRecord();
        r.deviceHwId = req.deviceHwId();
        devices.findByHwId(req.deviceHwId()).ifPresent(d -> {
            r.deviceId = d.id;
            r.crossingId = d.crossingId;
        });
        r.patrolType = PatrolModes.SELFCHECK;
        r.checkResult = result;
        r.checkDetail = nz(req.detail());
        r.selfCheckResult = "{\"result\":\"" + result + "\"}";
        r.patrolBy = operatorOf(request);
        r.patrolAt = Instant.now();
        r.lat = req.lat();
        r.lng = req.lng();
        records.save(r);
        opLog.record(request, OpTypes.CREATE, "patrol-selfcheck/" + r.id,
                "信号灯自检：" + result);
        return ok(Map.of("message", "自检已上报", "result", result));
    }

    // ------------------------------------------------------------------

    static String nz(String s) {
        return s == null ? "" : s;
    }

    static String operatorOf(HttpServletRequest request) {
        Object u = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        String name = u == null ? "" : String.valueOf(u);
        return name.isEmpty() ? "system" : name;
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

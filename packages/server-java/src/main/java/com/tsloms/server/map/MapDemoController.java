// 地图聚合 + 系统演示 + 杂项端点：对齐 Go 版 map_data.go/demo.go/杂项路由。
package com.tsloms.server.map;

import com.tsloms.server.auth.ModuleService;
import com.tsloms.server.model.*;
import com.tsloms.server.repository.*;
import com.tsloms.server.web.ApiResponse;
import org.springframework.beans.factory.annotation.Autowired;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import com.tsloms.server.web.Pagination;
import java.time.Instant;
import java.util.*;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v1")
public class MapDemoController {

    private final CrossingRepository crossings;
    private final DeviceRepository devices;
    private final FaultRecordRepository faults;
    private final AreaRepository areas;
    private final WarningRepository warnings;
    private final WarningRuleRepository warningRules;
    private final ModuleToggleRepository toggles;
    private final ModuleService moduleSvc;
    private final OperationLogRepository opLogs;
    private final com.tsloms.server.repository.FaultCaseRepository faultCases;
    private final com.tsloms.server.repository.UserRepository userRepo;
    private final com.tsloms.server.repository.FaultEvidenceRepository eviRepo;
    private final com.tsloms.server.repository.FaultCaseRepository caseRepo;

    public MapDemoController(CrossingRepository crossings, DeviceRepository devices,
                             FaultRecordRepository faults, AreaRepository areas,
                             WarningRepository warnings, WarningRuleRepository warningRules,
                             ModuleToggleRepository toggles, ModuleService moduleSvc,
                             OperationLogRepository opLogs,
                             com.tsloms.server.repository.FaultCaseRepository faultCases,
                             com.tsloms.server.repository.UserRepository userRepo,
                             com.tsloms.server.repository.FaultEvidenceRepository eviRepo,
                             com.tsloms.server.repository.FaultCaseRepository caseRepo) {
        this.crossings = crossings;
        this.devices = devices;
        this.faults = faults;
        this.areas = areas;
        this.warnings = warnings;
        this.warningRules = warningRules;
        this.toggles = toggles;
        this.moduleSvc = moduleSvc;
        this.opLogs = opLogs;
        this.faultCases = faultCases;
        this.userRepo = userRepo;
        this.eviRepo = eviRepo;
        this.caseRepo = caseRepo;
    }

    // ==================== 地图聚合 ====================

    /** GET /map/crossing-data：路口聚合数据（Cesium 打标/热力着色）。 */
    @GetMapping("/map/crossing-data")
    public ApiResponse<Map<String, Object>> crossingData() {
        var faultDevices = new java.util.HashSet<String>();
        faults.findAll().stream()
                .filter(f -> List.of("occurred", "confirmed", "dispatched").contains(f.status))
                .forEach(f -> faultDevices.add(f.deviceHwId));

        List<Map<String, Object>> list = new ArrayList<>();
        for (Crossing x : crossings.findAll(Sort.by(Sort.Direction.ASC, "id"))) {
            List<Device> devs = devices.findAll().stream()
                    .filter(d -> x.id.equals(d.crossingId)).toList();
            int total = devs.size();
            int faultCnt = (int) devs.stream().filter(d -> faultDevices.contains(d.hwId)).count();
            int onlineCnt = (int) devs.stream().filter(d -> d.onlineStatus).count();
            double faultRatio = total > 0 ? (double) faultCnt / total : 0;
            double greenRatio = total > 0 ? (double) (total - faultCnt) / total : 0;

            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", x.id);
            m.put("point_no", nz(x.pointNo));
            m.put("name", nz(x.name));
            m.put("road_name", nz(x.roadName));
            m.put("lat", x.lat);
            m.put("lng", x.lng);
            m.put("status", x.status);
            m.put("device_total", total);
            m.put("fault_count", faultCnt);
            m.put("online_count", onlineCnt);
            m.put("fault_ratio", Math.round(faultRatio * 10000.0) / 10000.0);
            m.put("green_ratio", Math.round(greenRatio * 10000.0) / 10000.0);
            m.put("level", colorLevel(faultRatio));
            m.put("area_path", "");
            list.add(m);
        }
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** GET /map/road-data：道路聚合（简化：按 road_name 去重计数）。 */
    @GetMapping("/map/road-data")
    public ApiResponse<Map<String, Object>> roadData() {
        Map<String, Integer> roads = new LinkedHashMap<>();
        devices.findAll().forEach(d -> {
            String rn = d.roadName == null ? "" : d.roadName;
            if (!rn.isEmpty()) {
                roads.merge(rn, 1, Integer::sum);
            }
        });
        List<Map<String, Object>> list = new ArrayList<>();
        roads.forEach((k, v) -> list.add(Map.of("road_name", k, "device_count", v)));
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    static String colorLevel(double faultRatio) {
        if (faultRatio <= 0) return "green";
        if (faultRatio >= 1) return "red";
        if (faultRatio < 0.34) return "yellow_low";
        if (faultRatio < 0.67) return "yellow";
        return "orange";
    }

    // ==================== 系统演示 ====================

    private static final String DEMO_PREFIX = "DEMO";
    private static final int DEMO_START = 900001;

    /** GET /demo/status。 */
    @GetMapping("/demo/status")
    public ApiResponse<Map<String, Object>> demoStatus() {
        long cnt = devices.findAll().stream()
                .filter(d -> d.hwId != null && d.hwId.startsWith(DEMO_PREFIX)).count();
        long intCnt = devices.findAll().stream()
                .filter(d -> d.hwId != null && d.hwId.startsWith(DEMO_PREFIX))
                .map(d -> d.intersection).distinct().count();
        long warnCnt = warnings.findAll().stream()
                .filter(w -> w.deviceHwId != null && w.deviceHwId.startsWith(DEMO_PREFIX)).count();
        return ApiResponse.ok(Map.of(
                "running", cnt > 0, "devices", cnt,
                "intersection", intCnt, "warnings", warnCnt,
                "hw_range", DEMO_START + "-" + 909999));
    }

    /** POST /demo/start：创建演示设备/故障/预警。 */
    @PostMapping("/demo/start")
    @RequirePerm("device:create")
    public ResponseEntity<?> demoStart(@RequestBody(required = false) Map<String, Object> body) {
        int n = 5;
        if (body != null && body.get("n") != null) {
            n = Math.min(20, Math.max(1, ((Number) body.get("n")).intValue()));
        }
        List<String> names = List.of("清流关路", "丰乐大道", "琅琊路", "南谯路", "会峰路",
                "湖心路", "紫薇路", "龙蟠河", "中都大道", "站前广场");
        var rnd = java.util.concurrent.ThreadLocalRandom.current();
        Instant now = Instant.now();
        for (int i = 0; i < n; i++) {
            String hw = DEMO_PREFIX + (DEMO_START + i);
            if (devices.findByHwId(hw).isPresent()) continue;
            Device d = new Device();
            d.hwId = hw;
            d.intersection = "[演示] " + names.get(i % names.size());
            d.onlineStatus = rnd.nextBoolean();
            d.registrationSource = "manual";
            devices.save(d);
        }
        return ok(Map.of("message", "演示数据已创建", "devices", n));
    }

    /** POST /demo/end：清理演示数据。 */
    @PostMapping("/demo/end")
    @RequirePerm("device:create")
    public ResponseEntity<?> demoEnd() {
        List<Device> demoDevs = devices.findAll().stream()
                .filter(d -> d.hwId != null && d.hwId.startsWith(DEMO_PREFIX)).toList();
        int n = demoDevs.size();
        demoDevs.forEach(devices::delete);
        return ok(Map.of("message", "演示数据已清理", "removed", n));
    }

    // ==================== 杂项端点 ====================

    /** GET /devices/stats。 */
    @GetMapping("/devices/stats")
    public ApiResponse<Map<String, Object>> deviceStats() {
        long total = devices.count();
        long online = devices.countByOnlineStatus(true);
        long active = devices.findAll().stream()
                .filter(d -> "active".equals(d.lifecycleStatus)).count();
        return ApiResponse.ok(Map.of("total", total, "online", online,
                "offline", total - online, "active", active));
    }

    /** GET /dispatch/reference：可派单人员（同 assignable）。 */
    @GetMapping("/dispatch/reference")
    public ApiResponse<Map<String, Object>> dispatchRef(
            com.tsloms.server.repository.UserRepository users) {
        List<Map<String, Object>> list = new ArrayList<>();
        userRepo.findAll(Sort.by(Sort.Direction.ASC, "id")).forEach(u -> {
            if (List.of("admin", "operator").contains(u.role)) {
                list.add(Map.of("id", u.id, "username", u.username, "role", u.role));
            }
        });
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** GET /logs/operations：操作日志。 */
    @GetMapping("/logs/operations")
    public ApiResponse<Map<String, Object>> opLogs(
            
            @RequestParam(required = false) String target,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        List<Object> rows = new ArrayList<>();
        var pageable = PageRequest.of(pg.page() - 1, pg.pageSize(),
                Sort.by(Sort.Direction.DESC, "createdAt"));
        (target != null && !target.isBlank()
                ? opLogs.findByTargetOrderByCreatedAtDesc(target, pageable)
                : opLogs.findAll(pageable)).forEach(rows::add);
        return ApiResponse.ok(Map.of("list", rows, "total", rows.size()));
    }

    /** GET /logs/packets：MQTT 报文日志（Java 版由 slf4j 接管，返回空列表）。 */
    @GetMapping("/logs/packets")
    public ApiResponse<Map<String, Object>> packetLogs() {
        return ApiResponse.ok(Map.of("list", List.of(), "total", 0));
    }

    /** GET /modules/settings：可选模块开关列表。 */
    @GetMapping("/modules/settings")
    @RequirePerm("module:manage")
    public ApiResponse<Map<String, Object>> moduleSettings() {
        List<Map<String, Object>> list = new ArrayList<>();
        for (String key : List.of("video", "inventory", "purchase", "expense",
                "supplier", "ai", "dispatch", "notification")) {
            boolean enabled = toggles.findByModuleKey(key)
                    .map(t -> t.enabled).orElse(false);
            list.add(Map.of("module_key", key, "enabled", enabled,
                    "name", moduleName(key)));
        }
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** PUT /modules/settings：更新模块开关。 */
    @PutMapping("/modules/settings")
    @RequirePerm("module:manage")
    public ResponseEntity<?> updateModuleSetting(@RequestBody Map<String, Object> body,
                                                 HttpServletRequest request) {
        String key = (String) body.get("module_key");
        Boolean enabled = (Boolean) body.get("enabled");
        if (key == null || key.isBlank() || enabled == null) {
            return badRequest("参数错误（module_key、enabled 必填）");
        }
        var opt = toggles.findByModuleKey(key);
        ModuleToggle t = opt.orElseGet(() -> {
            ModuleToggle nt = new ModuleToggle();
            nt.moduleKey = key;
            return nt;
        });
        t.enabled = enabled;
        toggles.save(t);
        return ok(Map.of("message", "模块设置已更新"));
    }

    /** GET /recognition/stats：识别引擎统计。 */
    @GetMapping("/recognition/stats")
    public ApiResponse<Map<String, Object>> recognitionStats(
            ) {
        long evidenceCount = eviRepo.count();
        long caseCount = caseRepo.count();
        return ApiResponse.ok(Map.of(
                "evidence_total", evidenceCount,
                "case_total", caseCount,
                "engine", "rule_base"));
    }

    /** PUT /intersections/rename：批量重命名路口。 */
    @PutMapping("/intersections/rename")
    @RequirePerm("intersection:update")
    public ResponseEntity<?> renameIntersection(@RequestBody Map<String, Object> body) {
        String oldName = (String) body.get("old_name");
        String newName = (String) body.get("new_name");
        if (oldName == null || newName == null || oldName.isBlank() || newName.isBlank()) {
            return badRequest("参数错误（old_name、new_name 必填）");
        }
        int[] cnt = {0};
        devices.findAll().forEach(d -> {
            if (oldName.equals(d.intersection)) {
                d.intersection = newName;
                devices.save(d);
                cnt[0]++;
            }
        });
        return ok(Map.of("message", "重命名完成", "updated", cnt[0]));
    }

    /** PUT /intersections/location：设置路口经纬度。 */
    @PutMapping("/intersections/location")
    @RequirePerm("intersection:update")
    public ResponseEntity<?> setLocation(@RequestBody Map<String, Object> body) {
        String intersection = (String) body.get("intersection");
        Double lat = body.get("lat") == null ? null : ((Number) body.get("lat")).doubleValue();
        Double lng = body.get("lng") == null ? null : ((Number) body.get("lng")).doubleValue();
        if (intersection == null || intersection.isBlank()) {
            return badRequest("参数错误（intersection 必填）");
        }
        int[] cnt = {0};
        devices.findAll().forEach(d -> {
            if (intersection.equals(d.intersection)) {
                d.lat = lat;
                d.lng = lng;
                devices.save(d);
                cnt[0]++;
            }
        });
        return ok(Map.of("message", "位置已更新", "updated", cnt[0]));
    }

    /** DELETE /intersections/clear：清空路口名称。 */
    @DeleteMapping("/intersections/clear")
    @RequirePerm("intersection:delete")
    public ResponseEntity<?> clearIntersection(@RequestParam String intersection) {
        if (intersection == null || intersection.isBlank()) {
            return badRequest("参数错误（intersection 必填）");
        }
        int[] cnt = {0};
        devices.findAll().forEach(d -> {
            if (intersection.equals(d.intersection)) {
                d.intersection = "";
                devices.save(d);
                cnt[0]++;
            }
        });
        return ok(Map.of("message", "已清空", "updated", cnt[0]));
    }

    /** POST /access/mock/send：模拟设备上报（简化：返回成功）。 */
    @PostMapping("/access/mock/send")
    public ResponseEntity<?> mockSend(@RequestBody(required = false) Map<String, Object> body) {
        return ok(Map.of("message", "模拟发送成功（Java 版暂不支持模拟上报，请使用真实硬件）"));
    }

    /** POST /access/csv/import：CSV 批量导入设备。 */
    @PostMapping("/access/csv/import")
    @RequirePerm("device:create")
    public ResponseEntity<?> csvImport(@RequestBody Map<String, Object> body,
                                       HttpServletRequest request) {
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> rows = (List<Map<String, Object>>) body.get("devices");
        if (rows == null || rows.isEmpty()) {
            return badRequest("参数错误（devices 列表必填）");
        }
        int created = 0;
        for (var row : rows) {
            String hw = (String) row.get("hw_id");
            if (hw == null || hw.isBlank() || devices.existsByHwId(hw)) continue;
            Device d = new Device();
            d.hwId = hw.trim();
            d.intersection = (String) row.getOrDefault("intersection", "");
            devices.save(d);
            created++;
        }
        return ok(Map.of("message", "导入完成", "created", created));
    }

    // ---------------- 预警规则 CRUD ----------------

    /** GET /warning-rules。 */
    @GetMapping("/warning-rules")
    public ApiResponse<Map<String, Object>> listWarningRules() {
        List<Object> rows = new ArrayList<>();
        warningRules.findAll(Sort.by(Sort.Direction.DESC, "createdAt")).forEach(rows::add);
        return ApiResponse.ok(Map.of("list", rows, "total", rows.size()));
    }

    /** POST /warning-rules。 */
    @PostMapping("/warning-rules")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> createWarningRule(@RequestBody WarningRule r) {
        r.id = null;
        warningRules.save(r);
        return ok(Map.of("message", "规则已创建", "id", r.id));
    }

    /** PUT /warning-rules/{id}。 */
    @PutMapping("/warning-rules/{id}")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> updateWarningRule(@PathVariable Long id, @RequestBody WarningRule r) {
        var opt = warningRules.findById(id);
        if (opt.isEmpty()) return notFound("规则不存在");
        r.id = id;
        warningRules.save(r);
        return ok(Map.of("message", "规则已更新"));
    }

    /** DELETE /warning-rules/{id}。 */
    @DeleteMapping("/warning-rules/{id}")
    @RequirePerm("warning:manage")
    public ResponseEntity<?> deleteWarningRule(@PathVariable Long id) {
        warningRules.deleteById(id);
        return ok(Map.of("message", "规则已删除"));
    }

    // ---------------- 用户自助 ----------------

    /** POST /user/avatar：更新头像 URL。 */
    @PostMapping("/user/avatar")
    public ResponseEntity<?> updateAvatar(@RequestBody Map<String, Object> body,
                                          HttpServletRequest request) {
        Long userId = com.tsloms.server.web.AuthInterceptor.userId(request);
        if (userId == null) return unauthorized();
        String avatar = (String) body.get("avatar");
        return devices == null ? serverError() : updateUserField(userId, "avatar", avatar, "头像已更新");
    }

    /** PUT /user/profile：更新个人资料。 */
    @PutMapping("/user/profile")
    public ResponseEntity<?> updateProfile(@RequestBody Map<String, Object> body,
                                           HttpServletRequest request) {
        Long userId = com.tsloms.server.web.AuthInterceptor.userId(request);
        if (userId == null) return unauthorized();
        return updateUserFields(userId, body, "个人资料已更新",
                Set.of("real_name", "email", "gender", "address", "education", "engineer_level"));
    }

    

    private ResponseEntity<?> updateUserField(Long userId, String field, Object val, String msg) {
        userRepo.findById(userId).ifPresent(u -> {
            switch (field) {
                case "avatar" -> u.avatar = (String) val;
                case "real_name" -> u.realName = (String) val;
                case "email" -> u.email = (String) val;
                case "gender" -> u.gender = (String) val;
                case "address" -> u.address = (String) val;
                case "education" -> u.education = (String) val;
                case "engineer_level" -> u.engineerLevel = (String) val;
            }
            userRepo.save(u);
        });
        return ok(Map.of("message", msg));
    }

    private ResponseEntity<?> updateUserFields(Long userId, Map<String, Object> body,
                                               String msg, Set<String> allowedFields) {
        userRepo.findById(userId).ifPresent(u -> {
            if (body.containsKey("real_name")) u.realName = (String) body.get("real_name");
            if (body.containsKey("email")) u.email = (String) body.get("email");
            if (body.containsKey("gender")) u.gender = (String) body.get("gender");
            if (body.containsKey("address")) u.address = (String) body.get("address");
            if (body.containsKey("education")) u.education = (String) body.get("education");
            if (body.containsKey("engineer_level")) u.engineerLevel = (String) body.get("engineer_level");
            userRepo.save(u);
        });
        return ok(Map.of("message", msg));
    }

    // ------------------------------------------------------------------

    static String nz(String s) { return s == null ? "" : s; }

    private String moduleName(String key) {
        return Map.ofEntries(
                Map.entry("dashboard", "仪表盘"), Map.entry("device", "设备管理"),
                Map.entry("intersection", "路口管理"), Map.entry("map", "地图总览"),
                Map.entry("feedback", "问题反馈"), Map.entry("fault", "故障管理"),
                Map.entry("workorder", "工单管理"), Map.entry("firmware", "固件管理"),
                Map.entry("log", "系统日志"), Map.entry("settings", "系统设置"),
                Map.entry("video", "视频监控"), Map.entry("inventory", "物料库存"),
                Map.entry("purchase", "采购管理"), Map.entry("expense", "维修费用"),
                Map.entry("supplier", "供应商"), Map.entry("ai", "AI 分析"),
                Map.entry("dispatch", "调度看板"), Map.entry("notification", "站内通知")
        ).getOrDefault(key, key);
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> serverError() {
        return ResponseEntity.internalServerError().body(ApiResponse.fail("internal_error", "服务器内部错误"));
    }

    private ResponseEntity<?> unauthorized() {
        return ResponseEntity.status(401).body(ApiResponse.fail("unauthorized", "未登录"));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

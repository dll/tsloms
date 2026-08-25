// 设备接口：列表筛选/详情/新增/更新/报废/恢复/删除。契约对齐 Go 版 handler/device.go。
package com.tsloms.server.device;

import com.tsloms.server.model.Device;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
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
@RequestMapping("/api/v1/devices")
public class DeviceController {

    private final DeviceRepository devices;
    private final OperationLogService opLog;

    public DeviceController(DeviceRepository devices, OperationLogService opLog) {
        this.devices = devices;
        this.opLog = opLog;
    }

    /** GET /devices：分页多条件筛选（路口/在线/接入/生命周期/硬件ID），updated_at 降序。 */
    @GetMapping
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String intersection,
            @RequestParam(name = "online_status", required = false) String onlineStatus,
            @RequestParam(name = "access_status", required = false) String accessStatus,
            @RequestParam(name = "lifecycle_status", required = false) String lifecycleStatus,
            @RequestParam(name = "hw_id", required = false) String hwId,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<Device> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (intersection != null && !intersection.isBlank()) {
                preds.add(cb.like(root.get("intersection"), "%" + intersection + "%"));
            }
            if (onlineStatus != null && !onlineStatus.isBlank()) {
                boolean online = "true".equals(onlineStatus) || "online".equals(onlineStatus);
                preds.add(cb.equal(root.get("onlineStatus"), online));
            }
            if (accessStatus != null && !accessStatus.isBlank()) {
                preds.add(cb.equal(root.get("accessStatus"), accessStatus));
            }
            if (lifecycleStatus != null && !lifecycleStatus.isBlank()) {
                preds.add(cb.equal(root.get("lifecycleStatus"), lifecycleStatus));
            }
            if (hwId != null && !hwId.isBlank()) {
                preds.add(cb.equal(root.get("hwId"), hwId));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = devices.count(spec);
        List<Device> rows = devices.findAll(spec,
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "updatedAt")))
                .getContent();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** GET /devices/{id}。 */
    @GetMapping("/{id:\\d+}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = devices.findById(id);
        if (opt.isEmpty()) {
            return notFound("设备不存在");
        }
        return ResponseEntity.ok(ApiResponse.ok(opt.get()));
    }

    /** 新增/更新请求体（字段对齐 Go 版 CreateDevice/UpdateDevice）。 */
    public record DeviceRequest(String hwId, String intersection, Long crossingId,
                                String roadName, Double lat, Double lng,
                                Integer networkCode, Integer stationCode,
                                String lifecycleStatus, String func, String orientation,
                                String direction, String batch, String remark,
                                String photo, String manualUrl, String manualName,
                                String repairManualUrl, String repairManualName,
                                Instant installedAt) {
    }

    /** POST /devices（手工预登记）。 */
    @PostMapping
    @RequirePerm("device:create")
    public ResponseEntity<?> create(@RequestBody DeviceRequest req, HttpServletRequest request) {
        if (req.hwId() == null || req.hwId().isBlank()) {
            return badRequest("请填写设备硬件ID");
        }
        if (devices.existsByHwId(req.hwId())) {
            return badRequest("设备硬件ID已存在");
        }
        Device d = new Device();
        apply(d, req);
        d.hwId = req.hwId().trim();
        d.registrationSource = "manual";
        devices.save(d);
        opLog.record(request, OpTypes.CREATE, "device/" + d.hwId, "新增设备");
        return ok(Map.of("message", "设备已创建", "id", d.id));
    }

    /** PUT /devices/{id}（device:update）。 */
    @PutMapping("/{id:\\d+}")
    @RequirePerm("device:update")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody DeviceRequest req,
                                    HttpServletRequest request) {
        var opt = devices.findById(id);
        if (opt.isEmpty()) {
            return notFound("设备不存在");
        }
        Device d = opt.get();
        apply(d, req);
        devices.save(d);
        opLog.record(request, OpTypes.UPDATE, "device/" + d.hwId, "更新设备");
        return ok(Map.of("message", "设备更新成功"));
    }

    private void apply(Device d, DeviceRequest req) {
        if (req.intersection() != null) {
            d.intersection = req.intersection();
        }
        if (req.crossingId() != null) {
            d.crossingId = req.crossingId();
        }
        if (req.roadName() != null) {
            d.roadName = req.roadName();
        }
        if (req.lat() != null) {
            d.lat = req.lat();
        }
        if (req.lng() != null) {
            d.lng = req.lng();
        }
        if (req.networkCode() != null) {
            d.networkCode = req.networkCode();
        }
        if (req.stationCode() != null) {
            d.stationCode = req.stationCode();
        }
        if (req.lifecycleStatus() != null && !req.lifecycleStatus().isBlank()) {
            d.lifecycleStatus = req.lifecycleStatus();
        }
        if (req.func() != null) {
            d.func = req.func();
        }
        if (req.orientation() != null) {
            d.orientation = req.orientation();
        }
        if (req.direction() != null) {
            d.direction = req.direction();
        }
        if (req.batch() != null) {
            d.batch = req.batch();
        }
        if (req.remark() != null) {
            d.remark = req.remark();
        }
        if (req.photo() != null) {
            d.photo = req.photo();
        }
        if (req.manualUrl() != null) {
            d.manualUrl = req.manualUrl();
        }
        if (req.manualName() != null) {
            d.manualName = req.manualName();
        }
        if (req.repairManualUrl() != null) {
            d.repairManualUrl = req.repairManualUrl();
        }
        if (req.repairManualName() != null) {
            d.repairManualName = req.repairManualName();
        }
        if (req.installedAt() != null) {
            d.installedAt = req.installedAt();
        }
    }

    /** POST /devices/{id}/retire（device:update）：报废。 */
    @PostMapping("/{id:\\d+}/retire")
    @RequirePerm("device:update")
    public ResponseEntity<?> retire(@PathVariable Long id, HttpServletRequest request) {
        var opt = devices.findById(id);
        if (opt.isEmpty()) {
            return notFound("设备不存在");
        }
        Device d = opt.get();
        d.lifecycleStatus = "retired";
        d.retiredAt = Instant.now();
        d.onlineStatus = false;
        devices.save(d);
        opLog.record(request, OpTypes.UPDATE, "device/" + d.hwId, "报废设备");
        return ok(Map.of("message", "设备已报废"));
    }

    /** POST /devices/{id}/restore（device:update）：恢复。 */
    @PostMapping("/{id:\\d+}/restore")
    @RequirePerm("device:update")
    public ResponseEntity<?> restore(@PathVariable Long id, HttpServletRequest request) {
        var opt = devices.findById(id);
        if (opt.isEmpty()) {
            return notFound("设备不存在");
        }
        Device d = opt.get();
        d.lifecycleStatus = "active";
        d.retiredAt = null;
        d.retiredReason = "";
        devices.save(d);
        opLog.record(request, OpTypes.UPDATE, "device/" + d.hwId, "恢复设备");
        return ok(Map.of("message", "设备已恢复"));
    }

    /** DELETE /devices/{id}（device:delete）：报废状态才可删。 */
    @DeleteMapping("/{id:\\d+}")
    @RequirePerm("device:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) {
        var opt = devices.findById(id);
        if (opt.isEmpty()) {
            return notFound("设备不存在");
        }
        Device d = opt.get();
        if (!"retired".equals(d.lifecycleStatus)) {
            return badRequest("仅报废状态的设备可删除");
        }
        devices.delete(d);
        opLog.record(request, OpTypes.DELETE, "device/" + d.hwId, "删除设备");
        return ok(Map.of("message", "设备已删除"));
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

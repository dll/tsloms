// 路口与区划接口：CRUD/设备列表/区划树。契约对齐 Go 版 handler/crossing.go。
package com.tsloms.server.crossing;

import com.tsloms.server.model.Area;
import com.tsloms.server.model.Crossing;
import com.tsloms.server.repository.AreaRepository;
import com.tsloms.server.repository.CrossingRepository;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.RequirePerm;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.data.domain.Sort;
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
public class CrossingController {

    private final CrossingRepository crossings;
    private final AreaRepository areas;
    private final DeviceRepository devices;

    public CrossingController(CrossingRepository crossings, AreaRepository areas,
                              DeviceRepository devices) {
        this.crossings = crossings;
        this.areas = areas;
        this.devices = devices;
    }

    static Map<String, Object> view(Crossing x) {
        Map<String, Object> v = new LinkedHashMap<>();
        v.put("id", x.id);
        v.put("point_no", x.pointNo);
        v.put("name", x.name);
        v.put("type", x.type);
        v.put("province_id", x.provinceId);
        v.put("city_id", x.cityId);
        v.put("district_id", x.districtId);
        v.put("street_id", x.streetId);
        v.put("community_id", x.communityId);
        v.put("road_id", x.roadId);
        v.put("road_name", x.roadName);
        v.put("lat", x.lat);
        v.put("lng", x.lng);
        v.put("status", x.status);
        v.put("fault_ratio", x.faultRatio);
        v.put("green_ratio", x.greenRatio);
        v.put("remark", x.remark);
        v.put("created_at", x.createdAt);
        v.put("updated_at", x.updatedAt);
        return v;
    }

    /** GET /crossings：keyword（名称/点位号）+ 状态筛选，按 id 升序。 */
    @GetMapping("/crossings")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String keyword,
            @RequestParam(required = false) String status) {
        List<Map<String, Object>> list = new ArrayList<>();
        crossings.findAll(Sort.by(Sort.Direction.ASC, "id")).forEach(x -> {
            if (keyword != null && !keyword.isBlank()
                    && !(x.name != null && x.name.contains(keyword))
                    && !(x.pointNo != null && x.pointNo.contains(keyword))) {
                return;
            }
            if (status != null && !status.isBlank() && !status.equals(x.status)) {
                return;
            }
            list.add(view(x));
        });
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** GET /crossings/{id}。 */
    @GetMapping("/crossings/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = crossings.findById(id);
        if (opt.isEmpty()) {
            return notFound("路口不存在");
        }
        Map<String, Object> data = new LinkedHashMap<>(view(opt.get()));
        // 关联设备摘要
        List<Object> devs = new ArrayList<>();
        devices.findAll().stream().filter(d -> id.equals(d.crossingId))
                .forEach(d -> devs.add(Map.of(
                        "id", d.id, "hw_id", d.hwId,
                        "intersection", d.intersection == null ? "" : d.intersection,
                        "online_status", d.onlineStatus)));
        data.put("devices", devs);
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    /** 路口请求体（对齐 Go 版 crossingRequest）。 */
    public record CrossingRequest(String pointNo, String name, String type,
                                  Long provinceId, Long cityId, Long districtId,
                                  Long streetId, Long communityId, Long roadId,
                                  String roadName, Double lat, Double lng,
                                  String status, String remark) {
    }

    private void apply(Crossing x, CrossingRequest r) {
        if (r.pointNo() != null && !r.pointNo().isBlank()) {
            x.pointNo = r.pointNo();
        }
        if (r.name() != null && !r.name().isBlank()) {
            x.name = r.name();
        }
        x.type = nz(r.type());
        x.provinceId = r.provinceId();
        x.cityId = r.cityId();
        x.districtId = r.districtId();
        x.streetId = r.streetId();
        x.communityId = r.communityId();
        x.roadId = r.roadId();
        x.roadName = nz(r.roadName());
        x.lat = r.lat();
        x.lng = r.lng();
        if (r.status() != null && !r.status().isBlank()) {
            x.status = r.status();
        }
        x.remark = nz(r.remark());
    }

    /** POST /crossings（crossing:manage）。 */
    @PostMapping("/crossings")
    @RequirePerm("crossing:manage")
    public ResponseEntity<?> create(@RequestBody CrossingRequest req) {
        Crossing x = new Crossing();
        apply(x, req);
        crossings.save(x);
        return ok(Map.of("message", "路口已创建", "id", x.id));
    }

    /** PUT /crossings/{id}（crossing:manage）。 */
    @PutMapping("/crossings/{id}")
    @RequirePerm("crossing:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody CrossingRequest req) {
        var opt = crossings.findById(id);
        if (opt.isEmpty()) {
            return notFound("路口不存在");
        }
        apply(opt.get(), req);
        crossings.save(opt.get());
        return ok(Map.of("message", "路口更新成功"));
    }

    /** DELETE /crossings/{id}（crossing:manage）。 */
    @DeleteMapping("/crossings/{id}")
    @RequirePerm("crossing:manage")
    public ResponseEntity<?> delete(@PathVariable Long id) {
        var opt = crossings.findById(id);
        if (opt.isEmpty()) {
            return notFound("路口不存在");
        }
        long bound = devices.findAll().stream()
                .filter(d -> id.equals(d.crossingId)).count();
        if (bound > 0) {
            return badRequest("该路口下仍有设备绑定，请先解绑设备");
        }
        crossings.delete(opt.get());
        return ok(Map.of("message", "删除成功"));
    }

    /** GET /crossings/{id}/devices：路口下设备列表。 */
    @GetMapping("/crossings/{id}/devices")
    public ResponseEntity<?> devicesOf(@PathVariable Long id) {
        List<Object> devs = new ArrayList<>();
        devices.findAll().stream().filter(d -> id.equals(d.crossingId))
                .forEach(devs::add);
        return ResponseEntity.ok(ApiResponse.ok(Map.of("list", devs)));
    }

    // ---------------- 区划树 ----------------

    private Map<String, Object> areaNode(Area a) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", a.id);
        m.put("code", a.code);
        m.put("name", a.name);
        m.put("type", a.areaType);
        m.put("parent_id", a.parentId);
        m.put("full_name", a.fullName);
        m.put("area_sort", a.areaSort);
        List<Map<String, Object>> children = new ArrayList<>();
        areas.findByParentIdOrderByIdAsc(a.id).forEach(c -> children.add(areaNode(c)));
        m.put("children", children);
        return m;
    }

    /** GET /areas/tree?filter_type=：区划树（可按层级过滤根类型以下内容）。 */
    @GetMapping("/areas/tree")
    public ApiResponse<Map<String, Object>> tree(
            @RequestParam(name = "filter_type", required = false) String filterType) {
        List<Area> roots = filterType == null || filterType.isBlank()
                ? areas.findByParentIdIsNullOrderByAreaSortAscIdAsc()
                : areas.findByAreaTypeOrderByAreaSortAscIdAsc(filterType);
        List<Map<String, Object>> nodes = new ArrayList<>();
        roots.forEach(a -> nodes.add(areaNode(a)));
        int count = countNodes(nodes);
        return ApiResponse.ok(Map.of("tree", nodes, "total", count));
    }

    private int countNodes(List<Map<String, Object>> nodes) {
        int n = nodes.size();
        for (Map<String, Object> m : nodes) {
            @SuppressWarnings("unchecked")
            List<Map<String, Object>> children =
                    (List<Map<String, Object>>) m.get("children");
            if (children != null) {
                n += countNodes(children);
            }
        }
        return n;
    }

    /** 区划请求体。 */
    public record AreaRequest(Long id, String code, String name, Long parentId,
                              String areaType, String fullName, Integer areaSort,
                              String remark) {
    }

    /** POST /areas（area:manage）。 */
    @PostMapping("/areas")
    @RequirePerm("area:manage")
    public ResponseEntity<?> createArea(@RequestBody AreaRequest req) {
        if (req.name() == null || req.name().isBlank() || req.areaType() == null) {
            return badRequest("参数错误（name、area_type 必填）");
        }
        Area a = new Area();
        apply(a, req);
        areas.save(a);
        return ok(Map.of("id", a.id, "message", "区划已创建"));
    }

    /** PUT /areas/{id}（area:manage）。 */
    @PutMapping("/areas/{id}")
    @RequirePerm("area:manage")
    public ResponseEntity<?> updateArea(@PathVariable Long id, @RequestBody AreaRequest req) {
        var opt = areas.findById(id);
        if (opt.isEmpty()) {
            return notFound("区划不存在");
        }
        apply(opt.get(), req);
        areas.save(opt.get());
        return ok(Map.of("message", "区划已更新"));
    }

    private void apply(Area a, AreaRequest req) {
        a.code = nz(req.code());
        a.name = req.name();
        a.parentId = req.parentId();
        a.areaType = req.areaType();
        a.fullName = nz(req.fullName());
        a.areaSort = req.areaSort() == null ? 0 : req.areaSort();
        a.remark = nz(req.remark());
    }

    /** DELETE /areas/{id}（area:manage）：有子区划拒绝。 */
    @DeleteMapping("/areas/{id}")
    @RequirePerm("area:manage")
    public ResponseEntity<?> deleteArea(@PathVariable Long id) {
        var opt = areas.findById(id);
        if (opt.isEmpty()) {
            return notFound("区划不存在");
        }
        if (!areas.findByParentIdOrderByIdAsc(id).isEmpty()) {
            return badRequest("存在子级区划，无法删除");
        }
        areas.delete(opt.get());
        return ok(Map.of("message", "删除成功"));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
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
}

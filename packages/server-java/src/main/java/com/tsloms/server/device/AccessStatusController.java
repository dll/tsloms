// 路口聚合/接入状态/模块开关端点：补齐 Go 版对应路由。
package com.tsloms.server.device;

import com.tsloms.server.auth.ModuleService;
import com.tsloms.server.mqtt.DeviceAccessService;
import com.tsloms.server.mqtt.PahoMqttGateway;
import com.tsloms.server.model.Device;
import com.tsloms.server.model.FaultRecord;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FaultRecordRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.RequirePerm;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1")
public class AccessStatusController {

    private final DeviceRepository devices;
    private final FaultRecordRepository faults;
    private final PahoMqttGateway mqttGateway;
    private final ModuleService modules;

    public AccessStatusController(DeviceRepository devices, FaultRecordRepository faults,
                                  PahoMqttGateway mqttGateway, ModuleService modules) {
        this.devices = devices;
        this.faults = faults;
        this.mqttGateway = mqttGateway;
        this.modules = modules;
    }

    /** GET /access/status：检测器接入总览（对齐 Go 版 DetectorAccessStatus）。 */
    @GetMapping("/access/status")
    public ApiResponse<Map<String, Object>> accessStatus() {
        boolean connected = mqttGateway.isConnected();
        String topic = mqttGateway.subscribeTopic();

        long online = devices.countByOnlineStatus(true);
        Instant cut = Instant.now().minus(java.time.Duration.ofMinutes(30));
        long active = devices.findAll().stream()
                .filter(d -> d.lastCheckinAt != null && !d.lastCheckinAt.isBefore(cut))
                .count();

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("mqtt", Map.of(
                "connected", connected,
                "subscribe", topic,
                "topic_prefix", topic));
        data.put("real_hardware", Map.of(
                "mode", "real",
                "enabled", connected,
                "online_devices", online,
                "active_detectors", active));
        data.put("mock_enabled", true);
        data.put("csv_enabled", true);
        data.put("server_time", Instant.now().toString());
        return ApiResponse.ok(data);
    }

    /** GET /intersections：按路口名聚合设备（total/online/fault/坐标），对齐 Go 版。 */
    @GetMapping("/intersections")
    public ApiResponse<Map<String, Object>> intersections() {
        // 活跃故障设备集合
        var faultDevices = new java.util.HashSet<String>();
        faults.findAll().stream()
                .filter(f -> List.of("occurred", "confirmed", "dispatched").contains(f.status))
                .forEach(f -> faultDevices.add(f.deviceHwId));

        record Agg(int total, int online, int fault, Double lat, Double lng) {
        }
        Map<String, Agg> agg = new LinkedHashMap<>();
        for (Device d : devices.findAll()) {
            String name = d.intersection == null ? "" : d.intersection;
            if (name.isEmpty()) {
                continue;
            }
            Agg a = agg.get(name);
            int t = (a == null ? 0 : a.total()) + 1;
            int on = (a == null ? 0 : a.online()) + (d.onlineStatus ? 1 : 0);
            int fl = (a == null ? 0 : a.fault()) + (faultDevices.contains(d.hwId) ? 1 : 0);
            Double lat = a == null ? d.lat : a.lat();
            Double lng = a == null ? d.lng : a.lng();
            agg.put(name, new Agg(t, on, fl, lat, lng));
        }

        List<Map<String, Object>> list = new ArrayList<>();
        agg.forEach((name, a) -> {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("name", name);
            m.put("total", a.total());
            m.put("online", a.online());
            m.put("fault", a.fault());
            m.put("lat", a.lat());
            m.put("lng", a.lng());
            list.add(m);
        });
        return ApiResponse.ok(Map.of("list", list, "total", list.size()));
    }

    /** GET /modules：已启用模块（含元信息），对齐 Go 版 ListEnabledModules。 */
    @GetMapping("/modules")
    public ApiResponse<Map<String, Object>> enabledModules() {
        List<Map<String, Object>> infos = new ArrayList<>();
        for (String k : modules.enabledModuleList()) {
            boolean core = List.of("dashboard", "device", "intersection", "map", "feedback",
                    "fault", "workorder", "firmware", "log", "settings").contains(k);
            infos.add(Map.of("key", k, "name", moduleName(k), "core", core));
        }
        return ApiResponse.ok(Map.of("modules", infos));
    }

    private static final Map<String, String> MODULE_NAMES = Map.ofEntries(
            Map.entry("dashboard", "仪表盘"),
            Map.entry("device", "设备管理"),
            Map.entry("intersection", "路口管理"),
            Map.entry("map", "地图总览"),
            Map.entry("feedback", "问题反馈"),
            Map.entry("fault", "故障管理"),
            Map.entry("workorder", "工单管理"),
            Map.entry("firmware", "固件管理"),
            Map.entry("log", "系统日志"),
            Map.entry("settings", "系统设置"),
            Map.entry("video", "视频监控"),
            Map.entry("inventory", "物料库存"),
            Map.entry("purchase", "采购管理"),
            Map.entry("expense", "维修费用"),
            Map.entry("supplier", "供应商"),
            Map.entry("ai", "AI 分析"),
            Map.entry("dispatch", "调度看板"),
            Map.entry("notification", "站内通知"));

    private String moduleName(String key) {
        return MODULE_NAMES.getOrDefault(key, key);
    }
}

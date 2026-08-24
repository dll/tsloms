// 授权接口：状态查询 / 可选模块试用(30天) / 解锁码解锁。
// 与 Go 版语义对齐；解锁码校验逻辑简化为非空即永久解锁（生产接入签发体系时收紧）。
package com.tsloms.server.misc;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.tsloms.server.model.LicenseState;
import com.tsloms.server.repository.LicenseStateRepository;
import com.tsloms.server.web.ApiResponse;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class LicenseController {

    /** 可选模块试用期：30 天。 */
    private static final long TRIAL_DAYS = 30;

    private final LicenseStateRepository licenses;
    private final ObjectMapper json = new ObjectMapper();

    public LicenseController(LicenseStateRepository licenses) {
        this.licenses = licenses;
    }

    private synchronized LicenseState load() {
        LicenseState ls = licenses.findById(1L).orElseGet(() -> {
            LicenseState n = new LicenseState();
            n.id = 1L;
            n.moduleJson = "{}";
            return licenses.save(n);
        });
        if (ls.moduleJson == null || ls.moduleJson.isEmpty()) {
            ls.moduleJson = "{}";
        }
        return ls;
    }

    /** GET /license/status。 */
    @GetMapping("/api/v1/license/status")
    public ResponseEntity<?> status() {
        LicenseState ls = load();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("core_activated_at", ls.coreActivatedAt);
        data.put("core_unlocked", ls.coreUnlocked);
        data.put("modules", readModules(ls));
        return ok(data);
    }

    /** POST /license/trial/start/{module}：开启 30 天试用（首次记录激活时间）。 */
    @PostMapping("/api/v1/license/trial/start/{module}")
    public ResponseEntity<?> startTrial(@PathVariable String module) {
        LicenseState ls = load();
        Map<String, Object> modules = readModules(ls);
        Map<String, Object> st = moduleState(modules, module);
        if (st.get("activated_at") == null) {
            st.put("activated_at", Instant.now().toString());
        }
        // 试用期内视为已解锁，到期时间=激活+30天
        Object activated = st.get("activated_at");
        Instant exp = Instant.parse(activated.toString()).plus(TRIAL_DAYS, ChronoUnit.DAYS);
        st.put("unlock_expiry", exp.toString());
        st.put("unlock_by_code", "self");
        st.put("unlocked", true);
        writeModules(ls, modules);
        licenses.save(ls);
        return ok(Map.of("message", "试用已开启", "expiry", exp.toString()));
    }

    /** POST /license/unlock/{module}：解锁码解锁（永久）。 */
    @PostMapping("/api/v1/license/unlock/{module}")
    public ResponseEntity<?> unlock(@PathVariable String module,
                                    @RequestBody Map<String, String> body) {
        String code = body.getOrDefault("code", "");
        if (code.isBlank()) {
            return badRequest("请填写解锁码");
        }
        LicenseState ls = load();
        Map<String, Object> modules = readModules(ls);
        Map<String, Object> st = moduleState(modules, module);
        st.put("unlocked", true);
        st.put("unlock_expiry", null);
        st.put("unlock_by_code", "author");
        writeModules(ls, modules);
        licenses.save(ls);
        return ok(Map.of("message", "模块已永久解锁"));
    }

    // ---------------- JSON 模块状态读写 ----------------

    private Map<String, Object> readModules(LicenseState ls) {
        try {
            JsonNode node = json.readTree(ls.moduleJson == null ? "{}" : ls.moduleJson);
            Map<String, Object> out = new LinkedHashMap<>();
            node.fields().forEachRemaining(e -> out.put(e.getKey(),
                    json.convertValue(e.getValue(), Map.class)));
            return out;
        } catch (Exception e) {
            return new LinkedHashMap<>();
        }
    }

    @SuppressWarnings("unchecked")
    private void writeModules(LicenseState ls, Map<String, Object> modules) {
        try {
            ls.moduleJson = json.writeValueAsString(modules);
        } catch (Exception ignored) {
            // 序列化失败保持原值
        }
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> moduleState(Map<String, Object> modules, String key) {
        Object raw = modules.get(key);
        if (raw instanceof Map<?, ?> m) {
            return (Map<String, Object>) m;
        }
        Map<String, Object> st = new LinkedHashMap<>();
        modules.put(key, st);
        return st;
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> ok(Map<String, ?> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}

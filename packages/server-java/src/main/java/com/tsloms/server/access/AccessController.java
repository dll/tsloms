// 检测器接入接口：MQTT 凭据创建（硬件测试关键路径）。
// 对齐 Go 版 handler/access.go CreateMQTTDeviceCredential。
package com.tsloms.server.access;

import com.tsloms.server.config.EmqxProperties;
import com.tsloms.server.config.MqttProperties;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.RequirePerm;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.security.SecureRandom;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/access")
public class AccessController {

    private final EmqxProperties emqx;
    private final MqttProperties mqtt;
    private final HttpClient http = HttpClient.newBuilder()
            .version(HttpClient.Version.HTTP_1_1)
            .connectTimeout(Duration.ofSeconds(8)).build();

    public AccessController(EmqxProperties emqx, MqttProperties mqtt) {
        this.emqx = emqx;
        this.mqtt = mqtt;
    }

    /** POST /access/mqtt-credentials：生成检测器 MQTT 接入凭据（一次性显示）。 */
    @PostMapping("/mqtt-credentials")
    @RequirePerm("device:create")
    public ResponseEntity<?> mqttCredentials() {
        String user = "det" + random(5);
        String password = random(8);

        String token = emqx.token();
        if (token == null || token.isBlank()) {
            if (emqx.apiUsername() != null && !emqx.apiUsername().isBlank()) {
                token = emqxLogin(emqx.apiUrl(), emqx.apiUsername(), emqx.apiPassword());
            }
        }
        if (token == null || token.isBlank()) {
            return ResponseEntity.status(503).body(ApiResponse.fail(
                    "emqx_api_not_configured",
                    "检测器账号已生成但未配置 EMQX 管理 API，请联系系统管理员配置 EMQX_API_TOKEN"));
        }

        int code = emqxCreateUser(emqx.apiUrl(), token, user, password);
        if (code != 200 && code != 201) {
            return ResponseEntity.status(503).body(ApiResponse.fail(
                    "emqx_api_error", "EMQX 创建用户失败(HTTP " + code + ")"));
        }

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("username", user);
        data.put("password", password);
        data.put("broker", mqtt.broker() != null ? mqtt.broker() : "tcp://127.0.0.1:1883");
        data.put("message", "账号已创建，请立即保存（仅显示一次）");
        return ResponseEntity.ok(ApiResponse.ok(data));
    }

    // ---------------- EMQX Dashboard API ----------------

    private final org.slf4j.Logger log =
            org.slf4j.LoggerFactory.getLogger(AccessController.class);

    /** 登录 EMQX Dashboard 获取 Bearer token。 */
    private String emqxLogin(String base, String username, String password) {
        try {
            String body = "{\"username\":\"" + esc(username)
                    + "\",\"password\":\"" + esc(password) + "\"}";
            HttpRequest req = HttpRequest.newBuilder()
                    .uri(URI.create(base + "/api/v5/login"))
                    .header("Content-Type", "application/json")
                    .timeout(Duration.ofSeconds(8))
                    .POST(HttpRequest.BodyPublishers.ofString(body)).build();
            HttpResponse<byte[]> resp = http.send(req, HttpResponse.BodyHandlers.ofByteArray());
            log.info("[TSLOMS] EMQX login resp status={} body_len={}",
                    resp.statusCode(), resp.body().length);
            if (resp.statusCode() != 200) {
                log.warn("[TSLOMS] EMQX login 非200: {}",
                        new String(resp.body(), java.nio.charset.StandardCharsets.UTF_8));
                return null;
            }
            var node = new com.fasterxml.jackson.databind.ObjectMapper().readTree(resp.body());
            String token = node.path("token").asText(null);
            log.info("[TSLOMS] EMQX login token={}", token == null ? "null" : "OK(" + token.length() + "ch)");
            return token;
        } catch (Exception e) {
            log.error("[TSLOMS] EMQX login 异常: {}", e.getMessage(), e);
            return null;
        }
    }

    /** 在 EMQX built_in_database 认证源中创建 MQTT 用户。 */
    private int emqxCreateUser(String base, String token, String user, String password) {
        try {
            String body = "{\"user_id\":\"" + esc(user)
                    + "\",\"password\":\"" + esc(password) + "\"}";
            HttpRequest req = HttpRequest.newBuilder()
                    .uri(URI.create(base + "/api/v5/authentication/password_based:built_in_database/users"))
                    .header("Authorization", "Bearer " + token)
                    .header("Content-Type", "application/json")
                    .timeout(Duration.ofSeconds(8))
                    .POST(HttpRequest.BodyPublishers.ofString(body)).build();
            HttpResponse<byte[]> resp = http.send(req, HttpResponse.BodyHandlers.ofByteArray());
            log.info("[TSLOMS] EMQX create user resp status={}", resp.statusCode());
            return resp.statusCode();
        } catch (Exception e) {
            log.error("[TSLOMS] EMQX create user 异常: {}", e.getMessage(), e);
            return 500;
        }
    }

    private static String esc(String s) {
        return s.replace("\\", "\\\\").replace("\"", "\\\"");
    }

    private static String random(int len) {
        String chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789";
        SecureRandom r = new SecureRandom();
        StringBuilder sb = new StringBuilder(len);
        for (int i = 0; i < len; i++) {
            sb.append(chars.charAt(r.nextInt(chars.length())));
        }
        return sb.toString();
    }
}

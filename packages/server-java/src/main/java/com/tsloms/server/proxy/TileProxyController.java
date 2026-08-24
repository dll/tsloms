// 瓦片/POI 代理：对齐 Go 版 handler/proxy.go（gaode/baidu/amap place）。
// 增强：磁盘缓存（默认 24h）降低高德风控触发；浏览器 UA。
package com.tsloms.server.proxy;

import com.tsloms.server.web.ApiResponse;
import java.net.URI;
import java.util.ArrayList;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/proxy")
public class TileProxyController {

    /** 浏览器 UA：高德/百度对 bot UA 返回占位图/403。 */
    private static final String UA =
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                    + "(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

    private static final Path CACHE_DIR = Path.of("/var/cache/tsloms-tiles");
    private static final Duration CACHE_TTL = Duration.ofHours(24);

    private final HttpClient http = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(8)).build();
    private final String amapWebKey;

    public TileProxyController(
            @Value("${AMAP_WEB_KEY:}") String amapWebKey) {
        this.amapWebKey = amapWebKey == null ? "" : amapWebKey;
    }

    // ---------------- 高德瓦片 ----------------

    /** GET /proxy/gaode?x&y&z&style：style=8 路网(2D)/6 卫星。带磁盘缓存。 */
    @GetMapping("/gaode")
    public ResponseEntity<byte[]> gaode(@RequestParam String x, @RequestParam String y,
                                        @RequestParam String z,
                                        @RequestParam(defaultValue = "8") String style) {
        if (x.isEmpty() || y.isEmpty() || z.isEmpty()) {
            return ResponseEntity.badRequest().build();
        }
        int xi = parse(x);
        int sub = xi % 4 + 1;
        String url;
        if ("6".equals(style)) {
            // 卫星影像：webst 子域
            url = String.format(
                    "https://webst%02d.is.autonavi.com/appmaptile?style=6&x=%s&y=%s&z=%s",
                    sub, enc(x), enc(y), enc(z));
        } else {
            // 路网标注（2D）：webrd 子域需 lang/size/scale 参数
            url = String.format(
                    "https://webrd%02d.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=%s&x=%s&y=%s&z=%s",
                    sub, enc(style), enc(x), enc(y), enc(z));
        }
        byte[] body = fetchWithCache(url, "gaode/" + style + "/" + z + "/" + x + "_" + y,
                "https://www.amap.com/");
        if (body == null) {
            return ResponseEntity.status(502).build();
        }
        return ResponseEntity.ok()
                .contentType(MediaType.IMAGE_JPEG)
                .header("Cache-Control", "public, max-age=86400")
                .body(body);
    }

    /** GET /proxy/baidu?x&y&z：百度瓦片（带 Referer 限制绕行）。 */
    @GetMapping("/baidu")
    public ResponseEntity<byte[]> baidu(@RequestParam String x, @RequestParam String y,
                                        @RequestParam String z) {
        if (x.isEmpty() || y.isEmpty() || z.isEmpty()) {
            return ResponseEntity.badRequest().build();
        }
        int sub = (parse(x) + parse(y)) % 4;
        String url = String.format(
                "https://maponline%d.bdimg.com/tile/?qt=vtile&x=%s&y=%s&z=%s&styles=pl&scaler=1",
                sub, enc(x), enc(y), enc(z));
        byte[] body = fetchWithCache(url, "baidu/" + z + "/" + x + "_" + y, null);
        if (body == null) {
            return ResponseEntity.status(502).build();
        }
        return ResponseEntity.ok()
                .contentType(MediaType.IMAGE_PNG)
                .header("Cache-Control", "public, max-age=86400")
                .body(body);
    }

    /** GET /proxy/amap/place?kw=：高德 POI 搜索（需 AMAP_WEB_KEY）。 */
    @GetMapping("/amap/place")
    public ResponseEntity<?> amapPlace(@RequestParam String kw,
                                       @RequestParam(required = false) String city,
                                       @RequestParam(name = "loc", required = false) String loc,
                                       @RequestParam(name = "radius", required = false) String radius) {
        if (kw == null || kw.isBlank()) {
            return badRequest("缺少搜索关键字");
        }
        if (amapWebKey.isEmpty()) {
            return ResponseEntity.ok(ApiResponse.ok(Map.of(
                    "pois", List.of(), "fallback", true,
                    "message", "未配置 AMAP_WEB_KEY，使用本地位点检索")));
        }
        StringBuilder q = new StringBuilder("key=").append(enc(amapWebKey))
                .append("&keywords=").append(enc(kw)).append("&output=json");
        if (city != null && !city.isBlank()) {
            q.append("&city=").append(enc(city));
        }
        if (loc != null && !loc.isBlank()) {
            q.append("&location=").append(enc(loc)).append("&radius=")
                    .append(enc(radius == null ? "10000" : radius));
        }
        try {
            HttpRequest req = HttpRequest.newBuilder()
                    .uri(URI.create("https://restapi.amap.com/v3/place/text?" + q))
                    .header("User-Agent", UA)
                    .timeout(Duration.ofSeconds(8))
                    .GET().build();
            HttpResponse<byte[]> resp = http.send(req, HttpResponse.BodyHandlers.ofByteArray());
            var root = new com.fasterxml.jackson.databind.ObjectMapper()
                    .readTree(resp.body());
            var out = new ArrayList<Map<String, Object>>();
            if ("1".equals(root.path("status").asText())) {
                for (var p : root.path("pois")) {
                    String location = p.path("location").asText();
                    String[] parts = location.split(",");
                    if (parts.length != 2) {
                        continue;
                    }
                    out.add(Map.of(
                            "name", p.path("name").asText(),
                            "lng", Double.parseDouble(parts[0]),
                            "lat", Double.parseDouble(parts[1]),
                            "address", p.path("address").asText()));
                }
            }
            return ResponseEntity.ok(ApiResponse.ok(Map.of(
                    "list", out, "source", "amap")));
        } catch (Exception e) {
            return ResponseEntity.internalServerError()
                    .body(ApiResponse.fail("internal_error", "POI 检索失败"));
        }
    }

    // ---------------- 缓存与抓取 ----------------

    /** 抓取远端瓦片，命中磁盘缓存（24h）直接回源文件，降低高德风控触发频率。 */
    private byte[] fetchWithCache(String url, String cacheKey, String referer) {
        Path file = CACHE_DIR.resolve(cacheKey + ".bin");
        try {
            if (Files.exists(file)
                    && Duration.ofMillis(System.currentTimeMillis() - Files.getLastModifiedTime(file)
                            .toMillis()).compareTo(CACHE_TTL) < 0) {
                return Files.readAllBytes(file);
            }
        } catch (Exception ignored) {
            // 缓存读取失败走网络
        }
        try {
            HttpRequest.Builder rb = HttpRequest.newBuilder().uri(URI.create(url))
                    .header("User-Agent", UA)
                    .timeout(Duration.ofSeconds(8)).GET();
            if (referer != null) {
                rb.header("Referer", referer);
            }
            HttpResponse<byte[]> resp = http.send(rb.GET().build(),
                    HttpResponse.BodyHandlers.ofByteArray());
            if (resp.statusCode() != 200 || resp.body().length == 0) {
                return null;
            }
            try {
                Files.createDirectories(file.getParent());
                Files.write(file, resp.body());
            } catch (Exception ignored) {
                // 缓存写失败不影响响应
            }
            return resp.body();
        } catch (Exception e) {
            return null;
        }
    }

    private static int parse(String s) {
        int n = 0;
        for (char ch : s.toCharArray()) {
            if (ch < '0' || ch > '9') {
                break;
            }
            n = n * 10 + (ch - '0');
        }
        return n;
    }

    private static String enc(String s) {
        return URLEncoder.encode(s, StandardCharsets.UTF_8);
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest()
                .contentType(MediaType.APPLICATION_JSON)
                .body(ApiResponse.fail("bad_request", msg));
    }
}

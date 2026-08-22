// 健康检查接口：契约对齐 Go 版 internal/handler/response.go Health。
package com.tsloms.server.web;

import com.tsloms.server.config.AppProperties;
import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * GET /api/v1/health
 *
 * <p>响应 data：{"status":"ok","service":"tsloms-server","env":&lt;APP_ENV&gt;}
 * 供 nginx 探活、CD 部署后验证、CO 巡检使用。
 */
@RestController
@RequestMapping("/api/v1")
public class HealthController {

    private final AppProperties app;

    public HealthController(AppProperties app) {
        this.app = app;
    }

    @GetMapping("/health")
    public ApiResponse<Map<String, String>> health() {
        return ApiResponse.ok(Map.of(
                "status", "ok",
                "service", "tsloms-server",
                "env", app.env()));
    }
}

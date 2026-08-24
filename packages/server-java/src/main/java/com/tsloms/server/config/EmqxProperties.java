// EMQX 管理 API 配置（检测器账号创建用，对齐 Go 版 EMQX_API_*）。
package com.tsloms.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "emqx")
public record EmqxProperties(String apiUrl, String token, String apiUsername,
                             String apiPassword) {
}

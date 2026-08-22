// JWT 配置属性：JWT_SECRET 环境变量，开发默认与 Go 版一致（tsloms-secret-key）。
package com.tsloms.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "jwt")
public record JwtProperties(String secret) {

    public static final String DEFAULT_SECRET = "tsloms-secret-key";

    public JwtProperties {
        if (secret == null || secret.isBlank()) {
            secret = DEFAULT_SECRET;
        }
    }
}

// 应用配置属性：与 Go 版 internal/config 环境变量语义对齐。
package com.tsloms.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * 应用级配置。
 *
 * @param env 运行环境（APP_ENV，默认 development；Go 版同名变量）
 */
@ConfigurationProperties(prefix = "app")
public record AppProperties(String env) {

    /** APP_ENV 缺省值，与 Go 版 getEnv("APP_ENV", "development") 一致。 */
    public static final String DEFAULT_ENV = "development";

    public AppProperties {
        if (env == null || env.isBlank()) {
            env = DEFAULT_ENV;
        }
    }

    /** 是否生产环境（Go 版 config.IsProduction 同义）。 */
    public boolean isProduction() {
        return "production".equals(env);
    }
}

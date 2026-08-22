// 密码哈希工具：BCrypt，与 Go 版 model.HashPassword / bcrypt.CompareHashAndPassword 互通。
// 既有用户库中的 $2a$ 哈希可被 Spring Security BCrypt 直接校验，双跑期账号数据无需转换。
package com.tsloms.server.model;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Component;

@Component
public final class PasswordHasher {

    private final BCryptPasswordEncoder encoder;

    public PasswordHasher() {
        // strength=10 与 Go bcrypt.DefaultCost 一致
        this.encoder = new BCryptPasswordEncoder(10);
    }

    /** 哈希密码（对齐 Go 版 HashPassword）。 */
    public String hash(String rawPassword) {
        return encoder.encode(rawPassword);
    }

    /** 校验明文与哈希是否匹配（对齐 Go 版 CompareHashAndPassword 语义，兼容历史 $2a$/$2b$ 前缀）。 */
    public boolean matches(String rawPassword, String storedHash) {
        return encoder.matches(rawPassword, storedHash);
    }
}

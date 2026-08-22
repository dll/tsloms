// JWT 服务单元测试：签发/解析往返、篡改拒绝、短密钥快速失败。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.tsloms.server.auth.JwtService;
import com.tsloms.server.config.JwtProperties;
import io.jsonwebtoken.Claims;
import io.jsonwebtoken.JwtException;
import io.jsonwebtoken.security.WeakKeyException;
import org.junit.jupiter.api.Test;

class JwtServiceTest {

    private static final String SECRET32 = "unit-test-secret-key-0123456789abcdef";

    @Test
    void 签发解析往返_claims对齐Go版() {
        JwtService svc = new JwtService(new JwtProperties(SECRET32));
        String token = svc.issue(42L, "operator");
        Claims claims = svc.parse(token);
        // Go 版 user_id 为 JSON 数值，Java 端以 Number 读取
        assertThat(((Number) claims.get("user_id")).longValue()).isEqualTo(42L);
        assertThat(claims.get("role", String.class)).isEqualTo("operator");
        assertThat(claims.getExpiration()).isAfter(claims.getIssuedAt());
    }

    @Test
    void 篡改令牌_解析失败() {
        JwtService svc = new JwtService(new JwtProperties(SECRET32));
        String token = svc.issue(1L, "admin");
        String tampered = token.substring(0, token.length() - 3) + "abc";
        assertThatThrownBy(() -> svc.parse(tampered)).isInstanceOf(JwtException.class);
    }

    @Test
    void 错误密钥签发_校验失败() {
        String token = new JwtService(new JwtProperties(SECRET32)).issue(1L, "viewer");
        JwtService other = new JwtService(
                new JwtProperties("another-secret-key-0123456789abcdef0123456789"));
        assertThatThrownBy(() -> other.parse(token)).isInstanceOf(JwtException.class);
    }

    @Test
    void 短密钥_启动即快速失败() {
        assertThatThrownBy(() -> new JwtService(new JwtProperties("short-secret")))
                .isInstanceOf(WeakKeyException.class);
    }
}

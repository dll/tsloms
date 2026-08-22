// JWT 签发与校验：HS256、72 小时有效期，claims 对齐 Go 版 issueToken。
package com.tsloms.server.auth;

import com.tsloms.server.config.JwtProperties;
import io.jsonwebtoken.Claims;
import io.jsonwebtoken.JwtException;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.WeakKeyException;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.Date;
import javax.crypto.SecretKey;
import org.springframework.stereotype.Service;

/**
 * JWT 服务。
 *
 * <p>与 Go 版差异说明：golang-jwt 不校验密钥长度，jjwt 要求 HS256 密钥 ≥32 字节；
 * 生产密钥（32+ 位强随机）两者通用，开发默认短密钥在启动时快速失败并给出明确提示。
 */
@Service
public class JwtService {

    /** 令牌有效期：72 小时（对齐 Go 版 issueToken）。 */
    private static final long TTL_HOURS = 72;

    private final JwtProperties props;
    private SecretKey key;

    public JwtService(JwtProperties props) {
        this.props = props;
        byte[] bytes = props.secret().getBytes(StandardCharsets.UTF_8);
        if (bytes.length < 32) {
            throw new WeakKeyException(
                    "JWT_SECRET 至少需要 32 字节（当前 " + bytes.length + "）；生产请配置强随机密钥");
        }
        // 构造器内初始化（不依赖容器生命周期，纯单测同样可用）
        this.key = new javax.crypto.spec.SecretKeySpec(bytes, "HmacSHA256");
    }

    /**
     * 签发令牌。claims：user_id(long)、role(string)、exp(72h 后)，与 Go 版一致。
     */
    public String issue(Long userId, String role) {
        Instant now = Instant.now();
        return Jwts.builder()
                .claim("user_id", userId)
                .claim("role", role)
                .expiration(Date.from(now.plus(TTL_HOURS, ChronoUnit.HOURS)))
                .issuedAt(Date.from(now))
                .signWith(key, Jwts.SIG.HS256)
                .compact();
    }

    /**
     * 解析并验证令牌（签名 + 有效期）。
     *
     * @return claims；无效时抛出 {@link JwtException}
     */
    public Claims parse(String token) {
        return Jwts.parser().verifyWith(key).build()
                .parseSignedClaims(token).getPayload();
    }
}

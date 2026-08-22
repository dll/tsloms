// 认证拦截器：JWT 校验 + 用户实时校验 + 停用拦截，语义对齐 Go 版 middleware.Auth。
// 401 响应为裸 {"error": ...} 形状（非统一信封），保持与 Go 版逐字节一致。
package com.tsloms.server.web;

import com.tsloms.server.auth.JwtService;
import com.tsloms.server.model.UserStatuses;
import com.tsloms.server.repository.UserRepository;
import io.jsonwebtoken.Claims;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.nio.charset.StandardCharsets;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.servlet.HandlerInterceptor;

@Component
public class AuthInterceptor implements HandlerInterceptor {

    /** 请求属性键：当前用户 ID（对齐 Go 版 c.Set("user_id")）。 */
    public static final String ATTR_USER_ID = "userId";
    /** 请求属性键：当前用户角色。 */
    public static final String ATTR_USER_ROLE = "userRole";
    /** 请求属性键：当前用户名。 */
    public static final String ATTR_USERNAME = "username";

    private final JwtService jwt;
    private final UserRepository users;

    public AuthInterceptor(JwtService jwt, UserRepository users) {
        this.jwt = jwt;
        this.users = users;
    }

    @Override
    public boolean preHandle(HttpServletRequest request, HttpServletResponse response, Object handler)
            throws Exception {
        String header = request.getHeader("Authorization");
        if (header == null || header.isEmpty()) {
            return reject(response, HttpStatus.UNAUTHORIZED, "{\"error\":\"unauthorized\"}");
        }
        String tokenStr = header.startsWith("Bearer ") ? header.substring("Bearer ".length()) : header;

        Claims claims;
        try {
            claims = jwt.parse(tokenStr);
        } catch (Exception e) {
            return reject(response, HttpStatus.UNAUTHORIZED, "{\"error\":\"invalid token\"}");
        }

        Object rawUserId = claims.get("user_id");
        if (!(rawUserId instanceof Number num)) {
            return reject(response, HttpStatus.UNAUTHORIZED, "{\"error\":\"invalid claims\"}");
        }
        Long userId = num.longValue();

        // 实时校验用户存在
        var user = users.findById(userId);
        if (user.isEmpty()) {
            return reject(response, HttpStatus.UNAUTHORIZED, "{\"error\":\"user not found\"}");
        }

        // 停用用户拒绝既有令牌
        if (UserStatuses.DISABLED.equals(user.get().status)) {
            return reject(response, HttpStatus.UNAUTHORIZED,
                    "{\"error\":\"user disabled\",\"message\":\"账号已停用，请联系管理员\"}");
        }

        request.setAttribute(ATTR_USER_ID, user.get().id);
        request.setAttribute(ATTR_USER_ROLE, user.get().role);
        request.setAttribute(ATTR_USERNAME, user.get().username);
        return true;
    }

    private boolean reject(HttpServletResponse response, HttpStatus status, String json) throws Exception {
        response.setStatus(status.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        response.setCharacterEncoding(StandardCharsets.UTF_8.name());
        response.getWriter().write(json);
        return false;
    }

    /** 从请求中取当前用户 ID（受保护路由内调用）。 */
    public static Long userId(HttpServletRequest request) {
        return (Long) request.getAttribute(ATTR_USER_ID);
    }
}

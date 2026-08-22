// 权限拦截器：处理 @RequirePerm 注解，语义对齐 Go 版 middleware.RequirePerm。
// 403 响应为裸 {"error":"forbidden","message":...} 形状（与 Go 版一致）。
package com.tsloms.server.web;

import com.tsloms.server.rbac.RbacService;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.nio.charset.StandardCharsets;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.method.HandlerMethod;
import org.springframework.web.servlet.HandlerInterceptor;

@Component
public class RbacInterceptor implements HandlerInterceptor {

    private final RbacService rbac;

    public RbacInterceptor(RbacService rbac) {
        this.rbac = rbac;
    }

    @Override
    public boolean preHandle(HttpServletRequest request, HttpServletResponse response, Object handler)
            throws Exception {
        if (!(handler instanceof HandlerMethod hm)) {
            return true;
        }
        RequirePerm ann = hm.getMethodAnnotation(RequirePerm.class);
        if (ann == null) {
            ann = hm.getBeanType().getAnnotation(RequirePerm.class);
        }
        if (ann == null) {
            return true;
        }

        Long userId = AuthInterceptor.userId(request);
        if (userId == null) {
            return reject(response, HttpStatus.UNAUTHORIZED, "{\"error\":\"unauthorized\"}");
        }
        try {
            if (!rbac.effectivePermissions(userId).contains(ann.value())) {
                return reject(response, HttpStatus.FORBIDDEN,
                        "{\"error\":\"forbidden\",\"message\":\"无此功能权限: " + ann.value() + "\"}");
            }
        } catch (Exception e) {
            return reject(response, HttpStatus.INTERNAL_SERVER_ERROR,
                    "{\"error\":\"permission query failed\",\"message\":\"权限查询失败\"}");
        }
        return true;
    }

    private boolean reject(HttpServletResponse response, HttpStatus status, String json) throws Exception {
        response.setStatus(status.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        response.setCharacterEncoding(StandardCharsets.UTF_8.name());
        response.getWriter().write(json);
        return false;
    }
}

// Web MVC 配置：注册认证与权限拦截器，公开路径对齐 Go 版路由分组。
package com.tsloms.server.web;

import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.InterceptorRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@Configuration
public class WebConfig implements WebMvcConfigurer {

    private final AuthInterceptor authInterceptor;
    private final RbacInterceptor rbacInterceptor;

    public WebConfig(AuthInterceptor authInterceptor, RbacInterceptor rbacInterceptor) {
        this.authInterceptor = authInterceptor;
        this.rbacInterceptor = rbacInterceptor;
    }

    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        // 公开路径（Go 版 api 组内未套 auth 中间件的路由）
        // _test 前缀为测试类路径专用探针，生产无对应实现；_perm 探针需认证后判权，不排除
        registry.addInterceptor(authInterceptor)
                .addPathPatterns("/api/v1/**")
                .excludePathPatterns(
                        "/api/v1/health",
                        "/api/v1/auth/login",
                        "/api/v1/auth/register",
                        "/api/v1/auth/captcha",
                        "/api/v1/_test/**");
        // 权限拦截器在认证之后执行
        registry.addInterceptor(rbacInterceptor)
                .addPathPatterns("/api/v1/**")
                .excludePathPatterns(
                        "/api/v1/health",
                        "/api/v1/auth/login",
                        "/api/v1/auth/register",
                        "/api/v1/auth/captcha",
                        "/api/v1/_test/**")
                .order(1);
    }
}

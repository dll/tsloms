// 功能权限注解：标注在 Controller 方法/类上，由 RbacInterceptor 校验。
// 对齐 Go 版 middleware.RequirePerm(perm)。
package com.tsloms.server.web;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

@Target({ElementType.METHOD, ElementType.TYPE})
@Retention(RetentionPolicy.RUNTIME)
public @interface RequirePerm {

    /** 所需权限码（如 device:create）。 */
    String value();
}

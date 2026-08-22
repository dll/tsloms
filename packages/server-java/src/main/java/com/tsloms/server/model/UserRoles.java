// 用户角色常量（对齐 Go 版 model/user.go RoleSuperAdmin 等常量）。
package com.tsloms.server.model;

public final class UserRoles {
    public static final String SUPER_ADMIN = "super_admin";
    public static final String ADMIN = "admin";
    public static final String OPERATOR = "operator";
    public static final String VIEWER = "viewer";

    private UserRoles() {
    }
}

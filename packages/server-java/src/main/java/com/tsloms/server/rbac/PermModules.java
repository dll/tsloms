// 权限模块常量：对齐 Go 版 rbac.go PermModule* 常量（只增不删既有权限码）。
package com.tsloms.server.rbac;

public final class PermModules {
    public static final String DEVICE = "device";
    public static final String INTERSECTION = "intersection";
    public static final String FAULT = "fault";
    public static final String WORKORDER = "workorder";
    public static final String MEDIA = "media";
    public static final String FIRMWARE = "firmware";
    public static final String INVENTORY = "inventory";
    public static final String SUPPLIER = "supplier";
    public static final String PURCHASE = "purchase";
    public static final String EXPENSE = "expense";
    public static final String USER = "user";
    public static final String DEPT = "dept";
    public static final String ROLE = "role";
    public static final String AI = "ai";
    public static final String WARNING = "warning";
    public static final String CROSSING = "crossing";
    public static final String AREA = "area";
    public static final String PATROL = "patrol";
    public static final String SETTINGS = "settings";
    public static final String DEMO = "demo";

    private PermModules() {
    }
}

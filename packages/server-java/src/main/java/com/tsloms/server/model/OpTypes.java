// 操作类型常量：对齐 Go 版 model.Op* 常量。
package com.tsloms.server.model;

public final class OpTypes {
    public static final String LOGIN = "login";
    public static final String CREATE = "create";
    public static final String UPDATE = "update";
    public static final String DELETE = "delete";
    public static final String DISPATCH = "dispatch";

    private OpTypes() {
    }
}

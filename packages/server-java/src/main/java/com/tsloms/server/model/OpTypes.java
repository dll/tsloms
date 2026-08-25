// 操作日志实体：列名对齐 Go 版 operation_logs MySQL 表。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import java.time.Instant;

/** 操作类型常量。 */
public final class OpTypes {
    public static final String LOGIN = "login";
    public static final String CREATE = "create";
    public static final String UPDATE = "update";
    public static final String DELETE = "delete";
    public static final String DISPATCH = "dispatch";

    private OpTypes() {
    }
}

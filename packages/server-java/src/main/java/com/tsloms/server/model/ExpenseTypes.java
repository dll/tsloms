// 维修费用类型常量与实体（对齐 Go 版 RepairExpense，表 repair_expenses）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

/** 费用类型常量。 */
public final class ExpenseTypes {
    public static final String MATERIAL = "material"; // 物料
    public static final String LABOR = "labor";       // 人工
    public static final String TRAFFIC = "traffic";   // 交通
    public static final String OTHER = "other";       // 其他

    private ExpenseTypes() {
    }
}

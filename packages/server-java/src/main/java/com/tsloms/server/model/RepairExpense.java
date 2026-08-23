// 维修费用单实体：对齐 Go 版 RepairExpense（表 repair_expenses）。
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
import java.time.LocalDate;

@Entity
@Table(name = "repair_expenses",
        uniqueConstraints = @UniqueConstraint(name = "uk_rexpense_no", columnNames = "expense_no"),
        indexes = {
                @Index(name = "idx_rexpense_work_order", columnList = "work_order_id"),
                @Index(name = "idx_rexpense_device_hw", columnList = "device_hw_id"),
                @Index(name = "idx_rexpense_type", columnList = "type"),
        })
public class RepairExpense {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 费用单号 FE{yyyyMMdd}{seq}。 */
    @Column(name = "expense_no", nullable = false, length = 32)
    public String expenseNo;

    @Column(name = "work_order_id")
    public Long workOrderId;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    /** material/labor/traffic/other。 */
    @Column(name = "type", nullable = false, length = 16)
    public String type;

    @Column(name = "amount", nullable = false)
    public Double amount = 0.0;

    @Column(name = "description", length = 255)
    public String description;

    /** 维修日期。 */
    @Column(name = "work_date")
    public LocalDate workDate;

    @Column(name = "operator", length = 64)
    public String operator;

    @Column(name = "confirmed", nullable = false)
    public boolean confirmed;

    @Column(name = "note", columnDefinition = "text")
    public String note;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (updatedAt == null) {
            updatedAt = now;
        }
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}

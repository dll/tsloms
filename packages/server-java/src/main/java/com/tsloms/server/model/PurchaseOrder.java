// 采购单实体：逐列对齐 Go 版 PurchaseOrder（表 purchase_orders）。
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

/** 状态流转：draft → partial → completed；可 cancelled。 */
@Entity
@Table(name = "purchase_orders",
        uniqueConstraints = @UniqueConstraint(name = "uk_po_order_no", columnNames = "order_no"),
        indexes = @Index(name = "idx_po_supplier", columnList = "supplier_id"))
public class PurchaseOrder {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 采购单号 PO{yyyyMMdd}{seq}。 */
    @Column(name = "order_no", nullable = false, length = 32)
    public String orderNo;

    @Column(name = "supplier_id", nullable = false)
    public Long supplierId;

    /** draft/partial/completed/cancelled。 */
    @Column(name = "status", nullable = false, length = 16)
    public String status = "draft";

    @Column(name = "total_amount", nullable = false)
    public Double totalAmount = 0.0;

    @Column(name = "received_at")
    public Instant receivedAt;

    @Column(name = "operator", length = 64)
    public String operator;

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

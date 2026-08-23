// 物料档案实体：逐列对齐 Go 版 Material（表 materials）。
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

/** 物料台账：同一物料可多处使用。 */
@Entity
@Table(name = "materials",
        uniqueConstraints = @UniqueConstraint(name = "uk_materials_code", columnNames = "code"),
        indexes = {
                @Index(name = "idx_materials_name", columnList = "name"),
                @Index(name = "idx_materials_category", columnList = "category"),
                @Index(name = "idx_materials_device_hw", columnList = "device_hw_id"),
                @Index(name = "idx_materials_supplier", columnList = "supplier_id"),
        })
public class Material {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "code", nullable = false, length = 32)
    public String code;

    @Column(name = "name", nullable = false, length = 64)
    public String name;

    /** 分类：灯组/电源/线材等。 */
    @Column(name = "category", length = 32)
    public String category;

    @Column(name = "spec", length = 128)
    public String spec;

    @Column(name = "unit", length = 16)
    public String unit;

    @Column(name = "unit_price", nullable = false)
    public Double unitPrice = 0.0;

    @Column(name = "stock", nullable = false)
    public Integer stock = 0;

    /** 低库存预警阈值。 */
    @Column(name = "threshold", nullable = false)
    public Integer threshold = 0;

    /** 关联设备 ID（可空）。 */
    @Column(name = "device_hw_id", length = 64)
    public String deviceHwId;

    @Column(name = "supplier_id")
    public Long supplierId;

    @Column(name = "note", columnDefinition = "text")
    public String note;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "active";

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

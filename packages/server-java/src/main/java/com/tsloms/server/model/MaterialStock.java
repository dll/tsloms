// 出入库流水实体：对齐 Go 版 MaterialStock（表 material_stocks）。
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

@Entity
@Table(name = "material_stocks", indexes = {
        @Index(name = "idx_ms_material_id", columnList = "material_id"),
        @Index(name = "idx_ms_type", columnList = "type"),
        @Index(name = "idx_ms_ref_id", columnList = "ref_id"),
        @Index(name = "idx_ms_work_order_id", columnList = "work_order_id"),
})
public class MaterialStock {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "material_id", nullable = false)
    public Long materialId;

    /** 物料名称（冗余）。 */
    @Column(name = "material_name", length = 64)
    public String materialName;

    /** in/use/return/gain/loss/adjust。 */
    @Column(name = "type", nullable = false, length = 16)
    public String type;

    /** 变动数量（正入负出）。 */
    @Column(name = "quantity", nullable = false)
    public Integer quantity;

    @Column(name = "price", nullable = false)
    public Double price = 0.0;

    @Column(name = "amount", nullable = false)
    public Double amount = 0.0;

    /** 关联类型（purchase/repair/adjust）。 */
    @Column(name = "ref_type", length = 24)
    public String refType;

    @Column(name = "ref_id", nullable = false)
    public Long refId;

    @Column(name = "work_order_id")
    public Long workOrderId;

    @Column(name = "operator", length = 64)
    public String operator;

    @Column(name = "note", length = 255)
    public String note;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}

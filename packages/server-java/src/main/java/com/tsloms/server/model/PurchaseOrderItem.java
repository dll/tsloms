// 采购明细实体：对齐 Go 版 PurchaseOrderItem（表 purchase_order_items）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.Table;

@Entity
@Table(name = "purchase_order_items", indexes = {
        @Index(name = "idx_poi_order_id", columnList = "order_id"),
        @Index(name = "idx_poi_material_id", columnList = "material_id"),
})
public class PurchaseOrderItem {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "order_id", nullable = false)
    public Long orderId;

    @Column(name = "material_id", nullable = false)
    public Long materialId;

    /** 物料名称（冗余）。 */
    @Column(name = "material_name", length = 64)
    public String materialName;

    @Column(name = "quantity", nullable = false)
    public Integer quantity;

    @Column(name = "price", nullable = false)
    public Double price = 0.0;

    @Column(name = "amount", nullable = false)
    public Double amount = 0.0;

    /** 已入库数量。 */
    @Column(name = "received_qty", nullable = false)
    public Integer receivedQty = 0;
}

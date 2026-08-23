// PurchaseOrderItemRepository ：PurchaseOrderItem 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.PurchaseOrderItem;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface PurchaseOrderItemRepository extends JpaRepository<PurchaseOrderItem, Long> {
    List<PurchaseOrderItem> findByOrderIdOrderByIdAsc(Long orderId);

    void deleteByOrderId(Long orderId);
}

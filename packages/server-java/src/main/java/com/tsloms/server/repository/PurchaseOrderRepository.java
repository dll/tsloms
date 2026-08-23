// PurchaseOrderRepository ：PurchaseOrder 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.PurchaseOrder;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface PurchaseOrderRepository extends JpaRepository<PurchaseOrder, Long>, JpaSpecificationExecutor<PurchaseOrder> {
    long countByOrderNoStartingWith(String prefix); Optional<PurchaseOrder> findByOrderNo(String orderNo);
}

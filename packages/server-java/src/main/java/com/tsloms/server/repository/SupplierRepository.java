// SupplierRepository ：Supplier 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Supplier;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface SupplierRepository extends JpaRepository<Supplier, Long>, JpaSpecificationExecutor<Supplier> {
    Optional<Supplier> findByName(String name);

    boolean existsByName(String name);
}

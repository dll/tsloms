// MaterialRepository ：Material 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Material;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface MaterialRepository extends JpaRepository<Material, Long>, JpaSpecificationExecutor<Material> {
    boolean existsByCode(String code);
}

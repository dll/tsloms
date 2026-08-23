// MaterialStockRepository ：MaterialStock 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.MaterialStock;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface MaterialStockRepository extends JpaRepository<MaterialStock, Long>, JpaSpecificationExecutor<MaterialStock> {
}

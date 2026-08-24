// FaultCaseRepository ：FaultCase 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.FaultCase;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FaultCaseRepository extends JpaRepository<FaultCase, Long> {
}

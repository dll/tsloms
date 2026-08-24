// WarningRepository ：Warning 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Warning;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface WarningRepository extends JpaRepository<Warning, Long>, JpaSpecificationExecutor<Warning> {
}

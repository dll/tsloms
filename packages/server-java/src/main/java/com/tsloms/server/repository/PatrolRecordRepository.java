// PatrolRecordRepository ：PatrolRecord 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.PatrolRecord;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface PatrolRecordRepository extends JpaRepository<PatrolRecord, Long>, JpaSpecificationExecutor<PatrolRecord> {
}

// 操作日志仓库与记录服务。
package com.tsloms.server.repository;

import com.tsloms.server.model.OperationLog;
import java.util.List;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;

public interface OperationLogRepository extends JpaRepository<OperationLog, Long> {

    List<OperationLog> findByTargetOrderByCreatedAtDesc(String target, Pageable pageable);
}

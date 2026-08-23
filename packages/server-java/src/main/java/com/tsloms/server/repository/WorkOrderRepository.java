// 工单仓库：状态统计与 SLA 超时查询。
package com.tsloms.server.repository;

import com.tsloms.server.model.WorkOrder;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface WorkOrderRepository extends JpaRepository<WorkOrder, Long> {

    long countByStatus(String status);

    Optional<WorkOrder> findFirstByFaultIdAndFaultActiveScope(Long faultId, Long faultActiveScope);

    /** SLA 超时工单数：pending 超 24h 或 processing 超 48h（对齐 Go 版口径）。 */
    @Query("SELECT COUNT(w) FROM WorkOrder w WHERE "
            + "(w.status = 'pending' AND w.createdAt < :pendingOverdue) OR "
            + "(w.status = 'processing' AND w.createdAt < :procOverdue)")
    long countOverdue(@Param("pendingOverdue") Instant pendingOverdue,
                      @Param("procOverdue") Instant procOverdue);

    /** 已完成且有闭环时间的工单（平均闭环时长统计用）。 */
    List<WorkOrder> findByStatusAndClosedAtNotNullAndCreatedAtGreaterThanEqual(
            String status, Instant since);
}

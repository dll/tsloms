// 故障记录仓库：统计聚合查询（仪表盘用）。
package com.tsloms.server.repository;

import com.tsloms.server.model.FaultRecord;
import java.time.Instant;
import java.util.List;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface FaultRecordRepository extends JpaRepository<FaultRecord, Long> {

    long countByStatusIn(List<String> statuses);

    long countByStatus(String status);

    long countByFirstSeenGreaterThanEqual(Instant since);

    /** 故障类型占比（饼图）：按类型分组计数，数量降序。 */
    @Query("SELECT f.faultType AS k, COUNT(f) AS cnt FROM FaultRecord f "
            + "WHERE f.firstSeen >= :since GROUP BY f.faultType "
            + "ORDER BY COUNT(f) DESC")
    List<NameCount> countByFaultTypeSince(@Param("since") Instant since);

    /** 设备故障排行 Top N（柱状图）。 */
    @Query("SELECT f.deviceHwId AS k, COUNT(f) AS cnt FROM FaultRecord f "
            + "WHERE f.firstSeen >= :since GROUP BY f.deviceHwId "
            + "ORDER BY COUNT(f) DESC")
    List<NameCount> countByDeviceSince(@Param("since") Instant since, Pageable topN);

    List<FaultRecord> findByFirstSeenGreaterThanEqual(Instant since);

    /** 通用名称-计数投影。 */
    interface NameCount {
        String getK();

        long getCnt();
    }
}

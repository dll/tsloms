// AIUsageRepository ：AIUsage 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.AIUsage;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AIUsageRepository extends JpaRepository<AIUsage, Long> {
}

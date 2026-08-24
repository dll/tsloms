// 预警规则仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.WarningRule;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface WarningRuleRepository extends JpaRepository<WarningRule, Long> {

    List<WarningRule> findByEnabledTrue();
}

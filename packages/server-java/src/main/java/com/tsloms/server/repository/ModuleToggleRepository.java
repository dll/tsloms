// 模块开关仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.ModuleToggle;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface ModuleToggleRepository extends JpaRepository<ModuleToggle, Long> {
    Optional<ModuleToggle> findByModuleKey(String moduleKey);

    List<ModuleToggle> findAllByEnabledTrue();
}

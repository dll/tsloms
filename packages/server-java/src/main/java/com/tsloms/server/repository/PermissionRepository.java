// RBAC 仓库层：权限/角色/关联查询。
package com.tsloms.server.repository;

import com.tsloms.server.model.Permission;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface PermissionRepository extends JpaRepository<Permission, Long> {
    Optional<Permission> findByCode(String code);

    boolean existsByCode(String code);
}

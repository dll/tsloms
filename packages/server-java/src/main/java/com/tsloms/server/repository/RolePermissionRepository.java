// 角色-权限关联仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.RolePermission;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface RolePermissionRepository extends JpaRepository<RolePermission, Long> {
    List<RolePermission> findByRoleId(Long roleId);
}

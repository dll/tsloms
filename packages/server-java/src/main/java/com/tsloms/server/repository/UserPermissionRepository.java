// 用户级权限覆写仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.UserPermission;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface UserPermissionRepository extends JpaRepository<UserPermission, Long> {
    List<UserPermission> findByUserId(Long userId);
}

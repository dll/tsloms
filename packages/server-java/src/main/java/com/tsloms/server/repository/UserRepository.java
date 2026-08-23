// 用户仓库：常用查询对齐 handler 层既有用法。
package com.tsloms.server.repository;

import com.tsloms.server.model.User;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;
import org.springframework.data.jpa.repository.Query;

public interface UserRepository
        extends JpaRepository<User, Long>, JpaSpecificationExecutor<User> {

    Optional<User> findByUsername(String username);

    Optional<User> findByPhoneLogin(String phoneLogin);

    boolean existsByUsername(String username);

    long countByDepartmentId(Long departmentId);

    /** 部门成员计数分组查询（对齐 Go 版 ListDepartments 的 GROUP BY 统计）。 */
    @Query("SELECT u.departmentId AS departmentId, COUNT(u) AS cnt FROM User u "
            + "WHERE u.departmentId IS NOT NULL GROUP BY u.departmentId")
    List<DeptCountProjection> countByDepartmentIdGrouped();

    /** 部门成员数投影。 */
    interface DeptCountProjection {
        Long getDepartmentId();

        long getCnt();
    }

    /** 按角色编码统计用户数（删除角色前检查）。 */
    long countByRole(String role);
}

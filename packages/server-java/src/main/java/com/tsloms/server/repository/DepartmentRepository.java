// 部门仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.Department;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DepartmentRepository extends JpaRepository<Department, Long> {

    boolean existsByName(String name);
}

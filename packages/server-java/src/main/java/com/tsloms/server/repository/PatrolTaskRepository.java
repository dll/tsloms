// PatrolTaskRepository ：PatrolTask 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.PatrolTask;
import org.springframework.data.jpa.repository.JpaRepository;

public interface PatrolTaskRepository extends JpaRepository<PatrolTask, Long> {
}

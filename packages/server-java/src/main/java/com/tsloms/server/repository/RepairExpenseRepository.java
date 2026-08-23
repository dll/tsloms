// RepairExpenseRepository ：RepairExpense 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.RepairExpense;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface RepairExpenseRepository extends JpaRepository<RepairExpense, Long>, JpaSpecificationExecutor<RepairExpense> {
    long countByExpenseNoStartingWith(String prefix);
}

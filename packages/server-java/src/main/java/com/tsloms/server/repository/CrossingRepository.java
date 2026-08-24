// CrossingRepository ：Crossing 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Crossing;
import org.springframework.data.jpa.repository.JpaRepository;

public interface CrossingRepository extends JpaRepository<Crossing, Long> {
    boolean existsByName(String name);
}

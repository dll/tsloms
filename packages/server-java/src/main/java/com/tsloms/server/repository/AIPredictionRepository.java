// AIPredictionRepository ：AIPrediction 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.AIPrediction;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AIPredictionRepository extends JpaRepository<AIPrediction, Long> {
}

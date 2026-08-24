// FaultEvidenceRepository ：FaultEvidence 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.FaultEvidence;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FaultEvidenceRepository extends JpaRepository<FaultEvidence, Long> {
    List<FaultEvidence> findByFaultIdOrderByIdAsc(Long faultId); List<FaultEvidence> findByEvaluationIdOrderByIdAsc(String evaluationId);
}

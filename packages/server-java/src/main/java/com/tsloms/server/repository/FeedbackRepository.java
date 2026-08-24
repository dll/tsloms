// FeedbackRepository ：Feedback 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Feedback;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface FeedbackRepository extends JpaRepository<Feedback, Long>, JpaSpecificationExecutor<Feedback> {
}
